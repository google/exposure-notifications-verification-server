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

package rbac

import "testing"

func TestRBAC_Permissions(t *testing.T) {
	t.Parallel()

	// This test might seem like it's redundant with the content in rbac.go, but
	// it's designed to ensure that the exact values for existing RBAC permissions
	// remain unchanged.
	cases := []struct {
		p   Permission
		exp int64
	}{
		{AuditRead, 2},
		{APIKeyRead, 4},
		{APIKeyWrite, 8},
		{CodeIssue, 16},
		{CodeBulkIssue, 32},
		{CodeRead, 64},
		{CodeExpire, 128},
		{SettingsRead, 256},
		{SettingsWrite, 512},
		{StatsRead, 1024},
		{MobileAppRead, 2048},
		{MobileAppWrite, 4096},
		{UserRead, 8192},
		{UserWrite, 16384},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.p.String(), func(t *testing.T) {
			t.Parallel()

			if got, want := int64(tc.p), tc.exp; got != want {
				t.Errorf("expected %d to be %d", got, want)
			}
		})
	}
}
