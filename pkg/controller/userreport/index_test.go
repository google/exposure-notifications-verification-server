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

package userreport_test

import (
	"crypto/rand"
	"encoding/base64"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/clients"
	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/internal/routes"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/sms"
	"golang.org/x/net/publicsuffix"
)

func TestIndex(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewENXRedirectServerConfig(t, testDatabaseInstance)

	// Create config.
	cfg := &config.RedirectConfig{
		DevMode:        true,
		HostnameConfig: map[string]string{},

		Features: config.FeatureConfig{},
	}
	cfg.Issue.ENExpressRedirectDomain = "127.0.0.1"

	// Crate a realm.
	realm, err := harness.Database.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}
	realm.RegionCode = "us-wa"
	realm.AddUserReportToAllowedTestTypes()
	realm.AllowUserReportWebView = true
	realm.AllowAdminUserReport = true
	if err := harness.Database.SaveRealm(realm, database.SystemTest); err != nil {
		t.Fatal(err)
	}

	authApp := &database.AuthorizedApp{
		Name:       "test",
		APIKeyType: database.APIKeyTypeDevice,
	}
	apikey, err := realm.CreateAuthorizedApp(harness.Database, authApp, database.SystemTest)
	if err != nil {
		t.Fatal(err)
	}

	smsConfig := &database.SMSConfig{
		RealmID:      realm.ID,
		ProviderType: sms.ProviderTypeNoop,
	}
	if err := harness.Database.SaveSMSConfig(smsConfig); err != nil {
		t.Fatal(err)
	}

	// Build routes.
	mux, err := routes.ENXRedirect(ctx, cfg, harness.Database, harness.Cacher, harness.KeyManager, harness.RateLimiter)
	if err != nil {
		t.Fatal(err)
	}

	// Start server.
	srv := httptest.NewServer(mux)
	t.Cleanup(func() {
		srv.Close()
	})

	// Generate the nonce
	nonceBytes := make([]byte, database.NonceLength)
	_, err = rand.Read(nonceBytes)
	if err != nil {
		t.Fatal(err)
	}
	nonce := base64.URLEncoding.EncodeToString(nonceBytes)

	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		t.Fatal(err)
	}
	// initiate the request
	redirClient, err := clients.NewENXRedirectWebClient(srv.URL, apikey,
		clients.WithCookieJar(jar), clients.WithTimeout(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}

	if err := redirClient.SendUserReportIndex(ctx, nonce); err != nil {
		t.Fatalf("error requesting user report web view: %v", err)
	}
}
