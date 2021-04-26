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
	"net/http"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/pkg/secrets"
	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

func TestHandleRotateSecrets(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	secretManager, err := secrets.NewInMemory(ctx, &secrets.Config{})
	if err != nil {
		t.Fatal(err)
	}
	secretManagerTyp, ok := secretManager.(secrets.SecretVersionManager)
	if !ok {
		t.Fatal("secret manager cannot manage versions")
	}

	h, err := render.New(ctx, nil, true)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("too_early", func(t *testing.T) {
		t.Parallel()

		db, _ := testDatabaseInstance.NewDatabase(t, nil)

		cfg := &config.RotationConfig{
			MinTTL: 5 * time.Minute,
		}

		c := New(cfg, db, nil, secretManagerTyp, h)

		w, r := envstest.BuildJSONRequest(ctx, t, http.MethodGet, "/", nil)

		c.HandleRotateSecrets().ServeHTTP(w, r)
		if got, want := w.Code, http.StatusOK; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}

		// again
		c.HandleRotateSecrets().ServeHTTP(w, r)
		if got, want := w.Code, http.StatusOK; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("database_error", func(t *testing.T) {
		t.Parallel()

		db, _ := testDatabaseInstance.NewDatabase(t, nil)
		db.SetRawDB(envstest.NewFailingDatabase())

		cfg := &config.RotationConfig{}

		c := New(cfg, db, nil, secretManagerTyp, h)

		w, r := envstest.BuildJSONRequest(ctx, t, http.MethodGet, "/", nil)
		c.HandleRotateSecrets().ServeHTTP(w, r)

		if got, want := w.Code, http.StatusInternalServerError; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})
}

func TestRotateSecret(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	secretManager, err := secrets.NewInMemory(ctx, &secrets.Config{})
	if err != nil {
		t.Fatal(err)
	}
	secretManagerTyp, ok := secretManager.(secrets.SecretVersionManager)
	if !ok {
		t.Fatal("secret manager cannot manage versions")
	}

	h, err := render.New(ctx, nil, true)
	if err != nil {
		t.Fatal(err)
	}

	cfg := &config.RotationConfig{
		SecretsParent:       "test/rotation",
		SecretActivationTTL: 500 * time.Millisecond,
		SecretDestroyTTL:    500 * time.Millisecond,
	}

	t.Run("rotates", func(t *testing.T) {
		t.Parallel()

		typ := database.SecretTypeCookieKeys
		parent := "my-secret"
		numBytes := 96

		db, _ := testDatabaseInstance.NewDatabase(t, nil)

		c := New(cfg, db, nil, secretManagerTyp, h)

		// Clear secrets created by bootstrap
		if err := db.RawDB().Unscoped().Delete(&database.Secret{}).Error; err != nil {
			t.Fatal(err)
		}

		var initialSecret *database.Secret

		// No secrets, so initial value should be created and active.
		{
			if err := c.rotateSecret(ctx, typ, parent, numBytes, 1*time.Nanosecond, 0); err != nil {
				t.Fatal(err)
			}
			secrets, err := db.ListSecrets()
			if err != nil {
				t.Fatal(err)
			}
			if got, want := len(secrets), 1; got != want {
				t.Fatalf("expected %d secret, got %d: %#v", want, got, secrets)
			}

			initialSecret = secrets[0]
			if got, want := initialSecret.Active, true; got != want {
				t.Errorf("expected %t to be %t: %#v", got, want, initialSecret)
			}
		}

		// Rotating again where minTTL has not elapsed does nothing.
		{
			if err := c.rotateSecret(ctx, typ, parent, numBytes, 5*time.Second, 0); err != nil {
				t.Fatal(err)
			}
			secrets, err := db.ListSecrets()
			if err != nil {
				t.Fatal(err)
			}
			if got, want := len(secrets), 1; got != want {
				t.Fatalf("expected %d secret, got %d: %#v", want, got, secrets)
			}
			if got, want := secrets[0].Active, true; got != want {
				t.Errorf("expected %t to be %t: %#v", got, want, secrets[0])
			}
		}

		// Rotate again where minTTL has elapsed generates a new secret.
		{
			if err := c.rotateSecret(ctx, typ, parent, numBytes, 1*time.Nanosecond, 0); err != nil {
				t.Fatal(err)
			}
			secrets, err := db.ListSecrets()
			if err != nil {
				t.Fatal(err)
			}
			if got, want := len(secrets), 2; got != want {
				t.Fatalf("expected %d secret, got %d: %#v", want, got, secrets)
			}

			// The activation TTL has not elapsed, so the first secret should still be
			// active, but the newly-created secret should be inactive.
			if got, want := secrets[0].Active, true; got != want {
				t.Errorf("expected %t to be %t: %#v", got, want, secrets[0])
			}
			if got, want := secrets[1].Active, false; got != want {
				t.Errorf("expected %t to be %t: %#v", got, want, secrets[1])
			}
		}

		// Rotate again should do nothing.
		{
			if err := c.rotateSecret(ctx, typ, parent, numBytes, 10*time.Second, 0); err != nil {
				t.Fatal(err)
			}
			secrets, err := db.ListSecrets()
			if err != nil {
				t.Fatal(err)
			}
			if got, want := len(secrets), 2; got != want {
				t.Fatalf("expected %d secret, got %d: %#v", want, got, secrets)
			}

			// The activation TTL has not elapsed, so the first secret should still be
			// active, but the newly-created secret should be inactive.
			if got, want := secrets[0].Active, true; got != want {
				t.Errorf("expected %t to be %t: %#v", got, want, secrets[0])
			}
			if got, want := secrets[1].Active, false; got != want {
				t.Errorf("expected %t to be %t: %#v", got, want, secrets[1])
			}
		}

		// Wait for activation delay, all secrets should be active.
		time.Sleep(cfg.SecretActivationTTL + 100*time.Millisecond)
		{
			if err := c.rotateSecret(ctx, typ, parent, numBytes, 0, 0); err != nil {
				t.Fatal(err)
			}
			secrets, err := db.ListSecrets()
			if err != nil {
				t.Fatal(err)
			}
			if got, want := len(secrets), 2; got != want {
				t.Fatalf("expected %d secret, got %d: %#v", want, got, secrets)
			}

			// The activation TTL has elapsed, so both should be active.
			if got, want := secrets[0].Active, true; got != want {
				t.Errorf("expected %t to be %t: %#v", got, want, secrets[0])
			}
			if got, want := secrets[1].Active, true; got != want {
				t.Errorf("expected %t to be %t: %#v", got, want, secrets[1])
			}
		}

		// If the maxTTL has passed, secrets should be inactive.
		{
			// Use a no minTTL because we don't want to create a new version.
			if err := c.rotateSecret(ctx, typ, parent, numBytes, 0, 1*time.Nanosecond); err != nil {
				t.Fatal(err)
			}
			secrets, err := db.ListSecrets()
			if err != nil {
				t.Fatal(err)
			}
			if got, want := len(secrets), 2; got != want {
				t.Fatalf("expected %d secret, got %d: %#v", want, got, secrets)
			}

			// The maxTTL has elapsed, so both should be inactive.
			if got, want := secrets[0].Active, false; got != want {
				t.Errorf("expected %t to be %t: %#v", got, want, secrets[0])
			}
			if got, want := secrets[1].Active, false; got != want {
				t.Errorf("expected %t to be %t: %#v", got, want, secrets[1])
			}
		}

		// After a secret has been moved to inactive for more than the activation
		// delay, it should be marked for deletion.
		time.Sleep(cfg.SecretActivationTTL + 100*time.Millisecond)
		{
			if err := c.rotateSecret(ctx, typ, parent, numBytes, 0, 1*time.Nanosecond); err != nil {
				t.Fatal(err)
			}
			secrets, err := db.ListSecrets()
			if err != nil {
				t.Fatal(err)
			}
			if got, want := len(secrets), 0; got != want {
				t.Fatalf("expected %d secret, got %d: %#v", want, got, secrets)
			}

			// Secrets should still exist in secret manager though.
			if _, err := secretManagerTyp.GetSecretValue(ctx, initialSecret.Reference); err != nil {
				t.Fatal(err)
			}
		}

		// After a secret has been deleted for more than the destroy TTL, purge it.
		time.Sleep(cfg.SecretDestroyTTL + 100*time.Millisecond)
		{
			if err := c.rotateSecret(ctx, typ, parent, numBytes, 0, 0); err != nil {
				t.Fatal(err)
			}
			secrets, err := db.ListSecrets(database.Unscoped())
			if err != nil {
				t.Fatal(err)
			}
			if got, want := len(secrets), 0; got != want {
				t.Errorf("expected %d secret, got %d: %#v", want, got, secrets)
			}

			// Secrets should still exist in secret manager though.
			if _, err := secretManagerTyp.GetSecretValue(ctx, initialSecret.Reference); err == nil {
				t.Errorf("expected error, got %#v", err)
			}
		}
	})
}
