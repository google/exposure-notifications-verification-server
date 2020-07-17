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

	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"
	httpcontext "github.com/gorilla/context"

	"github.com/google/exposure-notifications-server/pkg/cache"
)

const (
	// APIKeyHeader is the authorization header required for APIKey protected requests.
	APIKeyHeader = "X-API-Key"
)

type APIKeyMiddleware struct {
	ctx          context.Context
	db           *database.Database
	keyCache     *cache.Cache
	allowedTypes map[database.APIUserType]struct{}
}

// APIKeyAuth returns a gin Middleware function that reads the X-API-Key HTTP header
// and checks it against the authorized apps. The provided cache is used as a
// write through cache.
func APIKeyAuth(ctx context.Context, db *database.Database, keyCache *cache.Cache, allowedTypes ...database.APIUserType) *APIKeyMiddleware {
	cfg := APIKeyMiddleware{ctx, db, keyCache, make(map[database.APIUserType]struct{})}
	for _, t := range allowedTypes {
		cfg.allowedTypes[t] = struct{}{}
	}
	return &cfg
}

func (a *APIKeyMiddleware) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := logging.FromContext(a.ctx)
		apiKey := r.Header.Get(APIKeyHeader)
		if apiKey == "" {
			logger.Errorf("missing %s header", APIKeyHeader)
			controller.WriteJSON(w, http.StatusUnauthorized, api.ErrorReturn{Error: fmt.Sprintf("invalid request: missing %s header", APIKeyHeader)})
			return
		}

		// Load the authorized app by API key using the write thru cache.
		authAppCache, err := a.keyCache.WriteThruLookup(apiKey,
			func() (interface{}, error) {
				aa, err := a.db.FindAuthorizedAppByAPIKey(apiKey)
				if err != nil {
					return nil, err
				}
				return aa, nil
			})
		if err != nil {
			logger.Errorf("unable to lookup authorized app for apikey: %v", apiKey)
			controller.WriteJSON(w, http.StatusUnauthorized, api.ErrorReturn{Error: "invalid API Key"})
			return
		}
		authApp, ok := authAppCache.(*database.AuthorizedApp)
		if !ok {
			authApp = nil
		}

		// Check if the API key is authorized.
		if err != nil || authApp == nil || authApp.DeletedAt != nil {
			logger.Errorf("unauthorized API Key: %v err: %v", apiKey, err)
			controller.WriteJSON(w, http.StatusUnauthorized, api.ErrorReturn{Error: "unauthorized: API Key invalid"})
			return
		}
		if _, ok := a.allowedTypes[authApp.APIKeyType]; !ok {
			logger.Errorf("authorized API key is making wrong type of request: allowed: %v got: %v", a.allowedTypes, authApp.APIKeyType)
			controller.WriteJSON(w, http.StatusUnauthorized, api.ErrorReturn{Error: "unauthorized"})
			return
		}

		httpcontext.Set(r, "authorizedApp", authApp)

		next.ServeHTTP(w, r)
	})
}
