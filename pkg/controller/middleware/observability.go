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

package middleware

import (
	"context"
	"net/http"

	enobs "github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/internal/buildinfo"
	"github.com/gorilla/mux"
)

// WithObservability sets common observability context fields.
func WithObservability(ctx context.Context) (context.Context, mux.MiddlewareFunc) {
	ctx = enobs.WithBuildInfo(ctx, buildinfo.VerificationServer)

	return ctx, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := enobs.WithBuildInfo(r.Context(), buildinfo.VerificationServer)
			r = r.Clone(ctx)
			next.ServeHTTP(w, r)
		})
	}
}
