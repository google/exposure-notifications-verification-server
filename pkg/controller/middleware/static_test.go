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

package middleware_test

import (
	"net/http"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
)

func TestConfigureStaticAssets(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	handler := middleware.ConfigureStaticAssets(false)(emptyHandler())

	w, r := envstest.BuildJSONRequest(ctx, t, http.MethodGet, "/", nil)
	handler.ServeHTTP(w, r)

	if got, want := w.Header().Get("Cache-Control"), "public, max-age=604800"; got != want {
		t.Errorf("expected %q to be %q", got, want)
	}
	if got := w.Header().Get("Expires"); got == "" {
		t.Errorf("expected Expires to be set")
	}
	if got, want := w.Header().Get("Vary"), "Accept-Encoding"; got != want {
		t.Errorf("expected %q to be %q", got, want)
	}
}
