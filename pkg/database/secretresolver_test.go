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
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/pkg/secrets"
	"github.com/google/exposure-notifications-verification-server/internal/project"
)

func TestResolve(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	// Clear secrets created by bootstrap
	if err := db.db.Unscoped().Delete(&Secret{}).Error; err != nil {
		t.Fatal(err)
	}

	sm, err := secrets.NewInMemoryFromMap(ctx, map[string]string{
		"a/b/1": "one",
		"a/b/2": "two",
		"a/b/3": "three",
	})
	if err != nil {
		t.Fatal(err)
	}

	resolver := NewSecretResolver()

	for i := 1; i <= 3; i++ {
		secret := &Secret{
			Type:      SecretTypeAPIKeyDatabaseHMAC,
			Reference: fmt.Sprintf("a/b/%d", i),
			Active:    true,
			CreatedAt: time.Now().UTC().Add(time.Duration(-i) * time.Hour),
		}
		if err := db.SaveSecret(secret, SystemTest); err != nil {
			t.Fatal(err)
		}
	}

	value, err := resolver.Resolve(ctx, db, sm, SecretTypeAPIKeyDatabaseHMAC)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := value, [][]byte{[]byte("one"), []byte("two"), []byte("three")}; !reflect.DeepEqual(got, want) {
		t.Errorf("expected %#v to be %#v", got, want)
	}

	// This tests that the value is actually cached.
	if _, err := resolver.Resolve(ctx, nil, nil, SecretTypeAPIKeyDatabaseHMAC); err != nil {
		t.Fatal(err)
	}
}

func TestSecretResolver_ResolveReferences(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	// Clear secrets created by bootstrap
	if err := db.db.Unscoped().Delete(&Secret{}).Error; err != nil {
		t.Fatal(err)
	}

	resolver := NewSecretResolver()

	secret := &Secret{
		Type:      SecretTypeAPIKeyDatabaseHMAC,
		Reference: "a/b/c",
		Active:    true,
	}
	if err := db.SaveSecret(secret, SystemTest); err != nil {
		t.Fatal(err)
	}

	references, err := resolver.ResolveReferences(db, SecretTypeAPIKeyDatabaseHMAC)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := references, []string{"a/b/c"}; !reflect.DeepEqual(got, want) {
		t.Errorf("expected %#v to be %#v", got, want)
	}

	// This tests that the value is actually cached.
	if _, err := resolver.ResolveReferences(nil, SecretTypeAPIKeyDatabaseHMAC); err != nil {
		t.Fatal(err)
	}
}

func TestSecretResolver_ResolveValues(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	// Clear secrets created by bootstrap
	if err := db.db.Unscoped().Delete(&Secret{}).Error; err != nil {
		t.Fatal(err)
	}

	sm, err := secrets.NewInMemoryFromMap(ctx, map[string]string{
		"a/b/c": "hello",
	})
	if err != nil {
		t.Fatal(err)
	}

	resolver := NewSecretResolver()

	secret := &Secret{
		Type:      SecretTypeAPIKeyDatabaseHMAC,
		Reference: "a/b/c",
		Active:    true,
	}
	if err := db.SaveSecret(secret, SystemTest); err != nil {
		t.Fatal(err)
	}

	value, err := resolver.ResolveValue(ctx, sm, "a/b/c")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := value, []byte("hello"); !reflect.DeepEqual(got, want) {
		t.Errorf("expected %#v to be %#v", got, want)
	}

	// This tests that the value is actually cached.
	if _, err := resolver.ResolveValue(ctx, nil, "a/b/c"); err != nil {
		t.Fatal(err)
	}
}
