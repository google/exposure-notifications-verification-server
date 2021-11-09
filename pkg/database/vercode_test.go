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
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/uuid"
	"github.com/jinzhu/gorm"
)

func TestCodeType(t *testing.T) {
	t.Parallel()

	// This test might seem like it's redundant, but it's designed to ensure that
	// the exact values for existing types remain unchanged.
	cases := []struct {
		t   CodeType
		exp int
	}{
		{CodeTypeShort, 1},
		{CodeTypeLong, 2},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(fmt.Sprintf("%d", tc.t), func(t *testing.T) {
			t.Parallel()

			if got, want := int(tc.t), tc.exp; got != want {
				t.Errorf("Expected %d to be %d", got, want)
			}
		})
	}
}

func TestVerificationCode_BeforeSave(t *testing.T) {
	t.Parallel()

	t.Run("issuingExternalID", func(t *testing.T) {
		t.Parallel()

		var v VerificationCode
		v.IssuingExternalID = strings.Repeat("*", 256)
		_ = v.BeforeSave(&gorm.DB{})
		if errs := v.ErrorsFor("issuingExternalID"); len(errs) < 1 {
			t.Errorf("expected errors for %s", "issuingExternalID")
		}
	})
}

func TestVerificationCode_FindVerificationCode(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)
	realm, err := db.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}

	uuid := "5148c75c-2bc5-4874-9d1c-f9185d0e1b8a"
	code := "12345678"
	longCode := "abcdefgh12345678"

	vc := &VerificationCode{
		Realm:         realm,
		UUID:          uuid,
		Code:          code,
		LongCode:      longCode,
		TestType:      "confirmed",
		ExpiresAt:     time.Now().Add(time.Hour),
		LongExpiresAt: time.Now().Add(2 * time.Hour),
	}

	if err := realm.SaveVerificationCode(db, vc); err != nil {
		t.Fatalf("error creating verification code: %v", err)
	}

	{
		// Find by raw code
		got, err := realm.FindVerificationCode(db, code)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := got.Code, code; got == want {
			t.Errorf("expected %#v to not be %#v (should be hmac)", got, want)
		}
	}

	{
		// Find by raw long code
		got, err := realm.FindVerificationCode(db, longCode)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := got.LongCode, longCode; got == want {
			t.Errorf("expected %#v to not be %#v (should be hmac)", got, want)
		}
	}

	vc.Claimed = true
	if err := realm.SaveVerificationCode(db, vc); err != nil {
		t.Fatal(err)
	}
}

func TestVerificationCode_FindVerificationCodeByUUID(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	realm := NewRealmWithDefaults("testRealm")
	if err := db.SaveRealm(realm, SystemTest); err != nil {
		t.Fatal(err)
	}

	otherRealm := NewRealmWithDefaults("notThetestRealm")
	if err := db.SaveRealm(otherRealm, SystemTest); err != nil {
		t.Fatal(err)
	}

	vc := &VerificationCode{
		Code:          "123456",
		LongCode:      "defghijk329024",
		TestType:      "confirmed",
		RealmID:       realm.ID,
		ExpiresAt:     time.Now().Add(time.Hour),
		LongExpiresAt: time.Now().Add(2 * time.Hour),
	}

	if err := realm.SaveVerificationCode(db, vc); err != nil {
		t.Fatal(err)
	}

	codeUUID := vc.UUID
	if codeUUID == "" {
		t.Fatal("expected uuid")
	}

	t.Run("normal_find", func(t *testing.T) {
		t.Parallel()

		got, err := realm.FindVerificationCodeByUUID(db, codeUUID)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := got.ID, vc.ID; got != want {
			t.Errorf("expected %#v to be %#v", got, want)
		}
	})

	t.Run("wrong_realm", func(t *testing.T) {
		t.Parallel()

		_, err := otherRealm.FindVerificationCodeByUUID(db, codeUUID)
		if err == nil || !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatalf("expected error: not found, got: %v", err)
		}
	})

	t.Run("wrong_uuid", func(t *testing.T) {
		t.Parallel()

		_, err := realm.FindVerificationCodeByUUID(db, uuid.Must(uuid.NewRandom()).String())
		if err == nil || !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatalf("expected error: not found, got: %v", err)
		}
	})
}

func TestVerificationCode_ListRecentCodes(t *testing.T) {
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

	vc := &VerificationCode{
		RealmID:       realm.ID,
		IssuingUserID: user.ID,
		Code:          "123456",
		LongCode:      "defghijk329024",
		TestType:      "confirmed",
		ExpiresAt:     time.Now().Add(time.Hour),
		LongExpiresAt: time.Now().Add(2 * time.Hour),
	}

	if err := realm.SaveVerificationCode(db, vc); err != nil {
		t.Fatal(err, vc.ErrorMessages())
	}

	uuid := vc.UUID
	if uuid == "" {
		t.Fatal("expected uuid")
	}

	{
		got, err := realm.ListRecentCodes(db, user)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := got[0].ID, vc.ID; got != want {
			t.Errorf("expected %#v to be %#v", got, want)
		}
	}
}

func TestVerificationCode_ExpireVerificationCode(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)
	realm, err := db.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}

	vc := &VerificationCode{
		Realm:         realm,
		Code:          "123456",
		LongCode:      "defghijk329024",
		TestType:      "confirmed",
		ExpiresAt:     time.Now().Add(time.Hour),
		LongExpiresAt: time.Now().Add(2 * time.Hour),
	}

	if err := realm.SaveVerificationCode(db, vc); err != nil {
		t.Fatal(err)
	}

	uuid := vc.UUID
	if uuid == "" {
		t.Fatal("expected uuid")
	}

	{
		got, err := realm.ExpireCode(db, uuid, SystemTest)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := got.ID, vc.ID; got != want {
			t.Errorf("expected %#v to be %#v", got, want)
		}
		if got.ExpiresAt.After(time.Now()) {
			t.Errorf("expected expired, got %v", got.ExpiresAt)
		}
	}

	if _, err := realm.ExpireCode(db, uuid, SystemTest); err != nil {
		t.Fatal(err)
	}
}

func TestSaveUserReport(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)
	realm := NewRealmWithDefaults("The Grid")
	realm.AddUserReportToAllowedTestTypes()
	realm.SMSCountry = "us"
	if err := db.SaveRealm(realm, SystemTest); err != nil {
		t.Fatal(err)
	}

	vc := &VerificationCode{
		Realm:         realm,
		Code:          "123456",
		LongCode:      "defghijk329024",
		TestType:      "user-report",
		ExpiresAt:     time.Now().Add(time.Hour),
		LongExpiresAt: time.Now().Add(2 * time.Hour),
		Nonce:         generateNonce(t),
		PhoneNumber:   "+12068675309",
		NonceRequired: true,
	}

	if err := realm.SaveVerificationCode(db, vc); err != nil {
		t.Fatal(err, vc.ErrorMessages())
	}

	var userReport *UserReport
	var err error
	err = db.db.Transaction(func(tx *gorm.DB) error {
		userReport, err = db.FindUserReport(tx, vc.PhoneNumber)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if vc.UserReportID == nil || *vc.UserReportID != userReport.ID {
		t.Fatalf("userReportID not saved on verification code")
	}
	if userReport.CodeClaimed {
		t.Fatalf("user report was created in a claimed state")
	}
}

func TestVerCodeValidate(t *testing.T) {
	t.Parallel()

	realm := NewRealmWithDefaults("Test Realm")
	cases := []struct {
		Name string
		Code VerificationCode
		Err  error
	}{
		{
			Name: "code_too_short",
			Code: VerificationCode{Code: "1"},
			Err:  ErrCodeTooShort,
		},
		{
			Name: "invalid_test_type",
			Code: VerificationCode{
				Code:     "123456",
				LongCode: "123456",
				TestType: "self-reported",
			},
			Err: ErrInvalidTestType,
		},
		{
			Name: "already_expired",
			Code: VerificationCode{
				Code:      "123456",
				LongCode:  "123456",
				TestType:  "negative",
				ExpiresAt: time.Now().Add(-1 * time.Second),
			},
			Err: ErrCodeAlreadyExpired,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			if err := tc.Code.Validate(realm); !errors.Is(err, tc.Err) {
				t.Fatalf("wrong error, want %v, got: %v", tc.Err, err)
			}
		})
	}
}

func TestVerCodeIsExpired(t *testing.T) {
	t.Parallel()

	code := VerificationCode{
		Code:      "12345678",
		TestType:  "confirmed",
		ExpiresAt: time.Now().Add(time.Hour),
	}

	if got := code.IsExpired(); got {
		t.Errorf("code says expired, when shouldn't be")
	}
}

func TestDeleteVerificationCode(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)
	realm, err := db.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}

	code := VerificationCode{
		Realm:         realm,
		Code:          "12345678",
		LongCode:      "12345678",
		TestType:      "confirmed",
		ExpiresAt:     time.Now().Add(time.Hour),
		LongExpiresAt: time.Now().Add(time.Hour),
	}

	if err := realm.SaveVerificationCode(db, &code); err != nil {
		t.Fatal(err)
	}

	if err := realm.DeleteVerificationCode(db, code.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := realm.FindVerificationCode(db, "12345678"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatal(err)
	}
}

func TestVerificationCodesCleanup(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	now := time.Now()

	realm, err := db.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}

	cleanUpTo := 1

	testData := []*VerificationCode{
		{Code: "111111", LongCode: "111111", RealmID: realm.ID, TestType: "negative", ExpiresAt: now.Add(time.Second), LongExpiresAt: now.Add(time.Second)},
		{Code: "222222", LongCode: "222222", RealmID: realm.ID, TestType: "negative", ExpiresAt: now.Add(time.Second), LongExpiresAt: now.Add(time.Second)},
		// Cleanup line - will be cleaned above here.
		{Code: "333333", LongCode: "333333ABCDEF", RealmID: realm.ID, TestType: "negative", ExpiresAt: now.Add(time.Minute), LongExpiresAt: now.Add(time.Hour)},
	}
	for _, rec := range testData {
		if err := realm.SaveVerificationCode(db, rec); err != nil {
			t.Fatal(err, rec.ErrorMessages())
		}
	}

	// Need to let some time lapse since we can't back date records through normal channels.
	time.Sleep(2 * time.Second)

	if count, err := db.RecycleVerificationCodes(time.Millisecond * 500); err != nil {
		t.Fatal(err)
	} else if count != 2 {
		t.Fatalf("purge record count mismatch, want: 2, got: %v", count)
	}

	// Find first two by UUID.
	for i, vc := range testData {
		got, err := realm.FindVerificationCodeByUUID(db, vc.UUID)
		if err != nil {
			t.Fatal(err)
		}

		if i <= cleanUpTo {
			if got.Code != "" {
				t.Errorf("expected code to be empty, got: %v", got.Code)
			}
			if got.LongCode != "" {
				t.Errorf("expected code to be empty, got: %v", got.LongCode)
			}
		} else {
			if got.Code == "" {
				t.Errorf("expected code to not be empty, but was")
			}
			if got.LongCode == "" {
				t.Errorf("expected long code to not be empty, but was")
			}
		}
	}

	// Run the purge.
	if count, err := db.PurgeVerificationCodes(time.Millisecond * 500); err != nil {
		t.Fatalf("error doing purge: %v", err)
	} else if count != 2 {
		t.Fatalf("purge record count mismatch, want: 2, got: %v", count)
	}

	// Find first two by UUID, expect a not found error
	for i, vc := range testData {
		got, err := realm.FindVerificationCodeByUUID(db, vc.UUID)
		if i <= cleanUpTo {
			if err == nil {
				t.Fatalf("expected error, got: %v", got)
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				t.Fatalf("wrong error, want: %v got: %v", gorm.ErrRecordNotFound, err)
			}
		} else {
			if got, want := got.Code, testData[i].Code; got != want {
				t.Errorf("expected %q to be %q", got, want)
			}
			if got, want := got.LongCode, testData[i].LongCode; got != want {
				t.Errorf("expected %q to be %q", got, want)
			}
		}
	}
}

func TestStatDates(t *testing.T) {
	// Please note, this test is NOT exhaustive. A better engineer would try
	// all dates, and a bunch of corner cases. This is intended as a
	// smokescreen.
	t.Parallel()

	ctx := project.TestContext(t)

	db, _ := testDatabaseInstance.NewDatabase(t, nil)
	realm, err := db.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}

	user, err := db.FindUser(1)
	if err != nil {
		t.Fatal(err)
	}

	app := &AuthorizedApp{
		RealmID: realm.ID,
		Name:    "appy",
	}
	if err := db.SaveAuthorizedApp(app, SystemTest); err != nil {
		t.Fatal(err)
	}

	cacher, err := cache.NewInMemory(nil)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	nowStr := now.Format(project.RFC3339Date)

	tests := []struct {
		code     *VerificationCode
		statDate string
	}{
		{
			&VerificationCode{
				Realm:             realm,
				Code:              "111111",
				LongCode:          "111111",
				TestType:          "negative",
				ExpiresAt:         now.Add(time.Second),
				LongExpiresAt:     now.Add(time.Second),
				IssuingUserID:     user.ID,    // need for RealmUserStats
				IssuingAppID:      app.ID,     // need for AuthorizedAppStats
				IssuingExternalID: "aa-bb-cc", // need for ExternalIssuerStats
			},
			nowStr,
		},
	}

	for i, test := range tests {
		if err := realm.SaveVerificationCode(db, test.code); err != nil {
			t.Fatal(err, test.code.ErrorMessages())
		}

		test.code.Code = "111111"
		db.UpdateStats(ctx, test.code)

		{
			var stats []*RealmUserStat
			if err := db.db.
				Model(&UserStat{}).
				Select("*").
				Scan(&stats).
				Error; err != nil {
				if IsNotFound(err) {
					t.Fatalf("[%d] Error grabbing user stats %v", i, err)
				}
			}
			if len(stats) != 1 {
				t.Fatalf("[%d] expected one user stat", i)
			}
			if stats[0].CodesIssued != uint(i+1) {
				t.Errorf("[%d] expected stat.CodesIssued = %d, expected %d", i, stats[0].CodesIssued, i+1)
			}
			if f := stats[0].Date.Format(project.RFC3339Date); f != test.statDate {
				t.Errorf("[%d] expected stat.Date = %s, expected %s", i, f, test.statDate)
			}

			if _, err := user.StatsCached(ctx, db, cacher, realm); err != nil {
				t.Fatalf("error getting stats: %v", err)
			}
		}

		if len(test.code.IssuingExternalID) != 0 {
			var stats []*ExternalIssuerStat
			if err := db.db.
				Model(&ExternalIssuerStat{}).
				Select("*").
				Scan(&stats).
				Error; err != nil {
				if IsNotFound(err) {
					t.Fatalf("[%d] Error grabbing external issuer stats %v", i, err)
				}
			}
			if len(stats) != 1 {
				t.Fatalf("[%d] expected one user stat", i)
			}
			if stats[0].CodesIssued != uint(i+1) {
				t.Errorf("[%d] expected stat.CodesIssued = %d, expected %d", i, stats[0].CodesIssued, i+1)
			}
			if f := stats[0].Date.Format(project.RFC3339Date); f != test.statDate {
				t.Errorf("[%d] expected stat.Date = %s, expected %s", i, f, test.statDate)
			}

			if _, err := realm.ExternalIssuerStatsCached(ctx, db, cacher); err != nil {
				t.Fatalf("error getting stats: %v", err)
			}
		}

		{
			var stats []*AuthorizedAppStat
			if err := db.db.
				Model(&AuthorizedAppStat{}).
				Select("*").
				Scan(&stats).
				Error; err != nil {
				if IsNotFound(err) {
					t.Fatalf("[%d] Error grabbing app stats %v", i, err)
				}
			}
			if len(stats) != 1 {
				t.Fatalf("[%d] expected one user stat", i)
			}
			if stats[0].CodesIssued != uint(i+1) {
				t.Errorf("[%d] expected stat.CodesIssued = %d, expected %d", i, stats[0].CodesIssued, i+1)
			}
			if f := stats[0].Date.Format(project.RFC3339Date); f != test.statDate {
				t.Errorf("[%d] expected stat.Date = %s, expected %s", i, f, test.statDate)
			}

			if _, err := app.StatsCached(ctx, db, cacher); err != nil {
				t.Fatalf("error getting stats: %v", err)
			}
		}

		{
			var stats []*RealmStat
			if err := db.db.
				Model(&RealmStats{}).
				Select("*").
				Scan(&stats).
				Error; err != nil {
				if IsNotFound(err) {
					t.Fatalf("[%d] Error grabbing realm stats %v", i, err)
				}
			}
			if len(stats) != 1 {
				t.Fatalf("[%d] expected one user stat", i)
			}
			if stats[0].CodesIssued != uint(i+1) {
				t.Errorf("[%d] expected stat.CodesIssued = %d, expected %d", i, stats[0].CodesIssued, i+1)
			}
			if f := stats[0].Date.Format(project.RFC3339Date); f != test.statDate {
				t.Errorf("[%d] expected stat.Date = %s, expected %s", i, f, test.statDate)
			}

			if _, err := realm.StatsCached(ctx, db, cacher); err != nil {
				t.Fatalf("error getting stats: %v", err)
			}
		}
	}
}
