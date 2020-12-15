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

package database

import (
	"testing"
)

func TestMembership_AfterFind(t *testing.T) {
	t.Parallel()

	t.Run("user", func(t *testing.T) {
		t.Parallel()

		var m Membership
		_ = m.AfterFind()
		if errs := m.ErrorsFor("user"); len(errs) < 1 {
			t.Errorf("expected errors for %s", "user")
		}
	})

	t.Run("realm", func(t *testing.T) {
		t.Parallel()

		var m Membership
		_ = m.AfterFind()
		if errs := m.ErrorsFor("realm"); len(errs) < 1 {
			t.Errorf("expected errors for %s", "realm")
		}
	})
}
