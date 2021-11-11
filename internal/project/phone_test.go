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

package project

import "testing"

func TestPhoneParseError(t *testing.T) {
	t.Parallel()

	_, err := CanonicalPhoneNumber("5135551234", "")
	if err == nil {
		t.Fatal("expect error, got nil")
	}
}

func TestPhoneParse(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		input  []string
		want   string
		region string
	}{
		{
			name: "us_match",
			input: []string{
				"2068675309",
				"+1 2068675309",
				"(206) 8675309",
				"(206) 867-5309",
				"+1 (206) 867-5309",
			},
			want:   "+12068675309",
			region: "us",
		},
		{
			name: "gb_match",
			input: []string{
				"2071838750",
				"2071 838750",
				"+44 2071 838750",
			},
			want:   "+442071838750",
			region: "gb",
		},
		{
			name: "br_match",
			input: []string{
				"1155256325",
			},
			want:   "+551155256325",
			region: "br",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			for _, in := range tc.input {
				got, err := CanonicalPhoneNumber(in, tc.region)
				if err != nil {
					t.Errorf("error on input %q err: %v", in, err)
					continue
				}
				if got != tc.want {
					t.Errorf("wrong canonical version for %q got: %q want: %q", in, got, tc.want)
				}
			}
		})
	}
}
