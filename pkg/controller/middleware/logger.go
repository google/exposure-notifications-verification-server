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

package middleware

import (
	"net/http"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"go.uber.org/zap"

	"github.com/gorilla/mux"
)

// PopulateLogger populates the logger onto the context.
func PopulateLogger(originalLogger *zap.SugaredLogger) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			logger := originalLogger

			// If there's a request ID, set that on the logger.
			if id := controller.RequestIDFromContext(ctx); id != "" {
				logger = logger.With("request_id", id)
			}

			ctx = logging.WithLogger(ctx, logger)
			r = r.Clone(ctx)

			next.ServeHTTP(w, r)
		})
	}
}
