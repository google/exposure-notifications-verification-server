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
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/pkg/timeutils"
)

func TestSMS(t *testing.T) {
	realm := NewRealmWithDefaults("test")
	realm.SMSTextTemplate = "This is your Exposure Notifications Verification code: [enslink] Expires in [longexpires] hours"
	realm.RegionCode = "US-WA"

	got := realm.BuildSMSText("12345678", "abcdefgh12345678", "en.express")
	want := "This is your Exposure Notifications Verification code: https://us-wa.en.express/v?c=abcdefgh12345678 Expires in 24 hours"
	if got != want {
		t.Errorf("SMS text wrong, want: %q got %q", want, got)
	}

	realm.SMSTextTemplate = "State of Wonder, COVID-19 Exposure Verification code [code]. Expires in [expires] minutes. Act now!"
	got = realm.BuildSMSText("654321", "asdflkjasdlkfjl", "")
	want = "State of Wonder, COVID-19 Exposure Verification code 654321. Expires in 15 minutes. Act now!"
	if got != want {
		t.Errorf("SMS text wrong, want: %q got %q", want, got)
	}
}

func TestPerUserRealmStats(t *testing.T) {
	t.Parallel()

	db := NewTestDatabase(t)

	numDays := 7
	endDate := timeutils.Midnight(time.Now())
	startDate := timeutils.Midnight(endDate.Add(time.Duration(numDays) * -24 * time.Hour))

	// Create a new realm
	realm := NewRealmWithDefaults("test")
	if err := db.SaveRealm(realm, System); err != nil {
		t.Fatalf("error saving realm: %v", err)
	}

	// Create the users.
	users := []*User{}
	for userIdx, name := range []string{"Rocky", "Bullwinkle", "Boris", "Natasha"} {
		user := &User{
			Realms: []*Realm{realm},
			Name:   name,
			Email:  name + "@gmail.com",
			Admin:  false,
		}

		if err := db.SaveUser(user, System); err != nil {
			t.Fatalf("[%v] error creating user: %v", name, err)
		}
		users = append(users, user)

		// Add some stats per user.
		for i := 0; i < numDays; i++ {
			stat := &UserStats{
				RealmID:     realm.ID,
				UserID:      user.ID,
				Date:        startDate.Add(time.Duration(i) * 24 * time.Hour),
				CodesIssued: uint(10 + i + userIdx),
			}
			if err := db.SaveUserStats(stat); err != nil {
				t.Fatalf("error saving user stats %v", err)
			}
		}
	}

	if len(users) == 0 { // sanity check
		t.Error("len(users) = 0, expected ≠ 0")
	}

	stats, err := realm.CodesPerUser(db, startDate, endDate)
	if err != nil {
		t.Fatalf("error getting stats: %v", err)
	}

	if len(stats) != numDays*len(users) {
		t.Errorf("len(stats) = %d, expected %d", len(stats), numDays*len(users))
	}

	for i := 0; i < len(stats)-1; i++ {
		if stats[i].Date != stats[i+1].Date {
			if !stats[i].Date.After(stats[i+1].Date) {
				t.Errorf("[%d] dates should be in descending order: %v.After(%v)", i, stats[i].Date, stats[i+1].Date)
			}
		}
	}
}
