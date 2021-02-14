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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

func TestHandleVerificationRotation(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	keyManager := keys.TestKeyManager(t)
	keyManagerSigner, ok := keyManager.(keys.SigningKeyManager)
	if !ok {
		t.Fatal("kms cannot manage signing keys")
	}

	h, err := render.New(ctx, "", true)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("rotates", func(t *testing.T) {
		t.Parallel()

		db, _ := testDatabaseInstance.NewDatabase(t, nil)

		realm := database.NewRealmWithDefaults("state")
		realm.AutoRotateCertificateKey = true
		realm.UseRealmCertificateKey = true
		realm.CertificateIssuer = "iss"
		realm.CertificateAudience = "aud"
		realm.CertificateDuration = database.FromDuration(time.Second)
		if err := db.SaveRealm(realm, database.SystemTest); err != nil {
			t.Fatal(err)
		}

		cfg := &config.RotationConfig{
			VerificationSigningKeyMaxAge: 10 * time.Second,
			VerificationActivationDelay:  2 * time.Second,
			MinTTL:                       time.Microsecond,
		}
		c := New(cfg, db, keyManagerSigner, h)

		// create the initial signing key version, which will make it active.
		if _, err := realm.CreateSigningKeyVersion(ctx, db, database.SystemTest); err != nil {
			t.Fatal(err)
		}
		// Initial state - 1 active signing key.
		checkKeys(t, db, realm, 1, 0)

		// Wait the max age, and run the test.
		time.Sleep(cfg.VerificationSigningKeyMaxAge + time.Second)
		invokeRotate(ctx, t, c)
		// There should be 2 keys on the realm now, the older one should still be the active one.
		checkKeys(t, db, realm, 2, 1)

		// Wait long enough for the activation delay.
		time.Sleep(cfg.VerificationActivationDelay + time.Second)
		invokeRotate(ctx, t, c)
		// There should still be 2 signing keys, but now the first one should be active.
		checkKeys(t, db, realm, 2, 0)

		// Wait long enough for original key to be deleted.
		time.Sleep(cfg.VerificationActivationDelay + time.Second)
		invokeRotate(ctx, t, c)
		// Original key should be destroyed, only 1 key and it's active now.
		checkKeys(t, db, realm, 1, 0)
	})

	t.Run("too_early", func(t *testing.T) {
		t.Parallel()

		db, _ := testDatabaseInstance.NewDatabase(t, nil)

		cfg := &config.RotationConfig{
			VerificationSigningKeyMaxAge: 2 * time.Second,
			VerificationActivationDelay:  1 * time.Second,
			MinTTL:                       5 * time.Minute,
		}

		c := New(cfg, db, keyManagerSigner, h)

		r, err := http.NewRequest(http.MethodGet, "/", nil)
		if err != nil {
			t.Fatal(err)
		}
		r = r.Clone(ctx)

		w := httptest.NewRecorder()

		c.HandleVerificationRotate().ServeHTTP(w, r)
		if got, want := w.Code, 200; got != want {
			t.Errorf("expected %d to be %d", got, want)
		}

		// again
		c.HandleVerificationRotate().ServeHTTP(w, r)
		if got, want := w.Code, 200; got != want {
			t.Errorf("expected %d to be %d", got, want)
		}
	})

	t.Run("database_error", func(t *testing.T) {
		t.Parallel()

		db, _ := testDatabaseInstance.NewDatabase(t, nil)
		db.SetRawDB(envstest.NewFailingDatabase())

		cfg := &config.RotationConfig{
			VerificationSigningKeyMaxAge: 2 * time.Second,
			VerificationActivationDelay:  1 * time.Second,
			MinTTL:                       time.Microsecond,
		}
		c := New(cfg, db, keyManagerSigner, h)

		r, err := http.NewRequest(http.MethodGet, "/", nil)
		if err != nil {
			t.Fatal(err)
		}
		r = r.Clone(ctx)

		w := httptest.NewRecorder()

		c.HandleVerificationRotate().ServeHTTP(w, r)

		if got, want := w.Code, 500; got != want {
			t.Errorf("expected %d to be %d", got, want)
		}
	})
}

func checkKeys(tb testing.TB, db *database.Database, realm *database.Realm, count, active int) {
	tb.Helper()

	keys, err := realm.ListSigningKeys(db)
	if err != nil {
		tb.Fatalf("listing signing keys: %v", err)
	}

	if l := len(keys); l != count {
		tb.Fatalf("expected key count wrong, want: %v got: %v", count, l)
	}
	if !keys[active].Active {
		tb.Fatalf("expected active key (%v) is not active", active)
	}
}

func invokeRotate(ctx context.Context, tb testing.TB, c *Controller) {
	tb.Helper()

	r, err := http.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	if err != nil {
		tb.Fatal(err)
	}

	w := httptest.NewRecorder()

	c.HandleVerificationRotate().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		tb.Fatalf("invoke didn't return success, status: %v", w.Code)
	}
}
