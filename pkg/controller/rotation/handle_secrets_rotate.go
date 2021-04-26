// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rotation

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"time"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/hashicorp/go-multierror"
	"go.opencensus.io/stats"
)

// HandleRotateSecrets handles secrets rotation.
func (c *Controller) HandleRotateSecrets() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		logger := logging.FromContext(ctx).Named("rotation.HandleRotateSecrets")
		logger.Debugw("starting")
		defer logger.Debugw("finishing")

		ok, err := c.db.TryLock(ctx, secretsRotationLock, c.config.MinTTL)
		if err != nil {
			logger.Errorw("failed to acquire lock", "error", err)
			c.h.RenderJSON(w, http.StatusInternalServerError, err)
			return
		}
		if !ok {
			logger.Debugw("skipping (too early)")
			c.h.RenderJSON(w, http.StatusOK, fmt.Errorf("too early"))
			return
		}

		// If there are any errors, return them
		if err := c.RotateSecrets(ctx); err != nil {
			logger.Errorw("failed to rotate secrets", "error", err)
			c.h.RenderJSON(w, http.StatusInternalServerError, err)
			return
		}

		stats.Record(ctx, mSecretsSuccess.M(1))
		c.h.RenderJSON(w, http.StatusOK, nil)
	})
}

// RotateSecrets triggers a secret rotation. It does not take out a lock nor
// does it return an HTTP response. This is primarily used so other functions
// can perform initials ecrets bootstrapping.
func (c *Controller) RotateSecrets(ctx context.Context) error {
	logger := logging.FromContext(ctx).Named("rotation.RotateSecrets")

	var merr *multierror.Error

	// API key database HMAC keys
	func() {
		logger.Debugw("rotating API key database HMAC keys")
		defer logger.Debugw("finished rotating API key database HMAC keys")

		typ := database.SecretTypeAPIKeyDatabaseHMAC
		parent := "db-apikey-db-hmac"
		minTTL := c.config.APIKeyDatabaseHMACKeyMinAge
		maxTTL := time.Duration(0)
		if err := c.rotateSecret(ctx, typ, parent, 128, minTTL, maxTTL); err != nil {
			merr = multierror.Append(merr, fmt.Errorf("failed to rotate api key database hmac key: %w", err))
			return
		}
	}()

	// API key signature HMAC keys
	func() {
		logger.Debugw("rotating API key signature HMAC keys")
		defer logger.Debugw("finished rotating API key signature HMAC keys")

		typ := database.SecretTypeAPIKeySignatureHMAC
		parent := "db-apikey-sig-hmac"
		minTTL := c.config.APIKeySignatureHMACKeyMinAge
		maxTTL := time.Duration(0)
		if err := c.rotateSecret(ctx, typ, parent, 128, minTTL, maxTTL); err != nil {
			merr = multierror.Append(merr, fmt.Errorf("failed to rotate api key signature hmac key: %w", err))
			return
		}
	}()

	// Cookie key rotation
	func() {
		logger.Debugw("rotating cookie keys")
		defer logger.Debugw("finished rotating cookie keys")

		typ := database.SecretTypeCookieKeys
		parent := "cookie-keys"
		minTTL := c.config.CookieKeyMinAge
		maxTTL := c.config.CookieKeyMaxAge
		if err := c.rotateSecret(ctx, typ, parent, 32+64, minTTL, maxTTL); err != nil {
			merr = multierror.Append(merr, fmt.Errorf("failed to rotate cookie key: %w", err))
			return
		}
	}()

	// Phone number database HMAC
	func() {
		logger.Debugw("rotating phone number database HMAC keys")
		defer logger.Debugw("finished rotating phone number database HMAC keys")

		typ := database.SecretTypePhoneNumberDatabaseHMAC
		parent := "db-phone-number-hmac"
		minTTL := c.config.PhoneNumberDatabaseHMACKeyMinAge
		maxTTL := c.config.PhoneNumberDatabaseHMACKeyMaxAge
		if err := c.rotateSecret(ctx, typ, parent, 128, minTTL, maxTTL); err != nil {
			merr = multierror.Append(merr, fmt.Errorf("failed to rotate phone number database hmac key: %w", err))
			return
		}
	}()

	// Verification code database HMAC
	func() {
		logger.Debugw("rotating verification code database HMAC keys")
		defer logger.Debugw("finished rotating verification code database HMAC keys")

		typ := database.SecretTypeVerificationCodeDatabaseHMAC
		parent := "db-verification-code-hmac"
		minTTL := c.config.VerificationCodeDatabaseHMACKeyMinAge
		maxTTL := c.config.VerificationCodeDatabaseHMACKeyMaxAge
		if err := c.rotateSecret(ctx, typ, parent, 128, minTTL, maxTTL); err != nil {
			merr = multierror.Append(merr, fmt.Errorf("failed to rotate verification code database hmac key: %w", err))
			return
		}
	}()

	return merr.ErrorOrNil()
}

// rotateAPIKeyHMAC rotates the key used to HMAC API keys stored in the
// database.
func (c *Controller) rotateSecret(ctx context.Context, typ database.SecretType, parent string, numBytes int, minTTL, maxTTL time.Duration) error {
	now := time.Now().UTC()

	logger := logging.FromContext(ctx).Named("rotateSecret").
		With("type", typ).
		With("parent", parent).
		With("num_bytes", numBytes).
		With("min_ttl", minTTL).
		With("max_ttl", maxTTL)

	existing, err := c.db.ListSecretsForType(typ)
	if err != nil {
		return fmt.Errorf("failed to lookup existing secrets: %w", err)
	}

	// Find the "newest" secret that has been created. Since ListSecretsForType
	// sorts by usable, it's possible that a new secret exists but is not yet
	// active. That would be at the end of the list.
	latestSecretCreatedAt := time.Time{}
	for _, secret := range existing {
		if secret.CreatedAt.UTC().After(latestSecretCreatedAt) {
			latestSecretCreatedAt = secret.CreatedAt.UTC()
		}
	}
	logger.Debugw("calculated latest secret created time",
		"created_at", latestSecretCreatedAt,
		"ttl", now.Sub(latestSecretCreatedAt),
		"secrets", existing)

	// If there is no latest secret, or if the latest secret has existed for
	// longer than the minimum age, create a new one.
	if minTTL > 0 && now.Sub(latestSecretCreatedAt) > minTTL {
		logger.Infow("latest secret does not exist or is older than min_ttl, creating new secret",
			"secret", existing)

		ref, err := c.createUpstreamSecretVersion(ctx, parent, numBytes)
		if err != nil {
			return fmt.Errorf("failed to create new secret version: %w", err)
		}

		secret := &database.Secret{
			Type:      typ,
			Reference: ref,
			Active:    len(existing) == 0,
		}
		if err := c.db.SaveSecret(secret, RotationActor); err != nil {
			return fmt.Errorf("failed to save new secret %s: %w", ref, err)
		}
		existing = append(existing, secret)
	}

	logger.Debugw("activating existing secrets")
	for _, secret := range existing {
		modified := secret.CreatedAt.UTC() != secret.UpdatedAt.UTC()
		createdTTL := now.Sub(secret.CreatedAt.UTC())
		updatedTTL := now.Sub(secret.UpdatedAt.UTC())

		logger = logger.With(
			"secret", secret,
			"modified", modified,
			"created_ttl", createdTTL,
			"updated_ttl", updatedTTL)

		logger.Debugw("checking if secret is ready to be activated")

		// Activate any secrets that are ready to be activated. Please don't try to
		// optimize this. Yes, this could be reduced into a single SQL statement,
		// but we want to ensure the logging and AuditLog exist for debugging.
		if !secret.Active && !modified && createdTTL >= c.config.SecretActivationTTL {
			logger.Infow("activating secret", "secret", secret)

			secret.Active = true
			if err := c.db.SaveSecret(secret, RotationActor); err != nil {
				return fmt.Errorf("failed to activate secret %d: %w", secret.ID, err)
			}
		}

		// If a maximum TTL was given, check for expirations.
		if maxTTL > 0 {
			logger.Debugw("checking secret is expired")

			// If the secret has existed for longer than the maximum TTL, deactivate
			// it. This will move it to the end of the list. For HMAC-style secrets,
			// this means it will still be available to validate values as the cache
			// updates, but will not be used to HMAC new values.
			if secret.Active && createdTTL >= maxTTL {
				logger.Infow("deactivating expired secret", "secret", secret)

				secret.Active = false
				if err := c.db.SaveSecret(secret, RotationActor); err != nil {
					return fmt.Errorf("failed to deactivate secret %d: %w", secret.ID, err)
				}
			}

			// If the secret is inactive, experienced an update, and the time since
			// that update has exceeded the activation TTL, mark the secret for
			// deletion.
			//
			// The only mutable field is "Active", so if CreatedAt and UpdatedAt have
			// diverged, it means the secret was previously activated and is now
			// deactivated.
			//
			// Without this check, a secret with a low TTL could be marked for
			// deletion before activation.
			if !secret.Active && modified && updatedTTL >= c.config.SecretActivationTTL {
				logger.Infow("marking secret for deletion", "secret", secret)

				if err := c.db.DeleteSecret(secret, RotationActor); err != nil {
					return fmt.Errorf("failed to mark secret %d for deletion: %w", secret.ID, err)
				}
			}
		}
	}

	logger.Debugw("checking for secrets to purge")

	// Hard delete any secrets that are marked for deletion and have exceeded
	// the destroy TTL.
	toDelete, err := c.db.ListSecretsForType(typ, database.Unscoped())
	if err != nil {
		return fmt.Errorf("failed to list unscoped secrets for type %s: %w", typ, err)
	}
	for _, secret := range toDelete {
		deletedAt := secret.DeletedAt
		if deletedAt != nil {
			t := deletedAt.UTC()
			deletedAt = &t
		}

		if deletedAt != nil && now.Sub(*deletedAt) > c.config.SecretDestroyTTL {
			logger.Infow("purging expired secret", "secret", secret)

			if err := c.destroyUpstreamSecretVersion(ctx, secret.Reference); err != nil {
				return fmt.Errorf("failed to destroy %s: %w", secret.Reference, err)
			}

			if err := c.db.PurgeSecret(secret, RotationActor); err != nil {
				return fmt.Errorf("failed to purge secret %d: %w", secret.ID, err)
			}
		}
	}

	return nil
}

// createUpstreamSecretVersion creates an upstream secret version in the secret
// manager.
func (c *Controller) createUpstreamSecretVersion(ctx context.Context, name string, numBytes int) (string, error) {
	b, err := project.RandomBytes(numBytes)
	if err != nil {
		return "", fmt.Errorf("failed to generate %d, bytes for secret: %w", numBytes, err)
	}

	pth := path.Join(c.config.SecretsParent, name)
	version, err := c.secretManager.CreateSecretVersion(ctx, pth, b)
	if err != nil {
		return "", fmt.Errorf("failed to add secret version: %w", err)
	}
	return version, nil
}

func (c *Controller) destroyUpstreamSecretVersion(ctx context.Context, name string) error {
	if err := c.secretManager.DestroySecretVersion(ctx, name); err != nil {
		return fmt.Errorf("failed to destroy upstream secret version: %w", err)
	}
	return nil
}
