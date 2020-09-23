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
	"context"
	"net"
	"net/http"
	"strings"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/google/exposure-notifications-server/pkg/logging"

	"github.com/gorilla/mux"
)

// ProcessFirewall verifies the application-level firewall configuration.
//
// This must come after the realm has been loaded in the context, probably via a
// different middleware.
func ProcessFirewall(ctx context.Context, h *render.Renderer, typ string) mux.MiddlewareFunc {
	logger := logging.FromContext(ctx).Named("middleware.ProcessFirewall")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			realm := controller.RealmFromContext(ctx)
			if realm == nil {
				controller.MissingRealm(w, r, h)
				return
			}

			var allowedCIDRs []string
			switch typ {
			case "adminapi":
				allowedCIDRs = realm.AllowedCIDRsAdminAPI
			case "apiserver":
				allowedCIDRs = realm.AllowedCIDRsAPIServer
			case "server":
				allowedCIDRs = realm.AllowedCIDRsServer
			}

			// If there's no CIDRs, all traffic is allowed.
			if len(allowedCIDRs) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			logger.Debugw("validating ip in cidr block", "type", typ)

			// Get the remote address.
			ipStr := r.RemoteAddr

			// Check if x-forwarded-for exists, the load balancer sets this, and the
			// first entry is the real client IP.
			xff := r.Header.Get("x-forwarded-for")
			if xff != "" {
				ipStr = strings.Split(xff, ",")[0]
			}

			ip := net.ParseIP(ipStr)
			for _, c := range allowedCIDRs {
				_, cidr, err := net.ParseCIDR(c)
				if err != nil {
					logger.Warnw("failed to parse cidr", "cidr", c, "error", err)
					continue
				}

				if cidr.Contains(ip) {
					next.ServeHTTP(w, r)
					return
				}
			}

			logger.Errorw("ip is not in an allowed cidr block")
			controller.Unauthorized(w, r, h)
		})
	}
}
