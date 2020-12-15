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

package issueapi

import (
	"fmt"
	"strings"
	"testing"
)

func TestScrubPhoneNumber(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		not   string
	}{
		{input: "+11235550098"},
		{input: "+44 123 555 123"},
		{input: "+12065551234"},
		{input: "whatever"},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("case_%2d", i), func(t *testing.T) {
			t.Parallel()

			errMsg := fmt.Sprintf("The 'To' phone number: %s, is not currently reachable using the 'From' phone number: 12345 via SMS.", tc.input)
			got := scrubPhoneNumbers(errMsg)
			if strings.Contains(got, tc.input) {
				t.Errorf("phone number was not scrubbed")
			}
		})
	}
}
