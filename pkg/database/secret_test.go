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
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestSecret_BeforeSave(t *testing.T) {
	t.Parallel()

	t.Run("reference", func(t *testing.T) {
		t.Parallel()
		exerciseValidation(t, &Secret{}, "Reference", "reference")
	})
}

func TestDatabase_FindSecret(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	t.Run("not_found", func(t *testing.T) {
		t.Parallel()
		if _, err := db.FindSecret(60221023); !IsNotFound(err) {
			t.Fatalf("expected %v to be NotFound", err)
		}
	})

	t.Run("finds", func(t *testing.T) {
		t.Parallel()

		secret := &Secret{
			Type:      SecretTypeCookieKeys,
			Reference: "a/b/c",
		}
		if err := db.SaveSecret(secret, SystemTest); err != nil {
			t.Fatal(err)
		}

		record, err := db.FindSecret(secret.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := record.ID, secret.ID; got != want {
			t.Errorf("expected %d to be %d", got, want)
		}
	})
}

func TestDatabase_ListSecrets(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		db, _ := testDatabaseInstance.NewDatabase(t, nil)

		// Clear secrets created by bootstrap
		if err := db.db.Unscoped().Delete(&Secret{}).Error; err != nil {
			t.Fatal(err)
		}

		list, err := db.ListSecrets()
		if err != nil {
			t.Fatal(err)
		}
		if got, want := len(list), 0; got != want {
			t.Errorf("expected list to have %d elements: %v", want, list)
		}
	})

	t.Run("lists", func(t *testing.T) {
		t.Parallel()

		db, _ := testDatabaseInstance.NewDatabase(t, nil)

		// Clear secrets created by bootstrap
		if err := db.db.Unscoped().Delete(&Secret{}).Error; err != nil {
			t.Fatal(err)
		}

		for i := 0; i < 3; i++ {
			secret := &Secret{
				Type:      SecretTypeCookieKeys,
				Reference: fmt.Sprintf("a/b/c/%d", i),
			}
			if err := db.SaveSecret(secret, SystemTest); err != nil {
				t.Fatal(err)
			}
		}

		list, err := db.ListSecrets()
		if err != nil {
			t.Fatal(err)
		}
		if got, want := len(list), 3; got != want {
			t.Errorf("expected list to have %d elements: %v", want, list)
		}
	})
}

func TestDatabase_ListSecretsForType(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		db, _ := testDatabaseInstance.NewDatabase(t, nil)

		// Clear secrets created by bootstrap
		if err := db.db.Unscoped().Delete(&Secret{}).Error; err != nil {
			t.Fatal(err)
		}

		list, err := db.ListSecretsForType(SecretTypeCookieKeys)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := len(list), 0; got != want {
			t.Errorf("expected list to have %d elements: %v", want, list)
		}
	})

	t.Run("none_for_type", func(t *testing.T) {
		t.Parallel()

		db, _ := testDatabaseInstance.NewDatabase(t, nil)

		// Clear secrets created by bootstrap
		if err := db.db.Unscoped().Delete(&Secret{}).Error; err != nil {
			t.Fatal(err)
		}

		secret := &Secret{
			Type:      SecretTypeAPIKeyDatabaseHMAC,
			Reference: "a/b/c",
		}
		if err := db.SaveSecret(secret, SystemTest); err != nil {
			t.Fatal(err)
		}

		list, err := db.ListSecretsForType(SecretTypeCookieKeys)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := len(list), 0; got != want {
			t.Errorf("expected list to have %d elements: %v", want, list)
		}
	})

	t.Run("lists", func(t *testing.T) {
		t.Parallel()

		db, _ := testDatabaseInstance.NewDatabase(t, nil)

		// Clear secrets created by bootstrap
		if err := db.db.Unscoped().Delete(&Secret{}).Error; err != nil {
			t.Fatal(err)
		}

		now := time.Now().UTC()

		if err := db.SaveSecret(&Secret{
			Type:      SecretTypeCookieKeys,
			Reference: "active_1",
			Active:    true,
			CreatedAt: now.Add(-30 * time.Minute),
		}, SystemTest); err != nil {
			t.Fatal(err)
		}

		if err := db.SaveSecret(&Secret{
			Type:      SecretTypeCookieKeys,
			Reference: "active_2",
			Active:    true,
			CreatedAt: now.Add(-25 * time.Minute),
		}, SystemTest); err != nil {
			t.Fatal(err)
		}

		if err := db.SaveSecret(&Secret{
			Type:      SecretTypeCookieKeys,
			Reference: "inactive",
			Active:    false,
			CreatedAt: now,
		}, SystemTest); err != nil {
			t.Fatal(err)
		}

		list, err := db.ListSecretsForType(SecretTypeCookieKeys)
		if err != nil {
			t.Fatal(err)
		}
		got := make([]string, len(list))
		for i, v := range list {
			got[i] = v.Reference
		}

		want := []string{"active_2", "active_1", "inactive"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("expected %q to be %q", got, want)
		}
	})
}
