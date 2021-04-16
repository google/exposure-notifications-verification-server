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
	"os"
	"path"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/pkg/base64util"
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
		env := "DB_APIKEY_DATABASE_KEY"
		if err := c.rotateSecret(ctx, typ, parent, 128, minTTL, maxTTL, env); err != nil {
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
		env := "DB_APIKEY_SIGNATURE_KEY"
		if err := c.rotateSecret(ctx, typ, parent, 128, minTTL, maxTTL, env); err != nil {
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
		env := "COOKIE_KEYS"
		if err := c.rotateSecret(ctx, typ, parent, 64+32, minTTL, maxTTL, env); err != nil {
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
		env := "DB_PHONE_HMAC_KEY"
		if err := c.rotateSecret(ctx, typ, parent, 128, minTTL, maxTTL, env); err != nil {
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
		env := "DB_VERIFICATION_CODE_DATABASE_KEY"
		if err := c.rotateSecret(ctx, typ, parent, 128, minTTL, maxTTL, env); err != nil {
			merr = multierror.Append(merr, fmt.Errorf("failed to rotate verification code database hmac key: %w", err))
			return
		}
	}()

	return merr.ErrorOrNil()
}

// rotateAPIKeyHMAC rotates the key used to HMAC API keys stored in the
// database.
func (c *Controller) rotateSecret(ctx context.Context, typ database.SecretType, parent string, numBytes int, minTTL, maxTTL time.Duration, env string) error {
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

	// If the secret exist in the environment, import.
	// TODO(sethvargo): remove after 0.28.0+
	if val := strings.TrimSpace(os.Getenv(env)); len(existing) == 0 && val != "" {
		var mutators []importMutatorFunc

		if typ == database.SecretTypeCookieKeys {
			// Hack, but cookie keys are special
			mutators = append(mutators, mutateCookieKeysSecrets())
		}

		created, err := c.importExistingSecretFromEnv(ctx, typ, parent, env, val, mutators...)
		if err != nil {
			return fmt.Errorf("failed to import secrets from environment: %w", err)
		}
		existing = append(existing, created...)
	}

	// If there is no latest secret, or if the latest secret has existed for
	// longer than the minimum age, create a new one.
	if minTTL > 0 && (len(existing) == 0 || now.Sub(existing[0].CreatedAt) > minTTL) {
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

	for _, secret := range existing {
		// Activate any secrets that are ready to be activated. Please don't try to
		// optimize this. Yes, this could be reduced into a single SQL statement,
		// but we want to ensure the logging and AuditLog exist for debugging.
		if !secret.Active && secret.CreatedAt == secret.UpdatedAt && now.Sub(secret.CreatedAt) > c.config.SecretActivationTTL {
			logger.Infow("activating secret", "secret", secret)

			secret.Active = true
			if err := c.db.SaveSecret(secret, RotationActor); err != nil {
				return fmt.Errorf("failed to activate secret %d: %w", secret.ID, err)
			}
		}

		// If a maximum TTL was given, check for expirations.
		if maxTTL > 0 {
			// If the secret has existed for longer than the maximum TTL, deactivate
			// it. This will move it to the end of the list. For HMAC-style secrets,
			// this means it will still be available to validate values as the cache
			// updates, but will not be used to HMAC new values.
			if secret.Active && now.Sub(secret.CreatedAt) > maxTTL {
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
			if !secret.Active && secret.CreatedAt != secret.UpdatedAt && now.Sub(secret.UpdatedAt) > c.config.SecretActivationTTL {
				logger.Infow("marking secret for deletion", "secret", secret)

				if err := c.db.DeleteSecret(secret, RotationActor); err != nil {
					return fmt.Errorf("failed to mark secret %d for deletion: %w", secret.ID, err)
				}
			}
		}
	}

	// Hard delete any secrets that are marked for deletion and have exceeded
	// the destroy TTL.
	toDelete, err := c.db.ListSecretsForType(typ, database.Unscoped())
	if err != nil {
		return fmt.Errorf("failed to list unscoped secrets for type %s: %w", typ, err)
	}
	for _, secret := range toDelete {
		if secret.DeletedAt != nil && now.Sub(*secret.DeletedAt) > c.config.SecretDestroyTTL {
			logger.Infow("purging expired secret", "secret", secret)

			if err := c.destroyUpstreamSecretVresion(ctx, secret.Reference); err != nil {
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

func (c *Controller) destroyUpstreamSecretVresion(ctx context.Context, name string) error {
	if err := c.secretManager.DestroySecretVersion(ctx, name); err != nil {
		return fmt.Errorf("failed to destroy upstream secret version: %w", err)
	}
	return nil
}

// importMutatorFunc mutates raw bytes resolved secrets.
type importMutatorFunc func([][]byte) ([][]byte, error)

// importExistingSecretFromEnv downloads and parses v1 secrets into v2 secrets.
func (c *Controller) importExistingSecretFromEnv(ctx context.Context, typ database.SecretType, secretName, envvar, envval string, mutators ...importMutatorFunc) ([]*database.Secret, error) {
	logger := logging.FromContext(ctx).Named("importExistingSecretFromEnv").
		With("type", typ).
		With("env", envvar).
		With("secretName", secretName)

	now := time.Now().UTC()

	// allSecretsBytes is the ordered list of all secrets in bytes format.
	allSecretsBytes := make([][]byte, 0, 4)

	// Iterate over each secret in the envvar, separated by commas. This collects
	// the raw secret values as bytes.
	parts := strings.Split(envval, ",")
	for _, part := range parts {
		// These should be in the form secret://...?target=file. Transform them
		// back into their normal form.
		ref := strings.TrimSpace(part)
		if !strings.HasPrefix(ref, "secret://") {
			continue
		}
		ref = strings.TrimPrefix(ref, "secret://")
		ref = strings.TrimSuffix(ref, "?target=file")

		logger.Infow("importing secret value", "ref", ref)

		// Get the initial secret value.
		val, err := c.secretManager.GetSecretValue(ctx, ref)
		if err != nil {
			return nil, fmt.Errorf("failed to access initial secret %s: %w", ref, err)
		}

		// v1 secrets are base64-encoded separated by commas.
		insideParts := strings.Split(val, ",")
		for i, insidePart := range insideParts {
			dec, err := base64util.DecodeString(insidePart)
			if err != nil {
				return nil, fmt.Errorf("failed to decode initial secret %s at %d: %w", ref, i, err)
			}
			allSecretsBytes = append(allSecretsBytes, dec)
		}
	}

	// Apply any mutations.
	for _, fn := range mutators {
		var err error
		allSecretsBytes, err = fn(allSecretsBytes)
		if err != nil {
			return nil, err
		}
	}

	// Now that we have all bytes, iterate and create the secrets.
	orderedRefs := make([]string, 0, len(allSecretsBytes))
	for _, b := range allSecretsBytes {
		parent := path.Join(c.config.SecretsParent, secretName)
		version, err := c.secretManager.CreateSecretVersion(ctx, parent, b)
		if err != nil {
			return nil, fmt.Errorf("failed to add secret version to %s: %w", parent, err)
		}
		orderedRefs = append(orderedRefs, version)
	}

	// Now that all secrets are created upstream, create the database records.
	created := make([]*database.Secret, 0, len(orderedRefs))
	for i, ref := range orderedRefs {
		logger.Infow("create secret reference in database", "ref", ref)

		secret := &database.Secret{
			Type:      typ,
			Reference: ref,
			Active:    true,

			CreatedAt: now.Add(time.Duration(-i) * time.Hour),
			UpdatedAt: now.Add(time.Duration(-i) * time.Hour),
		}
		if err := c.db.SaveSecret(secret, RotationActor); err != nil {
			return nil, fmt.Errorf("failed to save secret ref %s in database: %w", ref, err)
		}
		created = append(created, secret)
	}

	return created, nil
}

// mutateCookieKeysSecrets handles the special case of importing cookie keys.
func mutateCookieKeysSecrets() importMutatorFunc {
	return func(in [][]byte) ([][]byte, error) {
		// Sanity check even number of elements.
		if l := len(in); l%2 != 0 {
			return nil, fmt.Errorf("invalid number of cookie secret bytes (%d), expected even", l)
		}

		// Cookie keys are currently special as separate secrets, but v2 cookie keys
		// are stored in the same secret. This concatenates each HMAC key with its
		// encryption key to be stored in the same secret moving forward.
		out := make([][]byte, 0, len(in)/2)
		for i := 0; i < len(in); i += 2 {
			b := make([]byte, 0, len(in[i])+len(in[i+1]))
			b = append(b, in[i]...)
			b = append(b, in[i+1]...)
			out = append(out, b)
		}
		return out, nil
	}
}
