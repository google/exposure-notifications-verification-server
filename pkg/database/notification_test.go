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
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestNotificationValidation(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	realm, err := db.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name         string
		notification *Notification
		wantErr      string
		errors       map[string]string
	}{
		{
			name: "no_realm",
			notification: func() *Notification {
				n := NewNotification(realm, NotificationAbuseLimitReached, "worry")
				n.RealmID = 0
				return n
			}(),
			wantErr: "validation failed",
			errors:  map[string]string{"realm_id": "must be set"},
		},
		{
			name:         "no_message",
			notification: NewNotification(realm, NotificationAbuseLimitReached, ""),
			wantErr:      "validation failed",
			errors:       map[string]string{"message": "cannot be blank"},
		},
		{
			name: "bad_category",
			notification: func() *Notification {
				n := NewNotification(realm, NotificationAbuseLimitReached, "worry")
				n.Category = notificationCeiling
				return n
			}(),
			wantErr: "validation failed",
			errors:  map[string]string{"category": "invalid category"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := db.ScheduleNotification(tc.notification, SystemTest)
			if err == nil {
				t.Fatalf("missing expected error: %q", tc.wantErr)
			}

		})
	}
}

func TestNotificationScheduleAndSend(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	realm, err := db.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}

	limitReached := NewNotification(realm, NotificationAbuseLimitReached, "code issue limit reached")
	if err := db.ScheduleNotification(limitReached, SystemTest); err != nil {
		t.Fatalf("scheduleNotification: %v", err)
	}

	got, err := db.SelectNotifications(10)
	if err != nil {
		t.Fatalf("selectNotifications: %v", err)
	}
	if l := len(got); l != 1 {
		t.Fatalf("wrong number of undelivered notifications found, want: 1 got: %v", l)
	}

	opts := cmp.Options{
		cmpopts.EquateApproxTime(time.Second),
		cmpopts.IgnoreUnexported(Errorable{}),
	}
	if diff := cmp.Diff(limitReached, got[0], opts); diff != "" {
		t.Fatalf("mismatch (-want, +got):\n%s", diff)
	}

	{
		// Attempt to send to close together.
		limitReached2 := NewNotification(realm, NotificationAbuseLimitReached, "code issue limit reached")
		if err := db.ScheduleNotification(limitReached2, SystemTest); err == nil {
			t.Fatalf("expected error, got none")
		} else if !strings.Contains(err.Error(), "cannot be scheduled for this realm until") {
			t.Fatalf("wrong error: %v", err)
		}
	}

	{
		// List notifications for this realm.
		notifications, err := db.ListRealmNotifications(realm.ID)
		if err != nil {
			t.Fatalf("listRealmNotifications: %v", err)
		}
		want := []*Notification{limitReached}
		if diff := cmp.Diff(want, notifications, opts); diff != "" {
			t.Fatalf("mismatch (-want, +got):\n%s", diff)
		}
	}

	// Mark this as delivered.
	if err := limitReached.MarkDelivered(db, []string{"delivered to `test phone`"}); err != nil {
		t.Fatalf("markDelivered: %v", err)
	}
}

func TestNotificationMarkAndSweep(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	realm, err := db.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}

	limitReached := NewNotification(realm, NotificationAbuseLimitReached, "code issue limit reached")
	if err := db.ScheduleNotification(limitReached, SystemTest); err != nil {
		t.Fatalf("scheduleNotification: %v", err)
	}

	time.Sleep(1100 * time.Millisecond)
	if got, err := db.DeleteNotifications(time.Second); err != nil {
		t.Fatalf("deleteNotifications: %v", err)
	} else if got != 1 {
		t.Fatalf("unexpected number of notifications marked: want: 1, got: %v", got)
	}

	time.Sleep(1100 * time.Millisecond)
	if got, err := db.PurgeNotifications(time.Second); err != nil {
		t.Fatalf("purgeNotifications: %v", err)
	} else if got != 1 {
		t.Fatalf("unexpected number of notifications purged: want: 1, got: %v", got)
	}
}
