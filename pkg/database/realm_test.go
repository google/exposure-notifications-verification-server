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
	"os"
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

func TestValidation(t *testing.T) {
	db, _ := testDatabaseInstance.NewDatabase(t, nil)
	os.Setenv("ENX_REDIRECT_DOMAIN", "https://en.express")

	valid := "State of Wonder, COVID-19 Exposure Verification code [code]. Expires in [expires] minutes. Act now!"

	cases := []struct {
		Name  string
		Input *Realm
		Error string
	}{
		{
			Name: "empty_name",
			Input: &Realm{
				Name: " ",
			},
			Error: "name cannot be blank",
		},
		{
			Name: "region_code_too_long",
			Input: &Realm{
				Name:       "foo",
				RegionCode: "USA-IS-A-OK",
			},
			Error: "regionCode cannot be more than 10 characters",
		},
		{
			Name: "enx_region_code_mismatch",
			Input: &Realm{
				Name:            "foo",
				RegionCode:      " ",
				EnableENExpress: true,
			},
			Error: "regionCode cannot be blank when using EN Express",
		},
		{
			Name: "rotation_warning_too_big",
			Input: &Realm{
				Name:                        "a",
				PasswordRotationPeriodDays:  42,
				PasswordRotationWarningDays: 43,
			},
			Error: "passwordWarn may not be longer than password rotation period",
		},
		{
			Name: "code_length_too_short",
			Input: &Realm{
				Name:       "a",
				CodeLength: 5,
			},
			Error: "codeLength must be at least 6",
		},
		{
			Name: "code_duration_too_long",
			Input: &Realm{
				Name:         "a",
				CodeLength:   6,
				CodeDuration: FromDuration(time.Hour + time.Minute),
			},
			Error: "codeDuration must be no more than 1 hour",
		},
		{
			Name: "long_code_length_too_short",
			Input: &Realm{
				Name:           "a",
				CodeLength:     6,
				LongCodeLength: 11,
			},
			Error: "longCodeLength must be at least 12",
		},
		{
			Name: "long_code_duration_too_long",
			Input: &Realm{
				Name:             "a",
				CodeLength:       6,
				LongCodeLength:   12,
				LongCodeDuration: FromDuration(24*time.Hour + time.Second),
			},
			Error: "longCodeDuration must be no more than 24 hours",
		},
		{
			Name: "missing_enx_link",
			Input: &Realm{
				Name:            "a",
				EnableENExpress: true,
				SMSTextTemplate: "call 1-800-555-1234",
			},
			Error: "SMSTextTemplate must contain \"[enslink]\"",
		},
		{
			Name: "enx_link_contains_region",
			Input: &Realm{
				Name:            "a",
				EnableENExpress: true,
				SMSTextTemplate: "[enslink] [region]",
			},
			Error: "SMSTextTemplate cannot contain \"[region]\" - this is automatically included in \"[enslink]\"",
		},
		{
			Name: "enx_link_contains_long_code",
			Input: &Realm{
				Name:            "a",
				EnableENExpress: true,
				SMSTextTemplate: "[enslink] [longcode]",
			},
			Error: "SMSTextTemplate cannot contain \"[longcode]\" - the long code is automatically included in \"[enslink]\"",
		},
		{
			Name: "link_missing_code",
			Input: &Realm{
				Name:            "a",
				EnableENExpress: false,
				SMSTextTemplate: "call me",
			},
			Error: "SMSTextTemplate must contain exactly one of \"[code]\" or \"[longcode]\"",
		},
		{
			Name: "link_both_codess",
			Input: &Realm{
				Name:            "a",
				EnableENExpress: false,
				SMSTextTemplate: "[code][longcode]",
			},
			Error: "SMSTextTemplate must contain exactly one of \"[code]\" or \"[longcode]\"",
		},
		{
			Name: "text_too_long",
			Input: &Realm{
				Name:            "a",
				EnableENExpress: false,
				SMSTextTemplate: `Lorem ipsum dolor sit amet, consectetur adipiscing elit. Phasellus nisi erat, sollicitudin eget malesuada vitae, lacinia eget justo. Nunc luctus tincidunt purus vel blandit. Suspendisse eget sapien elit. Vivamus scelerisque, quam vel vestibulum semper, tortor arcu ullamcorper lorem, efficitur imperdiet tellus arcu id lacus. Donec sit amet orci sed dolor interdum venenatis. Nam turpis lectus, pharetra sed lobortis id, aliquam ut neque. Quisque placerat arcu eu blandit finibus. Mauris eleifend nulla et orci vehicula mollis. Quisque ut ante id ante facilisis dignissim.

				Curabitur non massa urna. Phasellus sit amet nisi ut quam dapibus pretium eget in turpis. Phasellus et justo odio. In auctor, felis a tincidunt maximus, nunc erat vehicula ligula, ac posuere felis odio eget mauris. Nulla gravida.`,
			},
			Error: "SMSTextTemplate must be 800 characters or less, current message is 807 characters long",
		},
		{
			Name: "text_too_long",
			Input: &Realm{
				Name:            "a",
				EnableENExpress: false,
				SMSTextTemplate: strings.Repeat("[enslink]", 88),
			},
			Error: "SMSTextTemplate when expanded, the result message is too long (3168 characters). The max expanded message is 918 characters",
		},
		{
			Name: "valid",
			Input: &Realm{
				Name:            "a",
				CodeLength:      6,
				LongCodeLength:  12,
				EnableENExpress: false,
				SMSTextTemplate: valid,
			},
		},
		{
			Name: "alternate_sms_template",
			Input: &Realm{
				Name:                      "a",
				EnableENExpress:           false,
				SMSTextTemplate:           valid,
				SMSTextAlternateTemplates: map[string]*string{"alternate1": nil},
			},
			Error: "no template for label alternate1",
		},
		{
			Name: "alternate_sms_template valid",
			Input: &Realm{
				Name:                      "b",
				CodeLength:                6,
				LongCodeLength:            12,
				EnableENExpress:           false,
				SMSTextTemplate:           valid,
				SMSTextAlternateTemplates: map[string]*string{"alternate1": &valid},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			if err := db.SaveRealm(tc.Input, SystemTest); err == nil {
				if tc.Error != "" {
					t.Fatalf("expected error: %q got: nil", tc.Error)
				}
			} else if tc.Error == "" {
				t.Fatalf("expected no error, got %q", err.Error())
			} else if !strings.Contains(err.Error(), tc.Error) {
				t.Fatalf("wrong error, want: %q got: %q", tc.Error, err.Error())
			}
		})
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
			stat := &UserStat{
				RealmID:     realm.ID,
				UserID:      user.ID,
				Date:        startDate.Add(time.Duration(i) * 24 * time.Hour),
				CodesIssued: uint(10 + i + userIdx),
			}
			if err := db.SaveUserStat(stat); err != nil {
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

func TestRealm_SMSConfig(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	realm := NewRealmWithDefaults("test")
	if err := db.SaveRealm(realm, SystemTest); err != nil {
		t.Fatalf("error saving realm: %v", err)
	}

	// Initial realm should have no config
	if _, err := realm.SMSConfig(db); err == nil {
		t.Fatalf("expected error")
	}

	// Create config
	if err := db.SaveSMSConfig(&SMSConfig{
		RealmID:          realm.ID,
		TwilioAccountSid: "sid",
		TwilioAuthToken:  "token",
		TwilioFromNumber: "111-111-1111",
	}); err != nil {
		t.Fatal(err)
	}

	{
		// Now the realm should have a config
		smsConfig, err := realm.SMSConfig(db)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := smsConfig.TwilioAccountSid, "sid"; got != want {
			t.Errorf("expected %v to be %v", got, want)
		}
		if got, want := smsConfig.TwilioAuthToken, "token"; got != want {
			t.Errorf("expected %v to be %v", got, want)
		}
		if got, want := smsConfig.TwilioFromNumber, "111-111-1111"; got != want {
			t.Errorf("expected %v to be %v", got, want)
		}
	}

	// Create system config
	if err := db.SaveSMSConfig(&SMSConfig{
		TwilioAccountSid: "system-sid",
		TwilioAuthToken:  "system-token",
		IsSystem:         true,
	}); err != nil {
		t.Fatal(err)
	}

	// Create from number
	smsFromNumber := &SMSFromNumber{
		Label: "Default",
		Value: "222-222-2222",
	}
	if err := db.CreateOrUpdateSMSFromNumbers([]*SMSFromNumber{smsFromNumber}); err != nil {
		t.Fatal(err)
	}

	// Update to use system config
	realm.CanUseSystemSMSConfig = true
	realm.UseSystemSMSConfig = true
	realm.SMSFromNumberID = smsFromNumber.ID
	if err := db.SaveRealm(realm, SystemTest); err != nil {
		t.Fatal(err)
	}

	// The realm should use the system config.
	{
		smsConfig, err := realm.SMSConfig(db)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := smsConfig.TwilioAccountSid, "system-sid"; got != want {
			t.Errorf("expected %v to be %v", got, want)
		}
		if got, want := smsConfig.TwilioAuthToken, "system-token"; got != want {
			t.Errorf("expected %v to be %v", got, want)
		}
		if got, want := smsConfig.TwilioFromNumber, "222-222-2222"; got != want {
			t.Errorf("expected %v to be %v", got, want)
		}
	}
}
