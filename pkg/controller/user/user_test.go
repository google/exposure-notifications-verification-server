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

package user_test

import (
	"fmt"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
)

var testDatabaseInstance *database.TestInstance

func TestMain(m *testing.M) {
	testDatabaseInstance = database.MustTestInstance()
	defer testDatabaseInstance.MustClose()

	m.Run()
}

func provisionUsers(tb testing.TB, db *database.Database) (admin *database.User, user *database.User, realm *database.Realm) {
	tb.Helper()

	var err error

	realm, err = db.FindRealm(1)
	if err != nil {
		tb.Fatal(err)
	}

	admin, err = db.FindUser(1)
	if err != nil {
		tb.Fatal(err)
	}
	if err := admin.AddToRealm(db, realm, rbac.LegacyRealmAdmin, database.SystemTest); err != nil {
		tb.Fatal(err)
	}

	suffix, err := project.RandomHexString(6)
	if err != nil {
		tb.Fatal(err)
	}

	testUser := &database.User{
		Email: fmt.Sprintf("user-%s@example.com", suffix),
		Name:  "User",
	}
	if err := db.SaveUser(testUser, database.SystemTest); err != nil {
		tb.Fatal(err)
	}
	if err := testUser.AddToRealm(db, realm, 0, database.SystemTest); err != nil {
		tb.Fatal(err)
	}

	return admin, testUser, realm
}
