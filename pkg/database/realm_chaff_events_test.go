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

	"github.com/google/exposure-notifications-server/pkg/timeutils"
)

func TestDatabase_HasRealmChaffEventsMap(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)
	realm, err := db.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}

	{
		m, err := db.HasRealmChaffEventsMap()
		if err != nil {
			t.Fatal(err)
		}
		if m == nil {
			t.Errorf("map should not be nil")
		}
		if len(m) != 0 {
			t.Errorf("map should be empty")
		}
	}

	if err := realm.RecordChaffEvent(db, time.Now().UTC().Add(-24*time.Hour)); err != nil {
		t.Fatal(err)
	}

	{
		m, err := db.HasRealmChaffEventsMap()
		if err != nil {
			t.Fatal(err)
		}
		if m == nil {
			t.Errorf("map should not be nil")
		}
		if _, ok := m[realm.ID]; !ok {
			t.Errorf("map is missing realm: %#v", m)
		}
	}
}

func TestDatabase_PurgeRealmChaffEvents(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)
	realm, err := db.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}

	for i := 1; i < 10; i++ {
		ts := timeutils.UTCMidnight(time.Now().UTC()).Add(-24 * time.Hour * time.Duration(i))
		if err := realm.RecordChaffEvent(db, ts); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := db.PurgeRealmChaffEvents(0); err != nil {
		t.Fatal(err)
	}

	events, err := realm.ListChaffEvents(db)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if event.Present {
			t.Errorf("should not be present: %#v", event)
		}
	}
}
