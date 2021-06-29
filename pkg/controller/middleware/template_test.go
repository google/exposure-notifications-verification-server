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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
)

func TestPopulateTemplateVariables(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	cfg := &config.ServerConfig{
		ServerName:      "namey",
		MaintenanceMode: true,
	}
	if err := cfg.Process(ctx); err != nil {
		t.Fatal(err)
	}
	populateTemplateVariables := middleware.PopulateTemplateVariables(cfg)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = r.Clone(ctx)
	r.Header.Set("Accept", "application/json")

	w := httptest.NewRecorder()

	// Verify the proper fields are added to the template map.
	populateTemplateVariables(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		m := controller.TemplateMapFromContext(ctx)

		if got, want := m["server"], cfg.ServerName; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
		if got, want := m["title"], cfg.ServerName; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
		if _, ok := m["buildID"]; !ok {
			t.Errorf("expected buildID to be populated in template map")
		}
		if _, ok := m["buildTag"]; !ok {
			t.Errorf("expected buildTag to be populated in template map")
		}
		if got, want := m["systemNotice"].(string), "undergoing maintenance"; !strings.Contains(got, want) {
			t.Errorf("expected %q to contain %q", got, want)
		}
	})).ServeHTTP(w, r)
}
