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

	"github.com/google/exposure-notifications-verification-server/internal/project"
)

func TestTryLock(t *testing.T) {
	t.Parallel()

	lockName := "lockName"
	period := 1 * time.Second

	ctx := project.TestContext(t)
	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	if ok, err := db.TryLock(ctx, lockName, period); err != nil {
		t.Fatal(err)
	} else if !ok {
		t.Fatalf("failed to claim app sync lock when available")
	}

	if ok, err := db.TryLock(ctx, lockName, period); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Fatalf("allowed to claim lock when it should not be available")
	}

	time.Sleep(period)

	if ok, err := db.TryLock(ctx, lockName, period); err != nil {
		t.Fatal(err)
	} else if !ok {
		t.Fatalf("failed to claim app sync lock when available")
	}
}

func TestDatabase_CreateLock(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	cleanup1, err := db.CreateLock("x")
	if err != nil {
		t.Fatal(err)
	}

	// If the cleanup already exists, it's a noop
	cleanup2, err := db.CreateLock("x")
	if err != nil {
		t.Fatal(err)
	}

	if got, want := cleanup1.ID, cleanup2.ID; got != want {
		t.Errorf("Expected %d to be %d", got, want)
	}
}

func TestDatabase_FindLockStatus(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	want, err := db.CreateLock("x")
	if err != nil {
		t.Fatal(err)
	}

	got, err := db.FindLockStatus("x")
	if err != nil {
		t.Fatal(err)
	}

	if got, want := got.ID, want.ID; got != want {
		t.Errorf("Expected %d to be %d", got, want)
	}
}

func TestDatabase_ClaimLock(t *testing.T) {
	t.Parallel()

	t.Run("no_exist", func(t *testing.T) {
		t.Parallel()

		db, _ := testDatabaseInstance.NewDatabase(t, nil)
		cleanup, err := db.ClaimLock(&CleanupStatus{Type: "nope"}, 5*time.Second)
		if !IsNotFound(err) {
			t.Errorf("expected error, got: %v: %v", err, cleanup)
		}
	})

	t.Run("wrong_generation", func(t *testing.T) {
		t.Parallel()

		db, _ := testDatabaseInstance.NewDatabase(t, nil)

		if _, err := db.CreateLock("dirty"); err != nil {
			t.Fatal(err)
		}

		_, err := db.ClaimLock(&CleanupStatus{
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

		if _, err := db.CreateLock("dirty"); err != nil {
			t.Fatal(err)
		}

		cleanup, err := db.ClaimLock(&CleanupStatus{
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
