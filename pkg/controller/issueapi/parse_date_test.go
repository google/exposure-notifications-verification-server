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

package issueapi_test

import (
	"testing"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/issueapi"
)

func TestDateValidation(t *testing.T) {
	t.Parallel()

	utc, err := time.LoadLocation("UTC")
	if err != nil {
		t.Fatal(err)
	}
	var aug1 time.Time
	aug1, err = time.ParseInLocation(project.RFC3339Date, "2020-08-01", utc)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		v         string
		max       time.Time
		tzOffset  int
		shouldErr bool
		expected  string
	}{
		{"2020-08-01", aug1, 0, false, "2020-08-01"},
		{"2020-08-01", aug1, 60, false, "2020-08-01"},
		{"2020-08-01", aug1, 60 * 12, false, "2020-08-01"},
		{"2020-07-31", aug1, 60, false, "2020-07-31"},
		{"2020-08-01", aug1, -60, false, "2020-08-01"},
		{"2020-07-31", aug1, -60, false, "2020-07-31"},
		{"2020-07-30", aug1, -60, false, "2020-07-30"},
		{"2020-07-29", aug1, -60, true, "2020-07-30"},
	}
	for _, tc := range cases {
		tc := tc

		t.Run(tc.v, func(t *testing.T) {
			t.Parallel()

			date, err := time.ParseInLocation(project.RFC3339Date, tc.v, utc)
			if err != nil {
				t.Fatalf("error parsing date %q", tc.v)
			}
			min := tc.max.Add(-24 * time.Hour)
			var newDate *time.Time
			if newDate, err = issueapi.ValidateDate(date, min, tc.max, tc.tzOffset); newDate == nil {
				if err != nil {
					if !tc.shouldErr {
						t.Fatalf("validateDate returned an unexpected error: %q", err)
					}
				} else {
					t.Fatalf("expected error")
				}
			} else if s := newDate.Format(project.RFC3339Date); s != tc.expected {
				t.Fatalf("validateDate returned a different date %q != %q", s, tc.expected)
			}
		})
	}
}
