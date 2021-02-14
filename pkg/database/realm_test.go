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
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/pkg/timeutils"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/pagination"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/jinzhu/gorm"
	"github.com/lib/pq"
)

func TestTestType(t *testing.T) {
	t.Parallel()

	// This test might seem like it's redundant, but it's designed to ensure that
	// the exact values for existing types remain unchanged.
	cases := []struct {
		t   TestType
		exp int
	}{
		{TestTypeConfirmed, 2},
		{TestTypeLikely, 4},
		{TestTypeNegative, 8},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.t.Display(), func(t *testing.T) {
			t.Parallel()

			if got, want := int(tc.t), tc.exp; got != want {
				t.Errorf("Expected %d to be %d", got, want)
			}
		})
	}
}

func TestTestType_Display(t *testing.T) {
	t.Parallel()

	cases := []struct {
		t   TestType
		exp string
	}{
		{TestTypeConfirmed, "confirmed"},
		{TestTypeConfirmed | TestTypeLikely, "confirmed, likely"},
		{TestTypeLikely, "likely"},
		{TestTypeNegative, "negative"},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(fmt.Sprintf("%d", tc.t), func(t *testing.T) {
			t.Parallel()

			if got, want := tc.t.Display(), tc.exp; got != want {
				t.Errorf("expected %q to be %q", got, want)
			}
		})
	}
}

func TestAuthRequirement(t *testing.T) {
	t.Parallel()

	// This test might seem like it's redundant, but it's designed to ensure that
	// the exact values for existing types remain unchanged.
	cases := []struct {
		t   AuthRequirement
		exp int
	}{
		{MFAOptionalPrompt, 0},
		{MFARequired, 1},
		{MFAOptional, 2},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.t.String(), func(t *testing.T) {
			t.Parallel()

			if got, want := int(tc.t), tc.exp; got != want {
				t.Errorf("Expected %d to be %d", got, want)
			}
		})
	}
}

func TestAuthRequirement_String(t *testing.T) {
	t.Parallel()

	cases := []struct {
		t   AuthRequirement
		exp string
	}{
		{MFAOptionalPrompt, "prompt"},
		{MFARequired, "required"},
		{MFAOptional, "optional"},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(fmt.Sprintf("%d", tc.t), func(t *testing.T) {
			t.Parallel()

			if got, want := tc.t.String(), tc.exp; got != want {
				t.Errorf("expected %q to be %q", got, want)
			}
		})
	}
}

func TestRealm_BeforeSave(t *testing.T) {
	t.Parallel()

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
				RegionCode: "USA-IS-A-OK",
			},
			Error: "regionCode cannot be more than 10 characters",
		},
		{
			Name: "enx_region_code_mismatch",
			Input: &Realm{
				RegionCode:      " ",
				EnableENExpress: true,
			},
			Error: "regionCode cannot be blank when using EN Express",
		},
		{
			Name: "system_sms_forbidden",
			Input: &Realm{
				UseSystemSMSConfig:    true,
				CanUseSystemSMSConfig: false,
			},
			Error: "useSystemSMSConfig is not allowed on this realm",
		},
		{
			Name: "system_sms_missing_from",
			Input: &Realm{
				UseSystemSMSConfig:    true,
				CanUseSystemSMSConfig: true,
				SMSFromNumberID:       0,
			},
			Error: "smsFromNumber is required to use the system config",
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
			Error: "smsTextTemplate must contain \"[enslink]\"",
		},
		{
			Name: "enx_link_contains_region",
			Input: &Realm{
				Name:            "a",
				EnableENExpress: true,
				SMSTextTemplate: "[enslink] [region]",
			},
			Error: "smsTextTemplate cannot contain \"[region]\" - this is automatically included in \"[enslink]\"",
		},
		{
			Name: "enx_link_contains_long_code",
			Input: &Realm{
				Name:            "a",
				EnableENExpress: true,
				SMSTextTemplate: "[enslink] [longcode]",
			},
			Error: "smsTextTemplate cannot contain \"[longcode]\" - the long code is automatically included in \"[enslink]\"",
		},
		{
			Name: "link_missing_code",
			Input: &Realm{
				Name:            "a",
				EnableENExpress: false,
				SMSTextTemplate: "call me",
			},
			Error: "smsTextTemplate must contain exactly one of \"[code]\" or \"[longcode]\"",
		},
		{
			Name: "link_both_codess",
			Input: &Realm{
				Name:            "a",
				EnableENExpress: false,
				SMSTextTemplate: "[code][longcode]",
			},
			Error: "smsTextTemplate must contain exactly one of \"[code]\" or \"[longcode]\"",
		},
		{
			Name: "text_too_long",
			Input: &Realm{
				Name:            "a",
				EnableENExpress: false,
				SMSTextTemplate: `Lorem ipsum dolor sit amet, consectetur adipiscing elit. Phasellus nisi erat, sollicitudin eget malesuada vitae, lacinia eget justo. Nunc luctus tincidunt purus vel blandit. Suspendisse eget sapien elit. Vivamus scelerisque, quam vel vestibulum semper, tortor arcu ullamcorper lorem, efficitur imperdiet tellus arcu id lacus. Donec sit amet orci sed dolor interdum venenatis. Nam turpis lectus, pharetra sed lobortis id, aliquam ut neque. Quisque placerat arcu eu blandit finibus. Mauris eleifend nulla et orci vehicula mollis. Quisque ut ante id ante facilisis dignissim.

				Curabitur non massa urna. Phasellus sit amet nisi ut quam dapibus pretium eget in turpis. Phasellus et justo odio. In auctor, felis a tincidunt maximus, nunc erat vehicula ligula, ac posuere felis odio eget mauris. Nulla gravida.`,
			},
			Error: "smsTextTemplate must be 800 characters or less, current message is 807 characters long",
		},
		{
			Name: "text_too_long",
			Input: &Realm{
				Name:            "a",
				EnableENExpress: false,
				SMSTextTemplate: strings.Repeat("[enslink]", 88),
			},
			Error: "smsTextTemplate when expanded, the result message is too long (3168 characters). The max expanded message is 918 characters",
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
			Name: "alternate_sms_template_valid",
			Input: &Realm{
				Name:                      "b",
				CodeLength:                6,
				LongCodeLength:            12,
				EnableENExpress:           false,
				SMSTextTemplate:           valid,
				SMSTextAlternateTemplates: map[string]*string{"alternate1": &valid},
			},
		},
		{
			Name: "system_email_forbidden",
			Input: &Realm{
				UseSystemEmailConfig:    true,
				CanUseSystemEmailConfig: false,
			},
			Error: "useSystemEmailConfig is not allowed on this realm",
		},
		{
			Name: "email_invite_template_missing_link",
			Input: &Realm{
				EmailInviteTemplate: "banana",
			},
			Error: "emailInviteLink must contain \"[invitelink]\"",
		},
		{
			Name: "email_password_reset_template_missing_link",
			Input: &Realm{
				EmailPasswordResetTemplate: "banana",
			},
			Error: "emailPasswordResetTemplate must contain \"[passwordresetlink]\"",
		},
		{
			Name: "email_verify_template_missing_link",
			Input: &Realm{
				EmailVerifyTemplate: "banana",
			},
			Error: "emailVerifyTemplate must contain \"[verifylink]\"",
		},
		{
			Name: "certificate_issuer_blank",
			Input: &Realm{
				UseRealmCertificateKey: true,
				CertificateIssuer:      "",
			},
			Error: "certificateIssuer cannot be blank",
		},
		{
			Name: "certificate_audience_blank",
			Input: &Realm{
				UseRealmCertificateKey: true,
				CertificateAudience:    "",
			},
			Error: "certificateAudience cannot be blank",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			tc.Input.enxRedirectDomainOverride = "https://en.express"

			if err := tc.Input.BeforeSave(&gorm.DB{}); err != nil {
				if tc.Error != "" {
					if got, want := strings.Join(tc.Input.ErrorMessages(), ","), tc.Error; !strings.Contains(got, want) {
						t.Errorf("expected %q to be %q", got, want)
					}
				} else {
					t.Errorf("bad error: %s", err)
				}
			}
		})
	}
}

func TestRealm_BuildSMSText(t *testing.T) {
	t.Parallel()

	realm := NewRealmWithDefaults("test")
	realm.SMSTextTemplate = "This is your Exposure Notifications Verification code: [enslink] Expires in [longexpires] hours"
	realm.RegionCode = "US-WA"

	got, err := realm.BuildSMSText("12345678", "abcdefgh12345678", "en.express", "")
	if err != nil {
		t.Fatal(err)
	}
	want := "This is your Exposure Notifications Verification code: https://us-wa.en.express/v?c=abcdefgh12345678 Expires in 24 hours"
	if got != want {
		t.Errorf("SMS text wrong, want: %q got %q", want, got)
	}

	realm.SMSTextTemplate = "State of Wonder, COVID-19 Exposure Verification code [code]. Expires in [expires] minutes. Act now!"
	got, err = realm.BuildSMSText("654321", "asdflkjasdlkfjl", "", "")
	if err != nil {
		t.Fatal(err)
	}
	want = "State of Wonder, COVID-19 Exposure Verification code 654321. Expires in 15 minutes. Act now!"
	if got != want {
		t.Errorf("SMS text wrong, want: %q got %q", want, got)
	}
}

func TestRealm_BuildInviteEmail(t *testing.T) {
	t.Parallel()

	realm := NewRealmWithDefaults("test")
	realm.EmailInviteTemplate = "Welcome to [realmname] [invitelink]."

	if got, want := realm.BuildInviteEmail("https://join.now"), "Welcome to test https://join.now."; got != want {
		t.Errorf("expected %q to be %q", got, want)
	}
}

func TestRealm_BuildPasswordResetEmail(t *testing.T) {
	t.Parallel()

	realm := NewRealmWithDefaults("test")
	realm.EmailPasswordResetTemplate = "Hey [realmname] reset [passwordresetlink]."

	if got, want := realm.BuildPasswordResetEmail("https://reset.now"), "Hey test reset https://reset.now."; got != want {
		t.Errorf("expected %q to be %q", got, want)
	}
}

func TestRealm_BuildVerifyEmail(t *testing.T) {
	t.Parallel()

	realm := NewRealmWithDefaults("test")
	realm.EmailVerifyTemplate = "Hey [realmname] verify [verifylink]."

	if got, want := realm.BuildVerifyEmail("https://verify.now"), "Hey test verify https://verify.now."; got != want {
		t.Errorf("expected %q to be %q", got, want)
	}
}

func TestRealm_UserStats(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	realm, err := db.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}

	numDays := 7
	endDate := timeutils.Midnight(time.Now())
	startDate := timeutils.Midnight(endDate.Add(time.Duration(numDays) * -24 * time.Hour))

	// Create users.
	for userIdx, name := range []string{"Rocky", "Bullwinkle", "Boris", "Natasha"} {
		user := &User{
			Name:  name,
			Email: name + "@example.com",
		}
		if err := db.SaveUser(user, SystemTest); err != nil {
			t.Fatalf("failed to create user %q: %s", name, err)
		}

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

	stats, err := realm.UserStats(db)
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

func TestRealm_FindVerificationCodeByUUID(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)
	realm, err := db.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	verificationCode := &VerificationCode{
		RealmID:       realm.ID,
		Code:          "1111111",
		LongCode:      "11111111111111111",
		TestType:      "confirmed",
		TestDate:      &now,
		SymptomDate:   &now,
		ExpiresAt:     time.Now().UTC().Add(5 * time.Minute),
		LongExpiresAt: time.Now().UTC().Add(24 * time.Hour),
	}
	if err := db.SaveVerificationCode(verificationCode, realm); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name   string
		uuid   string
		errStr string
		expID  uint
	}{
		{
			name:   "empty",
			uuid:   "",
			errStr: "not found",
		},
		{
			name:   "invalid",
			uuid:   "dfdafa",
			errStr: "not found",
		},
		{
			name:  "found",
			uuid:  verificationCode.UUID,
			expID: verificationCode.ID,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			code, err := realm.FindVerificationCodeByUUID(db, tc.uuid)
			if (err != nil) != (tc.errStr != "") {
				t.Fatal(err)
			}

			if code != nil {
				if got, want := code.ID, tc.expID; got != want {
					t.Errorf("Expected %d to be %d", got, want)
				}
			}
		})
	}
}

func TestRealm_FindRealm(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	realm1 := NewRealmWithDefaults("realm1")
	realm1.RegionCode = "US-MOO"
	if err := db.SaveRealm(realm1, SystemTest); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name   string
		findFn func() (*Realm, error)
	}{
		{
			name:   "find_by_realm",
			findFn: func() (*Realm, error) { return db.FindRealm(realm1.ID) },
		},
		{
			name:   "find_by_name",
			findFn: func() (*Realm, error) { return db.FindRealmByName(realm1.Name) },
		},
		{
			name:   "find_by_region",
			findFn: func() (*Realm, error) { return db.FindRealmByRegion(realm1.RegionCode) },
		},
		{
			name:   "find_by_region_or_id/id",
			findFn: func() (*Realm, error) { return db.FindRealmByRegionOrID(fmt.Sprintf("%d", realm1.ID)) },
		},
		{
			name:   "find_by_region_or_id/region_code",
			findFn: func() (*Realm, error) { return db.FindRealmByRegionOrID(realm1.RegionCode) },
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			found, err := tc.findFn()
			if err != nil {
				t.Fatal(err)
			}

			if got, want := found.Name, realm1.Name; got != want {
				t.Errorf("expected %v to be %v", got, want)
			}
		})
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

	db.config.KeyRing = filepath.Join(project.Root(), "local", "test", "realm")
	db.config.MaxKeyVersions = 2

	realm1 := NewRealmWithDefaults("realm1")
	if err := db.SaveRealm(realm1, SystemTest); err != nil {
		t.Fatal(err)
	}

	// First creates ok
	if _, err := realm1.CreateSigningKeyVersion(ctx, db, SystemTest); err != nil {
		t.Fatal(err)
	}

	// Second creates ok
	if _, err := realm1.CreateSigningKeyVersion(ctx, db, SystemTest); err != nil {
		t.Fatal(err)
	}

	// Third fails over quota
	_, err := realm1.CreateSigningKeyVersion(ctx, db, SystemTest)
	if err == nil {
		t.Fatal("expected error")
	}
	if got, want := err.Error(), "too many available certificate signing keys"; !strings.Contains(got, want) {
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
	if err := realm1.DestroySigningKeyVersion(ctx, db, list[0].ID, SystemTest); err != nil {
		t.Fatal(err)
	}

	// Third should succeed now
	thirdKID, err := realm1.CreateSigningKeyVersion(ctx, db, SystemTest)
	if err != nil {
		t.Fatal(err)
	}

	// Make that key active
	list, err = realm1.ListSigningKeys(db)
	if err != nil {
		t.Fatal(err)
	}
	// Find that key and activate it.
	for _, k := range list {
		if k.GetKID() == thirdKID {
			if _, err := realm1.SetActiveSigningKey(db, k.ID, SystemTest); err != nil {
				t.Fatal(err)
			}
		}
	}

	// Purge the deleted key.
	count, err := db.PurgeSigningKeys(time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 record to be purged, got: %v", count)
	}
}

func TestRealm_CreateSMSSigningKeyVersion(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	db.config.KeyRing = filepath.Join(project.Root(), "local", "test", "realm")
	db.config.MaxKeyVersions = 2

	realm1 := NewRealmWithDefaults("realm1")
	if err := db.SaveRealm(realm1, SystemTest); err != nil {
		t.Fatal(err)
	}

	// First creates ok
	if _, err := realm1.CreateSMSSigningKeyVersion(ctx, db, SystemTest); err != nil {
		t.Fatal(err)
	}

	// Second creates ok
	if _, err := realm1.CreateSMSSigningKeyVersion(ctx, db, SystemTest); err != nil {
		t.Fatal(err)
	}

	// Third fails over quota
	_, err := realm1.CreateSMSSigningKeyVersion(ctx, db, SystemTest)
	if err == nil {
		t.Fatal("expected error")
	}
	if got, want := err.Error(), "too many available SMS signing keys"; !strings.Contains(got, want) {
		t.Errorf("expected %q to contain %q", got, want)
	}

	// Delete one
	list, err := realm1.ListSMSSigningKeys(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) < 1 {
		t.Fatal("empty list")
	}
	if err := realm1.DestroySMSSigningKeyVersion(ctx, db, list[0].ID, SystemTest); err != nil {
		t.Fatal(err)
	}

	// Third should succeed now
	thirdKID, err := realm1.CreateSMSSigningKeyVersion(ctx, db, SystemTest)
	if err != nil {
		t.Fatal(err)
	}

	// Make that key active
	list, err = realm1.ListSMSSigningKeys(db)
	if err != nil {
		t.Fatal(err)
	}
	// Find that key and activate it.
	for _, k := range list {
		if k.GetKID() == thirdKID {
			if _, err := realm1.SetActiveSMSSigningKey(db, k.ID, SystemTest); err != nil {
				t.Fatal(err)
			}
		}
	}

	// Purge the deleted key.
	count, err := db.PurgeSMSSigningKeys(time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 record to be purged, got: %v", count)
	}
}

func TestRealm_ListMemberships(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)
	realm, err := db.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}
	user, err := db.FindUser(1)
	if err != nil {
		t.Fatal(err)
	}
	if err := user.AddToRealm(db, realm, rbac.CodeIssue, SystemTest); err != nil {
		t.Fatal(err)
	}

	deletedUser := &User{
		Name:  "User",
		Email: "foo@bar.com",
	}
	if err := db.SaveUser(deletedUser, SystemTest); err != nil {
		t.Fatal(err)
	}
	if err := deletedUser.AddToRealm(db, realm, rbac.CodeIssue, SystemTest); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Add(-10 * time.Minute)
	deletedUser.DeletedAt = &now
	if err := db.SaveUser(deletedUser, SystemTest); err != nil {
		t.Fatal(err)
	}

	memberships, _, err := realm.ListMemberships(db, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(memberships) != 1 {
		t.Fatalf("expected %#v to have 1 element", memberships)
	}
	if got, want := memberships[0].UserID, user.ID; got != want {
		t.Errorf("Expected %d to be %d", got, want)
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
		TwilioFromNumber: "+15005550006",
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
		if got, want := smsConfig.TwilioFromNumber, "+15005550006"; got != want {
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
		Value: "+15005550000",
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
		if got, want := smsConfig.TwilioFromNumber, "+15005550000"; got != want {
			t.Errorf("expected %v to be %v", got, want)
		}
	}
}

func TestRealm_EmailConfig(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	realm, err := db.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}

	// Initial realm should have no config
	if _, err := realm.EmailConfig(db); err == nil {
		t.Fatalf("expected error")
	}

	// Create config
	if err := db.SaveEmailConfig(&EmailConfig{
		RealmID:      realm.ID,
		SMTPAccount:  "account",
		SMTPHost:     "host",
		SMTPPort:     "port",
		SMTPPassword: "password",
	}); err != nil {
		t.Fatal(err)
	}

	{
		// Now the realm should have a config
		EmailConfig, err := realm.EmailConfig(db)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := EmailConfig.SMTPAccount, "account"; got != want {
			t.Errorf("expected %v to be %v", got, want)
		}
		if got, want := EmailConfig.SMTPHost, "host"; got != want {
			t.Errorf("expected %v to be %v", got, want)
		}
		if got, want := EmailConfig.SMTPPort, "port"; got != want {
			t.Errorf("expected %v to be %v", got, want)
		}
	}

	// Create system config
	if err := db.SaveEmailConfig(&EmailConfig{
		SMTPAccount:  "system-account",
		SMTPHost:     "system-host",
		SMTPPort:     "system-port",
		SMTPPassword: "system-password",
		IsSystem:     true,
	}); err != nil {
		t.Fatal(err)
	}

	// Update to use system config
	realm.CanUseSystemEmailConfig = true
	realm.UseSystemEmailConfig = true
	if err := db.SaveRealm(realm, SystemTest); err != nil {
		t.Fatal(err)
	}

	// The realm should use the system config.
	{
		emailConfig, err := realm.EmailConfig(db)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := emailConfig.SMTPAccount, "system-account"; got != want {
			t.Errorf("expected %v to be %v", got, want)
		}
		if got, want := emailConfig.SMTPHost, "system-host"; got != want {
			t.Errorf("expected %v to be %v", got, want)
		}
		if got, want := emailConfig.SMTPPort, "system-port"; got != want {
			t.Errorf("expected %v to be %v", got, want)
		}
	}
}

func TestRealm_Audits(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	realm := NewRealmWithDefaults("realm1")
	if err := db.SaveRealm(realm, SystemTest); err != nil {
		t.Fatal(err)
	}

	realm.Name = "new name"
	realm.RegionCode = "US-NEW"
	realm.WelcomeMessage = "Welcome changed"
	realm.CodeLength = 6
	realm.CodeDuration = FromDuration(time.Hour)
	realm.LongCodeLength = 12
	realm.LongCodeDuration = FromDuration(6 * time.Hour)
	realm.SMSTextTemplate = "new [code]"
	realm.SMSCountry = "US"
	realm.CanUseSystemSMSConfig = true
	realm.EmailInviteTemplate = "new invite [invitelink]"
	realm.EmailPasswordResetTemplate = "new reset [passwordresetlink]"
	realm.EmailVerifyTemplate = "email verify [verifylink]"
	realm.CanUseSystemEmailConfig = true
	realm.MFAMode = MFARequired
	realm.MFARequiredGracePeriod = FromDuration(time.Hour)
	realm.EmailVerifiedMode = MFARequired
	realm.PasswordRotationPeriodDays = 3
	realm.PasswordRotationWarningDays = 2
	realm.AllowedCIDRsAdminAPI = pq.StringArray([]string{"0.0.0.0/0", "1.1.1.1/0"})
	realm.AllowedCIDRsAPIServer = pq.StringArray([]string{"0.0.0.0/0", "1.1.1.1/0"})
	realm.AllowedCIDRsServer = pq.StringArray([]string{"0.0.0.0/0", "1.1.1.1/0"})
	realm.AllowedTestTypes = TestTypeLikely
	realm.RequireDate = false
	realm.UseRealmCertificateKey = true
	realm.CertificateIssuer = "test issuer"
	realm.CertificateAudience = "test audience"
	realm.CertificateDuration = FromDuration(time.Hour)
	realm.AbusePreventionEnabled = true
	realm.AbusePreventionLimit = 100
	realm.AbusePreventionLimitFactor = 5
	if err := db.SaveRealm(realm, SystemTest); err != nil {
		t.Fatalf("%v, %v", err, realm.errors)
	}

	audits, _, err := db.ListAudits(&pagination.PageParams{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(audits), 32; got != want {
		t.Errorf("expected %d audits, got %d: %v", want, got, audits)
	}
}
