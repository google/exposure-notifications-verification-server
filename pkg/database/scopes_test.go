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

package database

import (
	"reflect"
	"testing"

	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
)

func TestScopes_parseUserSearch(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		q    string
		exp  *userSearchQuery
		err  bool
	}{
		{
			name: "empty",
			q:    "",
			exp:  &userSearchQuery{},
		},
		{
			name: "name",
			q:    "name:seth",
			exp: &userSearchQuery{
				name: "seth",
			},
		},
		{
			name: "name_twice",
			q:    "name:seth name:vargo",
			err:  true,
		},
		{
			name: "email",
			q:    "email:foo@bar.com",
			exp: &userSearchQuery{
				email: "foo@bar.com",
			},
		},
		{
			name: "email_twice",
			q:    "email:foo@bar.com email:bar@foo.com",
			err:  true,
		},
		{
			name: "name_email",
			q:    "name:seth email:foo@bar.com",
			exp: &userSearchQuery{
				name:  "seth",
				email: "foo@bar.com",
			},
		},
		{
			name: "naked",
			q:    "foo bar",
			exp: &userSearchQuery{
				other: []string{"foo", "bar"},
			},
		},
		{
			name: "can",
			q:    "can:APIKeyRead",
			exp: &userSearchQuery{
				withPerms: rbac.APIKeyRead,
			},
		},
		{
			name: "can_multi",
			q:    "can:APIKeyRead can:APIKeyWrite",
			exp: &userSearchQuery{
				withPerms: rbac.APIKeyRead | rbac.APIKeyWrite,
			},
		},
		{
			name: "cannot",
			q:    "cannot:APIKeyRead",
			exp: &userSearchQuery{
				withoutPerms: rbac.APIKeyRead,
			},
		},
		{
			name: "cannot_multi",
			q:    "cannot:APIKeyRead cannot:APIKeyWrite",
			exp: &userSearchQuery{
				withoutPerms: rbac.APIKeyRead | rbac.APIKeyWrite,
			},
		},
		{
			name: "can_cannot",
			q:    "can:APIKeyRead cannot:APIKeyWrite",
			exp: &userSearchQuery{
				withPerms:    rbac.APIKeyRead,
				withoutPerms: rbac.APIKeyWrite,
			},
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			resp, err := parseUserSearch(tc.q)
			if (err != nil) != tc.err {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(resp, tc.exp) {
				t.Errorf("expected %#v to be %#v", resp, tc.exp)
			}
		})
	}
}
