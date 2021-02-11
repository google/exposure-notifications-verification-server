// Copyright 2021 Google LLC
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

	"github.com/google/exposure-notifications-verification-server/internal/project"
)

func TestDatabase_FindSMSSigningKey(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	// not found
	{
		if _, err := db.FindSMSSigningKey(123456); !IsNotFound(err) {
			t.Errorf("expected %v to be NotFound", err)
		}
	}

	// found
	{
		realm, err := db.FindRealm(1)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := realm.CreateSMSSigningKeyVersion(ctx, db, SystemTest); err != nil {
			t.Fatal(err)
		}

		result, err := db.FindSMSSigningKey(1)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := result.RealmID, realm.ID; got != want {
			t.Errorf("expected %d to be %d", got, want)
		}
	}
}
