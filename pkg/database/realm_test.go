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
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/pkg/timeutils"
	"github.com/google/exposure-notifications-verification-server/internal/project"
)

func TestSMS(t *testing.T) {
	t.Parallel()

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

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	numDays := 7
	endDate := timeutils.Midnight(time.Now())
	startDate := timeutils.Midnight(endDate.Add(time.Duration(numDays) * -24 * time.Hour))

	// Create a new realm
	realm := NewRealmWithDefaults("test")
	if err := db.SaveRealm(realm, SystemTest); err != nil {
		t.Fatalf("error saving realm: %v", err)
	}

	// Create the users.
	users := []*User{}
	for userIdx, name := range []string{"Rocky", "Bullwinkle", "Boris", "Natasha"} {
		user := &User{
			Realms:      []*Realm{realm},
			Name:        name,
			Email:       name + "@gmail.com",
			SystemAdmin: false,
		}

		if err := db.SaveUser(user, SystemTest); err != nil {
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

	stats, err := realm.UserStats(db, startDate, endDate)
	if err != nil {
		t.Fatalf("error getting stats: %v", err)
	}

	for i := 0; i < len(stats)-1; i++ {
		if stats[i].Date != stats[i+1].Date {
			if !stats[i].Date.After(stats[i+1].Date) {
				t.Errorf("[%d] dates should be in descending order: %v.After(%v)", i, stats[i].Date, stats[i+1].Date)
			}
		}
	}
}

func TestRealm_FindMobileApp(t *testing.T) {
	t.Parallel()

	t.Run("access_across_realms", func(t *testing.T) {
		t.Parallel()

		db, _ := testDatabaseInstance.NewDatabase(t, nil)

		realm1 := NewRealmWithDefaults("realm1")
		if err := db.SaveRealm(realm1, SystemTest); err != nil {
			t.Fatal(err)
		}

		realm2 := NewRealmWithDefaults("realm2")
		if err := db.SaveRealm(realm2, SystemTest); err != nil {
			t.Fatal(err)
		}

		app1 := &MobileApp{
			Name:    "app1",
			RealmID: realm1.ID,
			URL:     "https://example1.com",
			OS:      OSTypeIOS,
			AppID:   "app1",
		}
		if err := db.SaveMobileApp(app1, SystemTest); err != nil {
			t.Fatal(err)
		}

		app2 := &MobileApp{
			Name:    "app2",
			RealmID: realm1.ID,
			URL:     "https://example2.com",
			OS:      OSTypeIOS,
			AppID:   "app2",
		}
		if err := db.SaveMobileApp(app2, SystemTest); err != nil {
			t.Fatal(err)
		}

		// realm1 should be able to lookup app1
		{
			found, err := realm1.FindMobileApp(db, app1.ID)
			if err != nil {
				t.Fatal(err)
			}

			if got, want := found.ID, app1.ID; got != want {
				t.Errorf("expected %v to be %v", got, want)
			}
		}

		// realm2 should NOT be able to lookup app1
		{
			if _, err := realm2.FindMobileApp(db, app1.ID); err == nil {
				t.Fatal("expected error")
			}
		}
	})
}

func TestRealm_CreateSigningKeyVersion(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	db.config.CertificateSigningKeyRing = filepath.Join(project.Root(), "local", "test", "realm")
	db.config.MaxCertificateSigningKeyVersions = 2

	realm1 := NewRealmWithDefaults("realm1")
	if err := db.SaveRealm(realm1, SystemTest); err != nil {
		t.Fatal(err)
	}

	// First creates ok
	if _, err := realm1.CreateSigningKeyVersion(ctx, db); err != nil {
		t.Fatal(err)
	}

	// Second creates ok
	if _, err := realm1.CreateSigningKeyVersion(ctx, db); err != nil {
		t.Fatal(err)
	}

	// Third fails over quota
	_, err := realm1.CreateSigningKeyVersion(ctx, db)
	if err == nil {
		t.Fatal("expected error")
	}
	if got, want := err.Error(), "too many available signing keys"; !strings.Contains(got, want) {
		t.Errorf("expected %q to contain %q", got, want)
	}

	// Delete one
	list, err := realm1.ListSigningKeys(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) < 1 {
		t.Fatal("empty list")
	}
	if err := realm1.DestroySigningKeyVersion(ctx, db, list[0].ID); err != nil {
		t.Fatal(err)
	}

	// Third should succeed now
	if _, err := realm1.CreateSigningKeyVersion(ctx, db); err != nil {
		t.Fatal(err)
	}
}
