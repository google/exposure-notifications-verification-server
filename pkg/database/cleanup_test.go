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
	"errors"
	"testing"
	"time"
)

func TestDatabase_CreateCleanup(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	cleanup1, err := db.CreateCleanup("x")
	if err != nil {
		t.Fatal(err)
	}

	// If the cleanup already exists, it's a noop
	cleanup2, err := db.CreateCleanup("x")
	if err != nil {
		t.Fatal(err)
	}

	if got, want := cleanup1.ID, cleanup2.ID; got != want {
		t.Errorf("expected %d to be %d", got, want)
	}
}

func TestDatabase_FindCleanupStatus(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	want, err := db.CreateCleanup("x")
	if err != nil {
		t.Fatal(err)
	}

	got, err := db.FindCleanupStatus("x")
	if err != nil {
		t.Fatal(err)
	}

	if got, want := got.ID, want.ID; got != want {
		t.Errorf("expected %d to be %d", got, want)
	}
}

func TestDatabase_ClaimCleanup(t *testing.T) {
	t.Parallel()

	t.Run("no_exist", func(t *testing.T) {
		t.Parallel()

		db, _ := testDatabaseInstance.NewDatabase(t, nil)
		cleanup, err := db.ClaimCleanup(&CleanupStatus{Type: "nope"}, 5*time.Second)
		if !IsNotFound(err) {
			t.Errorf("expected error, got: %v: %v", err, cleanup)
		}
	})

	t.Run("wrong_generation", func(t *testing.T) {
		t.Parallel()

		db, _ := testDatabaseInstance.NewDatabase(t, nil)

		if _, err := db.CreateCleanup("dirty"); err != nil {
			t.Fatal(err)
		}

		_, err := db.ClaimCleanup(&CleanupStatus{
			Type:       "dirty",
			Generation: 2,
		}, 1*time.Second)
		if got, want := err, ErrCleanupWrongGeneration; !errors.Is(err, want) {
			t.Errorf("expected %v to be %v", got, want)
		}
	})

	t.Run("exists", func(t *testing.T) {
		t.Parallel()

		db, _ := testDatabaseInstance.NewDatabase(t, nil)

		if _, err := db.CreateCleanup("dirty"); err != nil {
			t.Fatal(err)
		}

		cleanup, err := db.ClaimCleanup(&CleanupStatus{
			Type:       "dirty",
			Generation: 1,
		}, 1*time.Second)
		if err != nil {
			t.Fatal(err)
		}

		if got, want := cleanup.Generation, uint(2); got != want {
			t.Errorf("expected generation %d to be %d", got, want)
		}

		if got, now := cleanup.NotBefore, time.Now().UTC(); !got.After(now) {
			t.Errorf("expected %q to be after %q", got, now)
		}
	})
}
