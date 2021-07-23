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

package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

type CaptureOSHandler struct {
	OS database.OSType
}

func (c *CaptureOSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.OS = controller.OperatingSystemFromContext(r.Context())
}

func TestAddOperatingSystemFromUserAgent(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	addOS := middleware.AddOperatingSystemFromUserAgent()

	cases := []struct {
		name      string
		userAgent string
		want      database.OSType
	}{
		{
			name:      "android",
			userAgent: "Dalvik/2.1.0 (Linux; U; Android S Build/SP1A.210322.002)",
			want:      database.OSTypeAndroid,
		},
		{
			name:      "iOS_enx",
			userAgent: "bluetoothd (unknown version) CFNetwork/1237 Darwin/20.5.0",
			want:      database.OSTypeIOS,
		},
		{
			name:      "iphone",
			userAgent: "generic something that contains iPhone in it",
			want:      database.OSTypeIOS,
		},
		{
			name:      "unknown",
			userAgent: "being clever and using a customer user agent causes issues",
			want:      database.OSTypeUnknown,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r = r.Clone(ctx)
			r.Header.Set("User-Agent", tc.userAgent)

			w := httptest.NewRecorder()

			handler := &CaptureOSHandler{}
			addOS(handler).ServeHTTP(w, r)

			if got := handler.OS; got != tc.want {
				t.Errorf("Expected OS: %v to be %v", got, tc.want)
			}
		})
	}
}
