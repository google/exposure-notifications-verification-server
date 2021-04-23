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

package middleware

import (
	"net/http"
	"strings"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/gorilla/mux"
)

func AddOperatingSystemFromUserAgent() mux.MiddlewareFunc {
	userAgents := map[string]database.OSType{
		"darwin":                 database.OSTypeIOS,
		"iphone":                 database.OSTypeIOS,
		"alamofire":              database.OSTypeIOS,
		"dalvik":                 database.OSTypeAndroid,
		"androiddownloadmanager": database.OSTypeAndroid,
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			agent := strings.ToLower(r.UserAgent())

			osToSet := database.OSTypeUnknown
			for k, os := range userAgents {
				if strings.Contains(agent, k) {
					osToSet = os
					break
				}
			}

			ctx = controller.WithOperatingSystem(ctx, osToSet)
			r = r.Clone(ctx)

			next.ServeHTTP(w, r)
		})
	}
}
