// Copyright 2020 the Exposure Notifications Verification Server authors
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

	"github.com/google/exposure-notifications-verification-server/pkg/pagination"
)

func TestAuditEntry_BeforeSave(t *testing.T) {
	t.Parallel()

	cases := []struct {
		structField string
		field       string
	}{
		{"ActorID", "actor_id"},
		{"ActorDisplay", "actor_display"},
		{"Action", "action"},
		{"TargetID", "target_id"},
		{"TargetDisplay", "target_display"},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.field, func(t *testing.T) {
			t.Parallel()
			exerciseValidation(t, &AuditEntry{}, tc.structField, tc.field)
		})
	}
}

func TestDatabase_PurgeAuditEntries(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)
	for i := 0; i < 5; i++ {
		if err := db.SaveAuditEntry(&AuditEntry{
			RealmID:       1,
			ActorID:       "actor:1",
			ActorDisplay:  "Actor",
			Action:        "created",
			TargetID:      "target:1",
			TargetDisplay: "Target",
		}); err != nil {
			t.Fatal(err)
		}
	}

	// Should not purge entries (too young).
	{
		n, err := db.PurgeAuditEntries(24 * time.Hour)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := n, int64(0); got != want {
			t.Errorf("expected %d to purge, got %d", want, got)
		}
	}

	// Purges entries.
	{
		n, err := db.PurgeAuditEntries(1 * time.Nanosecond)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := n, int64(5); got != want {
			t.Errorf("expected %d to purge, got %d", want, got)
		}
	}
}

func TestDatabase_ListAudits(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		db, _ := testDatabaseInstance.NewDatabase(t, nil)

		audits, _, err := db.ListAudits(&pagination.PageParams{Limit: 1})
		if err != nil {
			t.Fatal(err)
		}
		if got, want := len(audits), 0; got != want {
			t.Errorf("expected %d audits, got %d: %v", want, got, audits)
		}
	})

	t.Run("lists", func(t *testing.T) {
		t.Parallel()

		db, _ := testDatabaseInstance.NewDatabase(t, nil)
		for i := 0; i < 5; i++ {
			if err := db.SaveAuditEntry(&AuditEntry{
				RealmID:       1,
				ActorID:       "actor:1",
				ActorDisplay:  "Actor",
				Action:        "created",
				TargetID:      "target:1",
				TargetDisplay: "Target",
			}); err != nil {
				t.Fatal(err)
			}
		}

		audits, _, err := db.ListAudits(&pagination.PageParams{Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		if got, want := len(audits), 5; got != want {
			t.Errorf("expected %d audits, got %d: %v", want, got, audits)
		}
	})
}
