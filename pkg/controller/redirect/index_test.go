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

package redirect

import "testing"

func TestBuildURL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		path   string
		region string
		exp    string
	}{
		{
			name:   "leading_slash_region",
			path:   "v",
			region: "/US-AA",
			exp:    "ens://v?r=%2FUS-AA",
		},
		{
			name:   "trailing_slash_region",
			path:   "v",
			region: "US-AA/",
			exp:    "ens://v?r=US-AA%2F",
		},
		{
			name:   "leading_slash_path",
			path:   "/v",
			region: "US-AA",
			exp:    "ens://v?r=US-AA",
		},
		{
			name:   "trailing_slash_path",
			path:   "v/",
			region: "US-AA",
			exp:    "ens://v/?r=US-AA",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, want := buildURL(tc.path, tc.region), tc.exp
			if got != want {
				t.Errorf("expected %q to be %q", got, want)
			}
		})
	}
}
