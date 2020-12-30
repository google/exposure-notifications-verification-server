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

package middleware_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mikehelmick/go-chaff"

	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

func TestProcessChaff(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	// Note: we intentionally override the header detector here to force the next
	// middleware to run.
	processChaff := middleware.ProcessChaff(db, chaff.New(), chaff.HeaderDetector("noops"))

	cases := []struct {
		name     string
		disabled bool
		headers  map[string]string
		incr     bool
	}{
		{
			name: "missing_header",
			headers: map[string]string{
				"foo": "bar",
			},
		},
		{
			name: "invalid_key",
			headers: map[string]string{
				middleware.ChaffHeader: "hello",
			},
		},
		{
			name:     "disabled",
			disabled: true,
			headers: map[string]string{
				middleware.ChaffHeader: middleware.ChaffDailyKey,
			},
		},
		{
			name: "daily",
			headers: map[string]string{
				middleware.ChaffHeader: middleware.ChaffDailyKey,
			},
			incr: true,
		},
	}

	for i, tc := range cases {
		i := i
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			realm := database.NewRealmWithDefaults(fmt.Sprintf("realm-%d", i))
			realm.DailyActiveUsersEnabled = !tc.disabled
			if err := db.SaveRealm(realm, database.SystemTest); err != nil {
				t.Fatal(err)
			}

			ctx := project.TestContext(t)
			ctx = controller.WithRealm(ctx, realm)

			r := httptest.NewRequest("GET", "/", nil)
			r = r.Clone(ctx)
			for k, v := range tc.headers {
				r.Header.Set(k, v)
			}

			w := httptest.NewRecorder()

			processChaff(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Give the goroutine a few seconds to finish writing. This will likely
				// be the source of a flakey test.
				if tc.incr {
					time.Sleep(3 * time.Second)
				}

				stats, err := realm.Stats(db)
				if err != nil {
					t.Fatal(err)
				}
				if len(stats) == 0 {
					t.Fatal("no stats")
				}
				stat := stats[0]

				if tc.incr {
					if got, want := stat.DailyActiveUsers, uint(1); got != want {
						t.Errorf("expected %d to be %d", got, want)
					}
				} else {
					if got, want := stat.DailyActiveUsers, uint(0); got != want {
						t.Errorf("expected %d to be %d", got, want)
					}
				}
			})).ServeHTTP(w, r)
		})
	}
}
