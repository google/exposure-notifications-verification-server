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
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

func TestConfigureCSRF(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	h, err := render.New(ctx, envstest.ServerAssetsPath(), true)
	if err != nil {
		t.Fatal(err)
	}

	key, err := project.RandomBytes(32)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.ServerConfig{
		CSRFAuthKey: key,
	}

	configureCSRF := middleware.ConfigureCSRF(cfg, h)

	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Accept", "application/json")

	w := httptest.NewRecorder()

	// Verify the proper fields are added to the template map.
	configureCSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		m := controller.TemplateMapFromContext(ctx)

		if _, ok := m["csrfField"]; !ok {
			t.Errorf("expected csrfField to be populated in template map")
		}
		if _, ok := m["csrfToken"]; !ok {
			t.Errorf("expected csrfToken to be populated in template map")
		}
		if _, ok := m["csrfMeta"]; !ok {
			t.Errorf("expected csrfMeta to be populated in template map")
		}
	})).ServeHTTP(w, r)
}
