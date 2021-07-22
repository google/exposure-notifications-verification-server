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
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
)

func TestInjectCurrentPath(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	injectCurrentPath := middleware.InjectCurrentPath()

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = r.Clone(ctx)
	r.Header.Set("Accept", "application/json")

	w := httptest.NewRecorder()

	// Verify the proper fields are added to the template map.
	injectCurrentPath(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		m := controller.TemplateMapFromContext(ctx)

		pth, ok := m["currentPath"]
		if !ok {
			t.Fatalf("expected currentPath to be populated in template map")
		}

		if _, ok := pth.(*middleware.Path); !ok {
			t.Fatalf("expected %T to be *middleware.Path", pth)
		}
	})).ServeHTTP(w, r)
}
