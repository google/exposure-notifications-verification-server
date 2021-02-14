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

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestRequiredPermissions(t *testing.T) {
	t.Parallel()

	for perm, needs := range requiredPermission {
		perm := perm
		needs := needs
		name := fmt.Sprintf("implied_by_%s", PermissionMap[perm][0])

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			actorPermission := perm
			for _, n := range needs {
				actorPermission |= n
			}

			got, err := CompileAndAuthorize(actorPermission, []Permission{perm})
			if err != nil {
				t.Fatalf("missing all required permissions for %v", PermissionNames(perm))
			}

			for _, n := range needs {
				if !Can(got, n) {
					t.Errorf("%v did not imply %v as expected", PermissionNames(perm), PermissionNames(n))
				}
			}
		})
	}

	t.Run("missing", func(t *testing.T) {
		t.Parallel()

		if _, err := CompileAndAuthorize(0, []Permission{APIKeyRead}); err == nil {
			t.Errorf("expected error")
		}
	})
}

func TestImpliedBy(t *testing.T) {
	t.Parallel()

	if got := ImpliedBy(APIKeyRead); !reflect.DeepEqual(got, []Permission{APIKeyWrite}) {
		t.Errorf("expected %q to imply %q", APIKeyRead, APIKeyWrite)
	}

	if got := ImpliedBy(APIKeyWrite); len(got) != 0 {
		t.Errorf("expected no implications, got %q", got)
	}
}

func TestPermission_Implied(t *testing.T) {
	t.Parallel()

	if got := APIKeyWrite.Implied(); !reflect.DeepEqual(got, []Permission{APIKeyRead}) {
		t.Errorf("expected %q to imply %q", APIKeyRead, APIKeyWrite)
	}
}

func TestPermissionNames(t *testing.T) {
	t.Parallel()

	cases := []struct {
		p   Permission
		exp string
	}{
		{0, ""},
		{APIKeyWrite, "APIKeyWrite"},
		{LegacyRealmUser, "CodeBulkIssue,CodeExpire,CodeIssue,CodeRead"},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.p.String(), func(t *testing.T) {
			t.Parallel()

			if got, want := strings.Join(PermissionNames(tc.p), ","), tc.exp; !reflect.DeepEqual(got, want) {
				t.Errorf("Expected %q to be %q", got, want)
			}
		})
	}
}

func TestPermission_String(t *testing.T) {
	t.Parallel()

	cases := []struct {
		p   Permission
		exp string
	}{
		{0, "Permission(0)"},
		{APIKeyWrite, "APIKeyWrite"},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.p.String(), func(t *testing.T) {
			t.Parallel()

			if got, want := tc.p.String(), tc.exp; got != want {
				t.Errorf("Expected %q to be %q", got, want)
			}
		})
	}
}

func TestPermission_Value(t *testing.T) {
	t.Parallel()

	cases := []struct {
		p   Permission
		exp int64
	}{
		{0, 0},
		{APIKeyWrite, 8},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.p.String(), func(t *testing.T) {
			t.Parallel()

			result, err := tc.p.Value()
			if err != nil {
				t.Fatal(err)
			}

			got, ok := result.(int64)
			if !ok {
				t.Fatalf("%T is not int64", result)
			}

			if got, want := got, tc.exp; got != want {
				t.Errorf("expected %v to be %v", got, want)
			}
		})
	}
}

func TestPermission_Description(t *testing.T) {
	t.Parallel()

	cases := []struct {
		p   Permission
		exp string
		err bool
	}{
		{0, "", true},
		{APIKeyWrite, "create, update, and delete API keys", false},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.p.String(), func(t *testing.T) {
			t.Parallel()

			result, err := tc.p.Description()
			if (err != nil) != tc.err {
				t.Fatal(err)
			}

			if got, want := result, tc.exp; got != want {
				t.Errorf("Expected %q to be %q", got, want)
			}
		})
	}
}

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
				t.Errorf("Expected %d to be %d", got, want)
			}
		})
	}
}
