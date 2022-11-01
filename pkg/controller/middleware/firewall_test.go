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

package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

func TestProcessFirewall(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	h, err := render.New(ctx, nil, true)
	if err != nil {
		t.Fatal(err)
	}

	processFirewall := middleware.ProcessFirewall(h, "server")(emptyHandler())

	cases := []struct {
		name       string
		ctx        context.Context
		remoteAddr string
		xff        string
		code       int
	}{
		{
			name: "no_realm",
			ctx:  ctx,
			code: http.StatusBadRequest,
		},
		{
			name: "realm_in_context",
			ctx:  controller.WithRealm(ctx, &database.Realm{}),
			code: http.StatusOK,
		},
		{
			name: "membership_in_context",
			ctx: controller.WithMembership(ctx, &database.Membership{
				Realm: &database.Realm{},
			}),
			code: http.StatusOK,
		},
		{
			name: "all_allowed4",
			ctx: controller.WithRealm(ctx, &database.Realm{
				AllowedCIDRsServer: []string{"0.0.0.0/0"},
			}),
			remoteAddr: "1.2.3.4",
			code:       http.StatusOK,
		},
		{
			name: "all_allowed6",
			ctx: controller.WithRealm(ctx, &database.Realm{
				AllowedCIDRsServer: []string{"::/0"},
			}),
			remoteAddr: "2001:db8::8a2e:370:7334",
			code:       http.StatusOK,
		},
		{
			name: "single_allowed_ip4",
			ctx: controller.WithRealm(ctx, &database.Realm{
				AllowedCIDRsServer: []string{"1.2.3.4/32"},
			}),
			remoteAddr: "1.2.3.4",
			code:       http.StatusOK,
		},
		{
			name: "single_allowed_ip6",
			ctx: controller.WithRealm(ctx, &database.Realm{
				AllowedCIDRsServer: []string{"2001::/0"},
			}),
			remoteAddr: "2001:db8::8a2e:370:7334",
			code:       http.StatusOK,
		},
		{
			name: "single_allowed_xff",
			ctx: controller.WithRealm(ctx, &database.Realm{
				AllowedCIDRsServer: []string{"5.6.7.8/32"},
			}),
			remoteAddr: "9.8.7.6",
			xff:        "5.6.7.8, 1.2.3.4",
			code:       http.StatusOK,
		},
		{
			name: "single_reject_ip4",
			ctx: controller.WithRealm(ctx, &database.Realm{
				AllowedCIDRsServer: []string{"1.2.3.4/32"},
			}),
			remoteAddr: "9.8.7.6",
			code:       http.StatusUnauthorized,
		},
		{
			name: "single_reject_ip6",
			ctx: controller.WithRealm(ctx, &database.Realm{
				AllowedCIDRsServer: []string{"2000::/64"},
			}),
			remoteAddr: "2001:db8::8a2e:370:7334",
			code:       http.StatusUnauthorized,
		},
		{
			name: "single_reject_xff",
			ctx: controller.WithRealm(ctx, &database.Realm{
				AllowedCIDRsServer: []string{"1.2.3.4/32"},
			}),
			remoteAddr: "1.2.3.4",          // xff is preferred over remote ip
			xff:        "9.8.7.6, 5.6.7.8", // Only trusts the last value in xff
			code:       http.StatusUnauthorized,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r = r.Clone(tc.ctx)
			r.Header.Set("Accept", "application/json")

			if v := tc.remoteAddr; v != "" {
				r.RemoteAddr = v
			}
			if v := tc.xff; v != "" {
				r.Header.Set("X-Forwarded-For", v)
			}

			w := httptest.NewRecorder()

			processFirewall.ServeHTTP(w, r)
			w.Flush()

			if got, want := w.Code, tc.code; got != want {
				t.Errorf("Expected %d to be %d", got, want)
			}
		})
	}
}
