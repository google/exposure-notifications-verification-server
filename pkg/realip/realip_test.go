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

package realip

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOnGoogleCloud(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		remoteAddr string
		xff        string
		exp        string
	}{
		{
			name: "none",
			exp:  "",
		},
		{
			name:       "remote_addr",
			remoteAddr: "1.1.1.1",
			exp:        "1.1.1.1",
		},
		{
			name:       "remote_addr_trim",
			remoteAddr: "  1.1.1.1  ",
			exp:        "1.1.1.1",
		},
		{
			name: "xff_single",
			xff:  "2.2.2.2",
			exp:  "",
		},
		{
			name: "xff_multi",
			xff:  "34.1.2.3,231.5.4.3,2.2.2.2",
			exp:  "231.5.4.3",
		},
		{
			name: "xff_multi_trim",
			xff:  "     34.1.2.3,  231.5.4.3,2.2.2.2",
			exp:  "231.5.4.3",
		},
		{
			name:       "remote_addr_with_xff",
			remoteAddr: "1.1.1.1",
			xff:        "34.1.2.3,231.5.4.3,2.2.2.2",
			exp:        "231.5.4.3",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = tc.remoteAddr
			if tc.xff != "" {
				r.Header.Set("X-Forwarded-For", tc.xff)
			}

			if got, want := FromGoogleCloud(r), tc.exp; got != want {
				t.Errorf("expected %q to be %q", got, want)
			}
		})
	}
}
