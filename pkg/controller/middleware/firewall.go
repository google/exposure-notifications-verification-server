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
	"net"
	"net/http"
	"strings"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/gorilla/mux"
)

// ProcessFirewall verifies the application-level firewall configuration.
//
// This must come after the realm has been loaded in the context, probably via a
// different middleware.
func ProcessFirewall(h *render.Renderer, typ string) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			logger := logging.FromContext(ctx).Named("middleware.ProcessFirewall")

			membership := controller.MembershipFromContext(ctx)
			if membership == nil {
				controller.MissingMembership(w, r, h)
				return
			}
			currentRealm := membership.Realm

			var allowedCIDRs []string
			switch typ {
			case "adminapi":
				allowedCIDRs = currentRealm.AllowedCIDRsAdminAPI
			case "apiserver":
				allowedCIDRs = currentRealm.AllowedCIDRsAPIServer
			case "server":
				allowedCIDRs = currentRealm.AllowedCIDRsServer
			default:
				logger.Errorw("unknown firewall type", "type", typ)
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

			// Parse as an IP.
			ip := net.ParseIP(ipStr)
			if ip == nil {
				logger.Errorw("provided ip could not be parsed")
			}

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
			return
		})
	}
}
