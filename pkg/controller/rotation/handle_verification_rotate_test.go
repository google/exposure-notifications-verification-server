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
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

type testUser struct{}

func (t *testUser) AuditID() string {
	return "1"
}

func (t *testUser) AuditDisplay() string {
	return "system"
}

func TestHandleVerificationRotation(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	realm := database.NewRealmWithDefaults("state")
	realm.AutoRotateCertificateKey = true
	realm.UseRealmCertificateKey = true
	realm.CertificateIssuer = "iss"
	realm.CertificateAudience = "aud"
	realm.CertificateDuration = database.FromDuration(time.Second)
	if err := db.SaveRealm(realm, &testUser{}); err != nil {
		t.Fatal(err)
	}

	keyManager := keys.TestKeyManager(t)
	keyManagerSigner, ok := keyManager.(keys.SigningKeyManager)
	if !ok {
		t.Fatal("kms cannot manage signing keys")
	}

	h, err := render.New(ctx, "", true)
	if err != nil {
		t.Fatal(err)
	}

	config := &config.RotationConfig{
		VerificationSigningKeyMaxAge: 10 * time.Second,
		VerificationActivationDelay:  2 * time.Second,
		MinTTL:                       time.Microsecond,
	}
	c := New(config, db, keyManagerSigner, h)

	// create the initial signing key version, which will make it active.
	if _, err := realm.CreateSigningKeyVersion(ctx, db); err != nil {
		t.Fatal(err)
	}
	// Initial state - 1 active signing key.
	checkKeys(t, db, realm, 1, 0)

	// Wait the max age, and run the test.
	time.Sleep(config.VerificationSigningKeyMaxAge + time.Second)
	invokeRotate(t, ctx, c)
	// There should be 2 keys on the realm now, the older one should still be the active one.
	checkKeys(t, db, realm, 2, 1)

	// Wait long enough for the activation delay.
	time.Sleep(config.VerificationActivationDelay + time.Second)
	invokeRotate(t, ctx, c)
	// There should still be 2 signing keys, but now the first one should be active.
	checkKeys(t, db, realm, 2, 0)

	// Wait long enough for original key to be deleted.
	time.Sleep(config.VerificationActivationDelay + time.Second)
	invokeRotate(t, ctx, c)
	// Original key should be destroyed, only 1 key and it's active now.
	checkKeys(t, db, realm, 1, 0)
}

func checkKeys(t testing.TB, db *database.Database, realm *database.Realm, count, active int) {
	t.Helper()

	keys, err := realm.ListSigningKeys(db)
	if err != nil {
		t.Fatalf("listing signing keys: %v", err)
	}

	if l := len(keys); l != count {
		t.Fatalf("expected key count wrong, want: %v got: %v", count, l)
	}
	if !keys[active].Active {
		t.Fatalf("expected active key (%v) is not active", active)
	}
}

func invokeRotate(t testing.TB, ctx context.Context, c *Controller) {
	t.Helper()

	r, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	r = r.Clone(ctx)

	w := httptest.NewRecorder()

	c.HandleVerificationRotate().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("invoke didn't return success, status: %v", w.Code)
	}
}
