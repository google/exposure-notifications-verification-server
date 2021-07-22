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

package api

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestGetAcceptedTestTypes(t *testing.T) {
	t.Parallel()

	allTypes := AcceptTypes{
		"confirmed":   struct{}{},
		"likely":      struct{}{},
		"negative":    struct{}{},
		"user-report": struct{}{},
	}
	nonUserReport := AcceptTypes{
		"confirmed": struct{}{},
		"likely":    struct{}{},
		"negative":  struct{}{},
	}
	upToConfirmed := AcceptTypes{
		"confirmed": struct{}{},
	}
	upToLikely := AcceptTypes{
		"confirmed": struct{}{},
		"likely":    struct{}{},
	}

	cases := []struct {
		name  string
		input []string
		want  AcceptTypes
		err   string
	}{
		{
			name:  "default_empty",
			input: []string{},
			want:  allTypes,
		},
		{
			name:  "invalid",
			input: []string{"SELF-REPORTED"},
			want:  nil,
			err:   "invalid accepted test type: self-reported",
		},
		{
			name:  "full_suite",
			input: []string{"CONFIRMED", "Likely", "negative", "user-report"},
			want:  allTypes,
		},
		{
			name:  "just_confirmed",
			input: []string{"confirmed"},
			want:  upToConfirmed,
		},
		{
			name:  "confirmed_likely",
			input: []string{"confirmed", "likely"},
			want:  upToLikely,
		},
		{
			name:  "just_likely",
			input: []string{"likely"},
			want:  upToLikely,
		},
		{
			name:  "just_negative",
			input: []string{"negative"},
			want:  nonUserReport,
		},
		{
			name:  "negative_and_user",
			input: []string{"negative", "user-report"},
			want:  allTypes,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			v := VerifyCodeRequest{
				AcceptTestTypes: tc.input,
			}
			got, err := v.GetAcceptedTestTypes()
			if err != nil && tc.err == "" {
				t.Fatalf("unexpected error: %v", err)
			} else if err == nil && tc.err != "" {
				t.Fatalf("expected error: %q, got: nil", tc.err)
			} else if err != nil && !strings.Contains(err.Error(), tc.err) {
				t.Fatalf("expected error %q, got: %v", tc.err, err)
			}

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Fatalf("mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}
