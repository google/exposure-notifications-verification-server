// Copyright 2020 the Exposure Notifications Verification Server authors
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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
)

func TestProcessDebug(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	processDebug := middleware.ProcessDebug()

	cases := []struct {
		name string
		exp  bool
	}{
		{
			name: "no_header",
			exp:  false,
		},
		{
			name: "header",
			exp:  true,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r = r.Clone(ctx)
			r.Header.Set("Content-Type", "text/html")

			if tc.exp {
				r.Header.Set(middleware.HeaderDebug, "1")
			}

			w := httptest.NewRecorder()
			processDebug(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.exp {
					id := w.Header().Get(middleware.HeaderDebugBuildID)
					if id == "" {
						t.Errorf("expected id to have a value")
					}

					tag := w.Header().Get(middleware.HeaderDebugBuildTag)
					if tag == "" {
						t.Errorf("expected tag to have a value")
					}
				}
			})).ServeHTTP(w, r)
		})
	}
}
