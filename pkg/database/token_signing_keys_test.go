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
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-verification-server/internal/project"
)

func TestDatabase_FindTokenSigningKey(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	t.Run("not_exist", func(t *testing.T) {
		t.Parallel()

		if _, err := db.FindTokenSigningKey(60221023); !IsNotFound(err) {
			t.Errorf("expected err to be NotFound, got %v", err)
		}
	})

	t.Run("finds", func(t *testing.T) {
		t.Parallel()

		key := &TokenSigningKey{
			KeyVersionID: "foo/bar/baz",
		}
		if err := db.SaveTokenSigningKey(key, SystemTest); err != nil {
			t.Fatal(err)
		}

		result, err := db.FindTokenSigningKey(key.ID)
		if err != nil {
			t.Fatal(err)
		}

		if got, want := result.ID, result.ID; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})
}

func TestDatabase_FindTokenSigningKeyByUUID(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	t.Run("invalid", func(t *testing.T) {
		t.Parallel()

		if _, err := db.FindTokenSigningKeyByUUID("invalid"); !IsNotFound(err) {
			t.Errorf("expected err to be NotFound, got %v", err)
		}
	})

	t.Run("not_exist", func(t *testing.T) {
		t.Parallel()

		if _, err := db.FindTokenSigningKeyByUUID("00000000-0000-0000-0000-000000000000"); !IsNotFound(err) {
			t.Errorf("expected err to be NotFound, got %v", err)
		}
	})

	t.Run("finds", func(t *testing.T) {
		t.Parallel()

		key := &TokenSigningKey{
			KeyVersionID: "foo/bar/baz",
		}
		if err := db.SaveTokenSigningKey(key, SystemTest); err != nil {
			t.Fatal(err)
		}

		result, err := db.FindTokenSigningKeyByUUID(key.UUID)
		if err != nil {
			t.Fatal(err)
		}

		if got, want := result.UUID, result.UUID; got != want {
			t.Errorf("expected %s to be %s", got, want)
		}
	})
}

func TestDatabase_ActiveTokenSigningKey(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	key := &TokenSigningKey{
		KeyVersionID: "foo/bar/baz",
	}

	// none
	{
		if _, err := db.ActiveTokenSigningKey(); !IsNotFound(err) {
			t.Errorf("expected err to be NotFound, got %v", err)
		}
	}

	// exists not active
	{
		if err := db.SaveTokenSigningKey(key, SystemTest); err != nil {
			t.Fatal(err)
		}

		if _, err := db.ActiveTokenSigningKey(); !IsNotFound(err) {
			t.Errorf("expected err to be NotFound, got %v", err)
		}
	}

	// active
	{
		if err := db.ActivateTokenSigningKey(key.ID, SystemTest); err != nil {
			t.Fatal(err)
		}

		result, err := db.ActiveTokenSigningKey()
		if err != nil {
			t.Fatal(err)
		}

		if got, want := result.ID, result.ID; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	}
}

func TestDatabase_ListTokenSigningKeys(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	// none
	{
		result, err := db.ListTokenSigningKeys()
		if err != nil {
			t.Fatal(err)
		}

		if result == nil {
			t.Fatal("result should not be nil")
		}
	}

	// lists
	{
		key := &TokenSigningKey{
			KeyVersionID: "foo/bar/baz",
		}
		if err := db.SaveTokenSigningKey(key, SystemTest); err != nil {
			t.Fatal(err)
		}

		list, err := db.ListTokenSigningKeys()
		if err != nil {
			t.Fatal(err)
		}
		if got, want := len(list), 1; got != want {
			t.Fatalf("expected %d to be %d", got, want)
		}

		if got, want := list[0].ID, key.ID; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	}
}

func TestDatabase_RotateTokenSigningKey(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	db, _ := testDatabaseInstance.NewDatabase(t, nil)
	keyManager := keys.TestKeyManager(t)
	keyManagerSigner, ok := keyManager.(keys.SigningKeyManager)
	if !ok {
		t.Fatal("kms cannot manage signing keys")
	}
	tokenSigningKey := keys.TestSigningKey(t, keyManager)

	key, err := db.RotateTokenSigningKey(ctx, keyManagerSigner, tokenSigningKey, SystemTest)
	if err != nil {
		t.Fatal(err)
	}

	if !key.IsActive {
		t.Error("key is not active")
	}
}

func TestDatabase_SaveTokenSigningKey(t *testing.T) {
	t.Parallel()

	t.Run("audits", func(t *testing.T) {
		t.Parallel()

		db, _ := testDatabaseInstance.NewDatabase(t, nil)

		key := &TokenSigningKey{
			KeyVersionID: "foo/bar/baz",
		}
		if err := db.SaveTokenSigningKey(key, SystemTest); err != nil {
			t.Fatal(err)
		}

		audits, _, err := db.ListAudits(nil)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := len(audits), 1; got != want {
			t.Errorf("expected %d audits, got %d: %#v", want, got, audits)
		}
	})
}

func TestDatabase_PurgeTokenSigningKeys(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	db, _ := testDatabaseInstance.NewDatabase(t, nil)
	keyManager := keys.TestKeyManager(t)
	keyManagerSigner, ok := keyManager.(keys.SigningKeyManager)
	if !ok {
		t.Fatal("kms cannot manage signing keys")
	}

	tokenSigningKey := keys.TestSigningKey(t, keyManager)

	for i := 0; i < 5; i++ {
		tokenSigningKeyVersion, err := keyManagerSigner.CreateKeyVersion(ctx, tokenSigningKey)
		if err != nil {
			t.Fatal(err)
		}

		key := &TokenSigningKey{
			KeyVersionID: tokenSigningKeyVersion,
		}
		if err := db.SaveTokenSigningKey(key, SystemTest); err != nil {
			t.Fatal(err)
		}
		if err := db.ActivateTokenSigningKey(key.ID, SystemTest); err != nil {
			t.Fatal(err)
		}
	}

	// Should not purge entries (too young).
	{
		n, err := db.PurgeTokenSigningKeys(ctx, keyManagerSigner, 24*time.Hour)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := n, int64(0); got != want {
			t.Errorf("expected %d to purge, got %d", want, got)
		}
	}

	// Purges entries.
	{
		n, err := db.PurgeTokenSigningKeys(ctx, keyManagerSigner, 1*time.Nanosecond)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := n, int64(4); got != want {
			t.Errorf("expected %d to purge, got %d", want, got)
		}
	}
}

func TestDatabase_ActivateTokenSigningKey(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	keyManager := keys.TestKeyManager(t)
	keyManagerSigner, ok := keyManager.(keys.SigningKeyManager)
	if !ok {
		t.Fatal("kms cannot manage signing keys")
	}

	tokenSigningKey := keys.TestSigningKey(t, keyManager)
	tokenSigningKeyVersion, err := keyManagerSigner.CreateKeyVersion(ctx, tokenSigningKey)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("activates", func(t *testing.T) {
		t.Parallel()

		db, _ := testDatabaseInstance.NewDatabase(t, nil)

		key := &TokenSigningKey{
			KeyVersionID: tokenSigningKeyVersion,
		}
		if err := db.SaveTokenSigningKey(key, SystemTest); err != nil {
			t.Fatal(err)
		}
		if err := db.ActivateTokenSigningKey(key.ID, SystemTest); err != nil {
			t.Fatal(err)
		}

		// Reload the key from the database.
		updatedKey, err := db.FindTokenSigningKey(key.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := updatedKey.IsActive, true; got != want {
			t.Errorf("expected is_active to be %t, got %t", want, got)
		}

		// Do it again to test "already active" condition.
		if err := db.ActivateTokenSigningKey(key.ID, SystemTest); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("audits", func(t *testing.T) {
		t.Parallel()

		db, _ := testDatabaseInstance.NewDatabase(t, nil)

		key := &TokenSigningKey{
			KeyVersionID: tokenSigningKeyVersion,
		}
		if err := db.SaveTokenSigningKey(key, SystemTest); err != nil {
			t.Fatal(err)
		}
		if err := db.ActivateTokenSigningKey(key.ID, SystemTest); err != nil {
			t.Fatal(err)
		}

		audits, _, err := db.ListAudits(nil)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := len(audits), 2; got != want {
			t.Errorf("expected %d audits, got %d: %#v", want, got, audits)
		}
	})
}
