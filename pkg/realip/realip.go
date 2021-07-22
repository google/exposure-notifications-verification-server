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

// Package realip attempts to extract the real IP address from a service running
// behind a load balancer.
package realip

import (
	"net/http"
	"strings"
)

const headerKeyXForwardedFor = "X-Forwarded-For"

// FromGoogleCloud extracts the best real client IP address from the request,
// handling xff behind a load balancer. It returns the empty string if no real
// IP was found.
func FromGoogleCloud(r *http.Request) string {
	if r == nil {
		return ""
	}

	// Get the remote addr
	ip := r.RemoteAddr

	// Check if x-forwarded-for exists, the load balancer sets this, and last
	// entry is the IP. We only check xff if there's at least two values to ensure
	// that it was, in fact, behind a load balancer.
	xff := r.Header.Get(headerKeyXForwardedFor)
	if xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 1 {
			ip = parts[len(parts)-1]
		}
	}

	return strings.TrimSpace(ip)
}
