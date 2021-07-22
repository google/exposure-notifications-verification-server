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

package middleware

import (
	"net/http"
	"strings"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/gorilla/mux"
)

const (
	// NonceHeader is the header for the incoming nonce
	NonceHeader = "X-Nonce"
)

// ProcessNonce reads the X-Nonce header and stores it in the context.
func ProcessNonce(h *render.Renderer) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			nonce := strings.TrimSpace(r.Header.Get(NonceHeader))

			// Save the authorized app on the context, it may be empty.
			ctx = controller.WithNonce(ctx, nonce)
			r = r.Clone(ctx)

			next.ServeHTTP(w, r)
		})
	}
}
