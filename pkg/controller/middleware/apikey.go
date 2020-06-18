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
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"

	"github.com/google/exposure-notifications-server/pkg/cache"

	"github.com/gin-gonic/gin"
)

const (
	// APIKeyHeader is the authorization header requred for APIKey protected requests.
	APIKeyHeader = "X-API-Key"
)

// APIKeyAuth returns a gin Middleware function that reads the X-API-Key HTTP header
// and checkes it against the authorized apps. The provided cache is used as a
// write through cache.
func APIKeyAuth(ctx context.Context, db *database.Database, keyCache *cache.Cache) gin.HandlerFunc {
	logger := logging.FromContext(ctx)
	return func(c *gin.Context) {
		apiKey := c.Request.Header.Get(APIKeyHeader)
		if apiKey == "" {
			logger.Errorf("missing %s header", APIKeyHeader)
			c.JSON(http.StatusUnauthorized, api.ErrorReturn{Error: fmt.Sprintf("invalid request: missing %s header", APIKeyHeader)})
			c.Abort()
			return
		}

		// Load the authorized app by API key using the write thru cache.
		authAppCache, err := keyCache.WriteThruLookup(apiKey,
			func() (interface{}, error) {
				aa, err := db.FindAuthoirizedAppByAPIKey(apiKey)
				if err != nil {
					return nil, err
				}
				return aa, nil
			})
		if err != nil {
			logger.Errorf("unable to lookup authorized app for apikey: %v", apiKey)
			c.JSON(http.StatusUnauthorized, api.ErrorReturn{Error: "invalid API Key"})
			c.Abort()
			return
		}
		authApp, ok := authAppCache.(*database.AuthorizedApp)
		if !ok {
			authApp = nil
		}

		// Check if the API key is authorized.
		if err != nil || authApp == nil || authApp.DeletedAt != nil {
			logger.Errorf("unauthorized API Key: %v err: %v", apiKey, err)
			c.JSON(http.StatusUnauthorized, api.ErrorReturn{Error: "unauthorized: API Key invalid"})
			c.Abort()
			return
		}

		c.Set("authorizedapp", authApp)

		c.Next()
	}
}
