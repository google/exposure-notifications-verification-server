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
	"encoding/base64"
	"fmt"
	"net/http"
	"reflect"
	"strings"
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
			if err := c.rotateSecret(ctx, typ, parent, numBytes, 1*time.Nanosecond, 0, ""); err != nil {
				t.Fatal(err)
			}
			secrets, err := db.ListSecrets()
			if err != nil {
				t.Fatal(err)
			}
			if got, want := len(secrets), 1; got != want {
				t.Errorf("expected %d secret, got %d: %#v", want, got, secrets)
			}

			initialSecret = secrets[0]
			if got, want := initialSecret.Active, true; got != want {
				t.Errorf("expected %t to be %t: %#v", got, want, initialSecret)
			}
		}

		// Rotating again where minTTL has not elapsed does nothing.
		{
			if err := c.rotateSecret(ctx, typ, parent, numBytes, 5*time.Second, 0, ""); err != nil {
				t.Fatal(err)
			}
			secrets, err := db.ListSecrets()
			if err != nil {
				t.Fatal(err)
			}
			if got, want := len(secrets), 1; got != want {
				t.Errorf("expected %d secret, got %d: %#v", want, got, secrets)
			}
			if got, want := secrets[0].Active, true; got != want {
				t.Errorf("expected %t to be %t: %#v", got, want, secrets[0])
			}
		}

		// Rotate again where minTTL has elapsed generates a new secret.
		{
			if err := c.rotateSecret(ctx, typ, parent, numBytes, 1*time.Nanosecond, 0, ""); err != nil {
				t.Fatal(err)
			}
			secrets, err := db.ListSecrets()
			if err != nil {
				t.Fatal(err)
			}
			if got, want := len(secrets), 2; got != want {
				t.Errorf("expected %d secret, got %d: %#v", want, got, secrets)
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
			if err := c.rotateSecret(ctx, typ, parent, numBytes, 0, 0, ""); err != nil {
				t.Fatal(err)
			}
			secrets, err := db.ListSecrets()
			if err != nil {
				t.Fatal(err)
			}
			if got, want := len(secrets), 2; got != want {
				t.Errorf("expected %d secret, got %d: %#v", want, got, secrets)
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
			if err := c.rotateSecret(ctx, typ, parent, numBytes, 0, 1*time.Nanosecond, ""); err != nil {
				t.Fatal(err)
			}
			secrets, err := db.ListSecrets()
			if err != nil {
				t.Fatal(err)
			}
			if got, want := len(secrets), 2; got != want {
				t.Errorf("expected %d secret, got %d: %#v", want, got, secrets)
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
			if err := c.rotateSecret(ctx, typ, parent, numBytes, 0, 1*time.Nanosecond, ""); err != nil {
				t.Fatal(err)
			}
			secrets, err := db.ListSecrets()
			if err != nil {
				t.Fatal(err)
			}
			if got, want := len(secrets), 0; got != want {
				t.Errorf("expected %d secret, got %d: %#v", want, got, secrets)
			}

			// Secrets should still exist in secret manager though.
			if _, err := secretManagerTyp.GetSecretValue(ctx, initialSecret.Reference); err != nil {
				t.Fatal(err)
			}
		}

		// After a secret has been deleted for more than the destroy TTL, purge it.
		time.Sleep(cfg.SecretDestroyTTL + 100*time.Millisecond)
		{
			if err := c.rotateSecret(ctx, typ, parent, numBytes, 0, 0, ""); err != nil {
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

func TestImportExistingSecretFromEnv(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	h, err := render.New(ctx, nil, true)
	if err != nil {
		t.Fatal(err)
	}

	cfg := &config.RotationConfig{
		SecretsParent: "test/rotation",
	}

	secretManager, err := secrets.NewInMemory(ctx, &secrets.Config{})
	if err != nil {
		t.Fatal(err)
	}
	secretManagerTyp, ok := secretManager.(secrets.SecretVersionManager)
	if !ok {
		t.Fatal("secret manager cannot manage versions")
	}
	notBase64Ref, err := secretManagerTyp.CreateSecretVersion(ctx, "not-b64",
		[]byte("%"))
	if err != nil {
		t.Fatal(err)
	}
	singleValueRef, err := secretManagerTyp.CreateSecretVersion(ctx, "single-value",
		[]byte(base64.StdEncoding.EncodeToString([]byte("hello"))))
	if err != nil {
		t.Fatal(err)
	}
	multiValueRef, err := secretManagerTyp.CreateSecretVersion(ctx, "multi-value",
		[]byte(base64.StdEncoding.EncodeToString([]byte("hello"))+","+base64.StdEncoding.EncodeToString([]byte("world"))))
	if err != nil {
		t.Fatal(err)
	}

	cookieEncryptionKeyValueRef, err := secretManagerTyp.CreateSecretVersion(ctx, "cookie-encryption-key",
		[]byte(base64.StdEncoding.EncodeToString([]byte("thisisroughly32charactersright??"))))
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name     string
		envval   string
		mutators []importMutatorFunc
		exp      int
		err      string
	}{
		{
			name:   "empty",
			envval: "",
		},
		{
			name:   "missing_secret",
			envval: "secret://foo",
			err:    "failed to access initial secret",
		},
		{
			name:   "secret_not_base64",
			envval: fmt.Sprintf("secret://%s", notBase64Ref),
			err:    "failed to decode initial secret",
		},
		{
			name:   "single_value",
			envval: fmt.Sprintf("secret://%s", singleValueRef),
			exp:    1,
		},
		{
			name:   "multi_value",
			envval: fmt.Sprintf("secret://%s", multiValueRef),
			exp:    2,
		},
		{
			name:   "multi_env_value",
			envval: fmt.Sprintf("secret://%s,secret://%s?target=file", singleValueRef, multiValueRef),
			exp:    3,
		},
		{
			// The cookie mutator should combine 2 values into 1.
			name:     "cookie_mutator",
			envval:   fmt.Sprintf("secret://%s,secret://%s", singleValueRef, cookieEncryptionKeyValueRef),
			mutators: []importMutatorFunc{mutateCookieKeysSecrets()},
			exp:      1,
		},
		{
			name:     "cookie_mutator_odd",
			envval:   fmt.Sprintf("secret://%s,secret://%s", singleValueRef, multiValueRef),
			mutators: []importMutatorFunc{mutateCookieKeysSecrets()},
			err:      "invalid number of cookie secret bytes (3)",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			db, _ := testDatabaseInstance.NewDatabase(t, nil)
			c := New(cfg, db, nil, secretManagerTyp, h)

			typ := database.SecretTypeCookieKeys
			result, err := c.importExistingSecretFromEnv(ctx, typ, tc.name, "env", tc.envval, tc.mutators...)
			if err != nil {
				if tc.err == "" {
					t.Fatal(err)
				}

				if got, want := err.Error(), tc.err; !strings.Contains(got, want) {
					t.Errorf("expected %q to contain %q", got, want)
				}
			} else {
				if tc.err != "" {
					t.Fatalf("expected error %q, got nothing", tc.err)
				}
			}

			if got, want := len(result), tc.exp; got != want {
				t.Errorf("expected %d secrets, got %d: %#v", want, got, result)
			}
		})
	}
}

func TestMutateCookieKeysSecrets(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   [][]byte
		out  [][]byte
		err  bool
	}{
		{
			name: "empty",
			in:   nil,
			out:  [][]byte{},
		},
		{
			name: "odd",
			in:   [][]byte{[]byte("hi")},
			err:  true,
		},
		{
			name: "merges",
			in:   [][]byte{[]byte("hello"), []byte("thisisroughly32charactersright??")},
			out:  [][]byte{[]byte("thisisroughly32charactersright??hello")},
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := mutateCookieKeysSecrets()(tc.in)
			if (err != nil) != tc.err {
				t.Fatal(err)
			}

			if got, want := got, tc.out; !reflect.DeepEqual(got, want) {
				t.Errorf("expected %#v to be %#v", got, want)
			}
		})
	}
}
