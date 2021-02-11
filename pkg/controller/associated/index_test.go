// Copyright 2020 Google LLC
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

package associated_test

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/internal/routes"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

func TestIndex(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServerConfig(t, testDatabaseInstance)

	// Create config.
	cfg := &config.RedirectConfig{
		AssetsPath: filepath.Join(project.Root(), "cmd", "enx-redirect", "assets"),
		DevMode:    true,
		HostnameConfig: map[string]string{
			"bad":   "nope",
			"empty": "aa",
			"okay":  "bb",
		},
	}

	// Set realm to resolve.
	realm1, err := harness.Database.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}
	realm1.RegionCode = "aa"
	if err := harness.Database.SaveRealm(realm1, database.SystemTest); err != nil {
		t.Fatal(err)
	}

	// Create another realm with apps.
	realm2 := database.NewRealmWithDefaults("okay")
	realm2.RegionCode = "bb"
	if err := harness.Database.SaveRealm(realm2, database.SystemTest); err != nil {
		t.Fatal(err)
	}

	// Create iOS app
	iosApp := &database.MobileApp{
		Name:    "app1",
		RealmID: realm2.ID,
		URL:     "https://app1.example.com/",
		OS:      database.OSTypeIOS,
		AppID:   "com.example.app1",
	}
	if err := harness.Database.SaveMobileApp(iosApp, database.SystemTest); err != nil {
		t.Fatal(err)
	}

	// Create Android app
	app2 := &database.MobileApp{
		Name:    "app2",
		RealmID: realm2.ID,
		URL:     "https://app2.example.com/",
		OS:      database.OSTypeAndroid,
		AppID:   "com.example.app2",
		SHA:     "AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA",
	}
	if err := harness.Database.SaveMobileApp(app2, database.SystemTest); err != nil {
		t.Fatal(err)
	}

	// Build routes.
	mux, err := routes.ENXRedirect(ctx, cfg, harness.Database, harness.Cacher)
	if err != nil {
		t.Fatal(err)
	}

	// Start server.
	srv := httptest.NewServer(mux)
	t.Cleanup(func() {
		srv.Close()
	})
	client := srv.Client()

	// The .well-known directory is a 404 and not a 500.
	t.Run("well-known_root_404", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(ctx, "GET", srv.URL+"/.well-known", nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if got, want := resp.StatusCode, 404; got != want {
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			t.Errorf("expected %d to be %d: %s", got, want, body)
		}
	})

	// Missing region is a 400
	t.Run("well-known_apple-app-site-association_missing_region", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(ctx, "GET", srv.URL+"/.well-known/apple-app-site-association", nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if got, want := resp.StatusCode, 404; got != want {
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			t.Errorf("expected %d to be %d: %s", got, want, body)
		}
	})

	// Unregistered region is a 400
	t.Run("well-known_apple-app-site-association_invalid_region", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(ctx, "GET", srv.URL+"/.well-known/apple-app-site-association", nil)
		req.Host = "not-real"
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if got, want := resp.StatusCode, 404; got != want {
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			t.Errorf("expected %d to be %d: %s", got, want, body)
		}
	})

	// Region in config that maps to a non-existent realm. In this test, "bad" is
	// in the config and points to a region code that isn't registered.
	t.Run("well-known_apple-app-site-association_misconfigured", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(ctx, "GET", srv.URL+"/.well-known/apple-app-site-association", nil)
		req.Host = "bad"
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if got, want := resp.StatusCode, 404; got != want {
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			t.Errorf("expected %d to be %d: %s", got, want, body)
		}
	})

	// Valid region with no apps is 404
	t.Run("well-known_apple-app-site-association_no_data", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(ctx, "GET", srv.URL+"/.well-known/apple-app-site-association", nil)
		req.Host = "empty"
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if got, want := resp.StatusCode, 404; got != want {
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			t.Errorf("expected %d to be %d: %s", got, want, body)
		}
	})

	// Valid region with apps is 200
	t.Run("well-known_apple-app-site-association_result", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(ctx, "GET", srv.URL+"/.well-known/apple-app-site-association", nil)
		req.Host = "okay"
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if got, want := resp.StatusCode, 200; got != want {
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			t.Errorf("expected %d to be %d: %s", got, want, body)
		}
	})

	// Missing region is a 400
	t.Run("well-known_assetlinksjson_missing_region", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(ctx, "GET", srv.URL+"/.well-known/assetlinks.json", nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if got, want := resp.StatusCode, 404; got != want {
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			t.Errorf("expected %d to be %d: %s", got, want, body)
		}
	})

	// Unregistered region is a 400
	t.Run("well-known_assetlinksjson_invalid_region", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(ctx, "GET", srv.URL+"/.well-known/assetlinks.json", nil)
		req.Host = "not-real"
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if got, want := resp.StatusCode, 404; got != want {
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			t.Errorf("expected %d to be %d: %s", got, want, body)
		}
	})

	// Region in config that maps to a non-existent realm. In this test, "bad" is
	// in the config and points to a region code that isn't registered.
	t.Run("well-known_assetlinksjson_misconfigured", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(ctx, "GET", srv.URL+"/.well-known/assetlinks.json", nil)
		req.Host = "bad"
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if got, want := resp.StatusCode, 404; got != want {
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			t.Errorf("expected %d to be %d: %s", got, want, body)
		}
	})

	// Valid region with no apps is 404
	t.Run("well-known_assetlinksjson_no_data", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(ctx, "GET", srv.URL+"/.well-known/assetlinks.json", nil)
		req.Host = "empty"
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if got, want := resp.StatusCode, 404; got != want {
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			t.Errorf("expected %d to be %d: %s", got, want, body)
		}
	})

	// Valid region with apps is 200
	t.Run("well-known_assetlinksjson_result", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(ctx, "GET", srv.URL+"/.well-known/assetlinks.json", nil)
		req.Host = "okay"
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if got, want := resp.StatusCode, 200; got != want {
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			t.Errorf("expected %d to be %d: %s", got, want, body)
		}
	})
}
