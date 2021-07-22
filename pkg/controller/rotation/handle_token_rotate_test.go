// Copyright 2021 the Exposure Notifications Verification Server authors
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

	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

func TestHandleRotateTokenSigningKey(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	keyManager := keys.TestKeyManager(t)
	keyManagerSigner, ok := keyManager.(keys.SigningKeyManager)
	if !ok {
		t.Fatal("kms cannot manage signing keys")
	}
	tokenSigningKey := keys.TestSigningKey(t, keyManager)

	h, err := render.New(ctx, nil, true)
	if err != nil {
		t.Fatal(err)
	}

	cfg := &config.RotationConfig{
		TokenSigning: config.TokenSigningConfig{
			TokenSigningKey: tokenSigningKey,
		},
		TokenSigningKeyMaxAge: 30 * time.Second,
	}

	t.Run("rotates", func(t *testing.T) {
		t.Parallel()

		db, _ := testDatabaseInstance.NewDatabase(t, nil)

		c := New(cfg, db, keyManagerSigner, nil, h)

		// Rotating should create a new key since none exists.
		{
			w, r := envstest.BuildJSONRequest(ctx, t, http.MethodGet, "/", nil)
			c.HandleRotateTokenSigningKey().ServeHTTP(w, r)

			keys, err := db.ListTokenSigningKeys()
			if err != nil {
				t.Fatal(err)
			}
			if got, want := len(keys), 1; got != want {
				t.Errorf("got %d keys, expected %d", got, want)
			}
		}

		// Rotating again should create a new key (after expiring the one just
		// created).
		{
			key, err := db.ActiveTokenSigningKey()
			if err != nil {
				t.Fatal(err)
			}
			key.CreatedAt = time.Now().UTC().Add(-24 * time.Hour)
			if err := db.SaveTokenSigningKey(key, database.SystemTest); err != nil {
				t.Fatal(err)
			}

			w, r := envstest.BuildJSONRequest(ctx, t, http.MethodGet, "/", nil)
			c.HandleRotateTokenSigningKey().ServeHTTP(w, r)

			keys, err := db.ListTokenSigningKeys()
			if err != nil {
				t.Fatal(err)
			}
			if got, want := len(keys), 2; got != want {
				t.Errorf("got %d keys, expected %d", got, want)
			}
		}

		// Rotating again should not create a new key (not enough time has elapsed
		// since TokenSigningKeyMaxAge).
		{
			w, r := envstest.BuildJSONRequest(ctx, t, http.MethodGet, "/", nil)
			c.HandleRotateTokenSigningKey().ServeHTTP(w, r)

			keys, err := db.ListTokenSigningKeys()
			if err != nil {
				t.Fatal(err)
			}
			if got, want := len(keys), 2; got != want {
				t.Errorf("got %d keys, expected %d", got, want)
			}
		}
	})

	t.Run("too_early", func(t *testing.T) {
		t.Parallel()

		db, _ := testDatabaseInstance.NewDatabase(t, nil)

		cfg := &config.RotationConfig{
			TokenSigning: config.TokenSigningConfig{
				TokenSigningKey: tokenSigningKey,
			},
			TokenSigningKeyMaxAge: 30 * time.Second,
			MinTTL:                5 * time.Minute,
		}

		c := New(cfg, db, keyManagerSigner, nil, h)

		w, r := envstest.BuildJSONRequest(ctx, t, http.MethodGet, "/", nil)

		c.HandleRotateTokenSigningKey().ServeHTTP(w, r)
		if got, want := w.Code, http.StatusOK; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}

		// again
		c.HandleRotateTokenSigningKey().ServeHTTP(w, r)
		if got, want := w.Code, http.StatusOK; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("database_error", func(t *testing.T) {
		t.Parallel()

		db, _ := testDatabaseInstance.NewDatabase(t, nil)
		db.SetRawDB(envstest.NewFailingDatabase())

		c := New(cfg, db, keyManagerSigner, nil, h)

		w, r := envstest.BuildJSONRequest(ctx, t, http.MethodGet, "/", nil)
		c.HandleRotateTokenSigningKey().ServeHTTP(w, r)

		if got, want := w.Code, http.StatusInternalServerError; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})
}
