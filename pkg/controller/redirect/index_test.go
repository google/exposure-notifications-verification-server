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

package redirect_test

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
			"bad":    "nope",
			"realm1": "aa",
			"realm2": "bb",
			"realm3": "cc",
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
	realm2 := database.NewRealmWithDefaults("realm2")
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
	androidApp := &database.MobileApp{
		Name:    "app2",
		RealmID: realm2.ID,
		URL:     "https://app2.example.com/",
		OS:      database.OSTypeAndroid,
		AppID:   "com.example.app2",
		SHA:     "AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA",
	}
	if err := harness.Database.SaveMobileApp(androidApp, database.SystemTest); err != nil {
		t.Fatal(err)
	}

	// Create yet another realm with no apps, but enx is enabled
	realm3 := database.NewRealmWithDefaults("realm3")
	realm3.RegionCode = "cc"
	if err := harness.Database.SaveRealm(realm3, database.SystemTest); err != nil {
		t.Fatal(err)
	}
	// Enable ENX, bypassing validations because it's annoying to set all those
	// fields manually.
	if err := harness.Database.RawDB().
		Model(&database.Realm{}).
		Where("id = ?", realm3.ID).
		UpdateColumn("enable_en_express", true).
		Error; err != nil {
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

	// Don't follow redirects.
	client.CheckRedirect = func(r *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	// Bad path
	t.Run("bad_path", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(ctx, "GET", srv.URL+"/css/appiew/main/gift.css", nil)
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

	// No matching region returns a 404
	t.Run("no_matching_region", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(ctx, "GET", srv.URL, nil)
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

	// A matching region that doesn't point to a realm returns 404
	t.Run("matching_region_no_realm", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(ctx, "GET", srv.URL, nil)
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

	// A matching region that has no apps should 404
	t.Run("matching_region_no_apps", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(ctx, "GET", srv.URL+"/app?c=123456", nil)
		req.Host = "realm1"
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

	// Not a mobile user agent with no query redirects to marketing
	t.Run("not_mobile_user_agent", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(ctx, "GET", srv.URL, nil)
		req.Host = "realm2"
		req.Header.Set("User-Agent", "bananarama")
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if got, want := resp.StatusCode, 303; got != want {
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			t.Errorf("expected %d to be %d: %s", got, want, body)
		}

		exp := "https://www.google.com/covid19/exposurenotifications/"
		if got, want := resp.Header.Get("Location"), exp; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
	})

	// Not a mobile user agent with a code returns a 404
	t.Run("not_mobile_user_agent", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(ctx, "GET", srv.URL+"/app?c=123456", nil)
		req.Host = "realm2"
		req.Header.Set("User-Agent", "bananarama")
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

	// Android redirects
	t.Run("android_redirect", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(ctx, "GET", srv.URL+"/app?c=123456", nil)
		req.Host = "realm2"
		req.Header.Set("User-Agent", "android")
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if got, want := resp.StatusCode, 303; got != want {
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			t.Errorf("expected %d to be %d: %s", got, want, body)
		}

		exp := "intent://app?c=123456&r=BB#Intent;scheme=ens;package=com.example.app2;action=android.intent.action.VIEW;category=android.intent.category.BROWSABLE;S.browser_fallback_url=https%3A%2F%2Fapp2.example.com%2F;end"
		if got, want := resp.Header.Get("Location"), exp; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
	})

	// Android redirect where enx is enabled
	t.Run("android_redirect_enx", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(ctx, "GET", srv.URL+"/app?c=123456", nil)
		req.Host = "realm3"
		req.Header.Set("User-Agent", "android")
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if got, want := resp.StatusCode, 303; got != want {
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			t.Errorf("expected %d to be %d: %s", got, want, body)
		}

		if got, want := resp.Header.Get("Location"), "ens://app?c=123456&r=CC"; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
	})

	// iOS redirects
	t.Run("ios_redirect", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(ctx, "GET", srv.URL+"/app?c=123456", nil)
		req.Host = "realm2"
		req.Header.Set("User-Agent", "iphone")
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if got, want := resp.StatusCode, 303; got != want {
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			t.Errorf("expected %d to be %d: %s", got, want, body)
		}

		if got, want := resp.Header.Get("Location"), "https://app1.example.com/"; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
	})

	// iOS redirect when enx is enabled
	t.Run("ios_redirect_enx", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(ctx, "GET", srv.URL+"/app?c=123456", nil)
		req.Host = "realm3"
		req.Header.Set("User-Agent", "iphone")
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if got, want := resp.StatusCode, 303; got != want {
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			t.Errorf("expected %d to be %d: %s", got, want, body)
		}

		if got, want := resp.Header.Get("Location"), "ens://app?c=123456&r=CC"; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
	})
}
