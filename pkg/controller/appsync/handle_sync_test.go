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

package appsync

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/clients"
	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

func TestHandleSync(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	h, err := render.New(ctx, nil, true)
	if err != nil {
		t.Fatal(err)
	}

	srv := testAppSyncServer(t)
	t.Cleanup(func() {
		srv.Close()
	})

	cfg := &config.AppSyncConfig{
		AppSyncURL:         srv.URL,
		FileSizeLimitBytes: 64000,
		AppSyncMinPeriod:   5 * time.Minute,
	}

	t.Run("syncs", func(t *testing.T) {
		t.Parallel()

		db, _ := testDatabaseInstance.NewDatabase(t, nil)

		c, err := New(cfg, db, h)
		if err != nil {
			t.Fatal(err)
		}

		w, r := envstest.BuildJSONRequest(ctx, t, http.MethodGet, "/", nil)

		c.HandleSync().ServeHTTP(w, r)
		if got, want := w.Code, http.StatusOK; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}

		// again
		c.HandleSync().ServeHTTP(w, r)
		if got, want := w.Code, http.StatusOK; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("internal_error", func(t *testing.T) {
		t.Parallel()

		db, _ := testDatabaseInstance.NewDatabase(t, nil)

		cfg := &config.AppSyncConfig{
			AppSyncURL:         "totally invalid",
			FileSizeLimitBytes: 64000,
			AppSyncMinPeriod:   5 * time.Minute,
		}

		c, err := New(cfg, db, h)
		if err != nil {
			t.Fatal(err)
		}

		w, r := envstest.BuildJSONRequest(ctx, t, http.MethodGet, "/", nil)

		c.HandleSync().ServeHTTP(w, r)
		if got, want := w.Code, http.StatusInternalServerError; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("database_error", func(t *testing.T) {
		t.Parallel()

		db, _ := testDatabaseInstance.NewDatabase(t, nil)
		db.SetRawDB(envstest.NewFailingDatabase())

		c, err := New(cfg, db, h)
		if err != nil {
			t.Fatal(err)
		}

		w, r := envstest.BuildJSONRequest(ctx, t, http.MethodGet, "/", nil)

		c.HandleSync().ServeHTTP(w, r)
		if got, want := w.Code, http.StatusInternalServerError; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})
}

func testAppSyncServer(tb testing.TB) *httptest.Server {
	tb.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := &clients.AppsResponse{
			Apps: []clients.App{
				{
					Region: "US-WA",
					IsEnx:  true,
					AndroidTarget: clients.AndroidTarget{
						Namespace:              "android_app",
						PackageName:            "testAppID-butDifferent",
						SHA256CertFingerprints: "AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA",
					},
				}, {
					Region: "US-WA",
					IsEnx:  true,
					AndroidTarget: clients.AndroidTarget{
						Namespace:              "android_app",
						PackageName:            "testAppId2",
						SHA256CertFingerprints: "BB:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA",
					},
					AgencyColor: "#AABBCC",
					AgencyImage: "https://example.com/logo.png",
				},
			},
		}

		b, err := json.Marshal(result)
		if err != nil {
			tb.Fatal(b)
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, string(b))
	}))
}
