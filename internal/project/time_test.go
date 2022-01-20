// Copyright 2022 the Exposure Notifications Verification Server authors
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
	"time"
)

func TestTruncateUTCHour(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   time.Time
		exp  time.Time
	}{
		{
			name: "zero",
			in:   time.Time{},
			exp:  time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "no_changes",
			in:   time.Date(2002, 1, 15, 5, 0, 0, 0, time.UTC),
			exp:  time.Date(2002, 1, 15, 5, 0, 0, 0, time.UTC),
		},
		{
			name: "truncates_minutes",
			in:   time.Date(2002, 1, 15, 5, 42, 0, 0, time.UTC),
			exp:  time.Date(2002, 1, 15, 5, 0, 0, 0, time.UTC),
		},
		{
			name: "truncates_seconds",
			in:   time.Date(2002, 1, 15, 5, 0, 34, 0, time.UTC),
			exp:  time.Date(2002, 1, 15, 5, 0, 0, 0, time.UTC),
		},
		{
			name: "truncates_nanos",
			in:   time.Date(2002, 1, 15, 5, 0, 0, 248438902, time.UTC),
			exp:  time.Date(2002, 1, 15, 5, 0, 0, 0, time.UTC),
		},
		{
			name: "converts_to_utc",
			in:   time.Date(2002, 1, 15, 5, 0, 0, 0, time.FixedZone("test", 1)),
			exp:  time.Date(2002, 1, 15, 4, 0, 0, 0, time.UTC),
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got, want := TruncateUTCHour(tc.in), tc.exp; got != want {
				t.Errorf("expected %v to be %v", got, want)
			}
		})
	}
}
