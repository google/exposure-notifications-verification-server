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

// Package middleware contains application specific gin middleware functions.
package middleware

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"github.com/gorilla/mux"

	"github.com/google/exposure-notifications-server/pkg/cache"
)

const (
	// APIKeyHeader is the authorization header required for APIKey protected requests.
	APIKeyHeader = "X-API-Key"
)

// RequireAPIKey reads the X-API-Key header and validates it is a real
// authorized app. It also ensures currentAuthorizedApp is set in the template
// map.
func RequireAPIKey(ctx context.Context, cache *cache.Cache, db *database.Database, h *render.Renderer, allowedTypes []database.APIUserType) mux.MiddlewareFunc {
	logger := logging.FromContext(ctx).Named("middleware.RequireAPIKey")

	allowedTypesMap := make(map[database.APIUserType]struct{}, len(allowedTypes))
	for _, t := range allowedTypes {
		allowedTypesMap[t] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			apiKey := r.Header.Get(APIKeyHeader)
			if apiKey == "" {
				logger.Debugw("missing API key in request")
				controller.Unauthorized(w, r, h)
				return
			}

			// Load the authorized app by API key using the write thru cache.
			authAppCached, err := cache.WriteThruLookup(apiKey, func() (interface{}, error) {
				aa, err := db.FindAuthorizedAppByAPIKey(apiKey)
				if err != nil {
					return nil, err
				}
				if aa == nil {
					return nil, nil
				}
				return aa, nil
			})
			if err != nil {
				logger.Errorw("failed to lookup authorized app", "error", err)
				controller.InternalError(w, r, h, err)
				return
			}

			if authAppCached == nil {
				logger.Debugw("authorized app does not exist")
				controller.Unauthorized(w, r, h)
				return
			}

			authApp, ok := authAppCached.(*database.AuthorizedApp)
			if !ok {
				err := fmt.Errorf("expected %T to be *database.AuthorizedApp", authApp)
				logger.Errorw("failed to get authorized app from cache", "error", err)
				controller.InternalError(w, r, h, err)
				return
			}

			if authApp.DeletedAt != nil {
				logger.Debugw("authorized app is deleted")
				controller.Unauthorized(w, r, h)
				return
			}

			if _, ok := allowedTypesMap[authApp.APIKeyType]; !ok {
				logger.Debugw("wrong request type", "got", authApp.APIKeyType, "allowed", allowedTypes)
				controller.Unauthorized(w, r, h)
				return
			}

			// Save the authorizedapp in the template map.
			m := controller.TemplateMapFromContext(ctx)
			m["currentAuthorizedApp"] = authApp

			// Save the authorized app on the context.
			ctx = controller.WithAuthorizedApp(ctx, authApp)
			*r = *r.WithContext(ctx)

			logger.Debugw("done")
			next.ServeHTTP(w, r)
		})
	}
}
