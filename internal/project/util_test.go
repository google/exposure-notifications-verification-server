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

package project

import (
	"testing"
)

func TestRoot(t *testing.T) {
	t.Parallel()

	if got := Root(); got == "" {
		t.Errorf("expected root")
	}
}

func TestAllDigits(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		testString string
		expect     bool
	}{
		{
			name:       "digits",
			testString: "123987",
			expect:     true,
		},
		{
			name:       "some non-digits",
			testString: "123some",
			expect:     false,
		},
		{
			name:       "empty",
			testString: "",
			expect:     false,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := AllDigits(tc.testString); got != tc.expect {
				t.Errorf("wrong result for %q. got %t want %t", tc.testString, got, tc.expect)
			}
		})
	}
}
