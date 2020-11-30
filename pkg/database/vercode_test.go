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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/jinzhu/gorm"
)

func TestVerificationCode_FindVerificationCode(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	uuid := "5148c75c-2bc5-4874-9d1c-f9185d0e1b8a"
	code := "12345678"
	longCode := "abcdefgh12345678"

	maxAge := time.Hour
	vc := &VerificationCode{
		UUID:          uuid,
		Code:          code,
		LongCode:      longCode,
		TestType:      "confirmed",
		ExpiresAt:     time.Now().Add(time.Hour),
		LongExpiresAt: time.Now().Add(2 * time.Hour),
	}

	if err := db.SaveVerificationCode(vc, maxAge); err != nil {
		t.Fatalf("error creating verification code: %v", err)
	}

	{
		// Find by raw code
		got, err := db.FindVerificationCode(code)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := got.Code, code; got == want {
			t.Errorf("expected %#v to not be %#v (should be hmac)", got, want)
		}
	}

	{
		// Find by raw long code
		got, err := db.FindVerificationCode(longCode)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := got.LongCode, longCode; got == want {
			t.Errorf("expected %#v to not be %#v (should be hmac)", got, want)
		}
	}

	vc.Claimed = true
	if err := db.SaveVerificationCode(vc, maxAge); err != nil {
		t.Fatal(err)
	}
}

func TestVerificationCode_FindVerificationCodeByUUID(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	realm, err := db.CreateRealm("testRealm")
	if err != nil {
		t.Fatalf("failed to create realm: %v", err)
	}
	otherRealm, err := db.CreateRealm("notThetestRealm")
	if err != nil {
		t.Fatalf("failed to create realm: %v", err)
	}

	vc := &VerificationCode{
		Code:          "123456",
		LongCode:      "defghijk329024",
		TestType:      "confirmed",
		RealmID:       realm.ID,
		ExpiresAt:     time.Now().Add(time.Hour),
		LongExpiresAt: time.Now().Add(2 * time.Hour),
	}

	if err := db.SaveVerificationCode(vc, time.Hour); err != nil {
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

	var realmID uint = 123
	var userID uint = 456

	vc := &VerificationCode{
		RealmID:       realmID,
		IssuingUserID: userID,
		Code:          "123456",
		LongCode:      "defghijk329024",
		TestType:      "confirmed",
		ExpiresAt:     time.Now().Add(time.Hour),
		LongExpiresAt: time.Now().Add(2 * time.Hour),
	}

	if err := db.SaveVerificationCode(vc, time.Hour); err != nil {
		t.Fatal(err)
	}

	uuid := vc.UUID
	if uuid == "" {
		t.Fatal("expected uuid")
	}

	{
		r := &Realm{Model: gorm.Model{ID: realmID}}
		u := &User{Model: gorm.Model{ID: userID}}
		got, err := db.ListRecentCodes(r, u)
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

	vc := &VerificationCode{
		Code:          "123456",
		LongCode:      "defghijk329024",
		TestType:      "confirmed",
		ExpiresAt:     time.Now().Add(time.Hour),
		LongExpiresAt: time.Now().Add(2 * time.Hour),
	}

	if err := db.SaveVerificationCode(vc, time.Hour); err != nil {
		t.Fatal(err)
	}

	uuid := vc.UUID
	if uuid == "" {
		t.Fatal("expected uuid")
	}

	{
		got, err := db.ExpireCode(uuid)
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

	if _, err := db.ExpireCode(uuid); err == nil {
		t.Errorf("Expected code already expired, got %v", err)
	}
}

func TestVerCodeValidate(t *testing.T) {
	t.Parallel()

	maxAge := time.Hour * 24 * 14
	oldTest := time.Now().Add(-1 * 20 * oneDay)
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
			Name: "invalid_symptom_date",
			Code: VerificationCode{
				Code:        "123456",
				LongCode:    "123456",
				TestType:    "negative",
				SymptomDate: &oldTest,
			},
			Err: ErrTestTooOld,
		},
		{
			Name: "invalid_test_date",
			Code: VerificationCode{
				Code:     "123456",
				LongCode: "123456",
				TestType: "negative",
				TestDate: &oldTest,
			},
			Err: ErrTestTooOld,
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

			if err := tc.Code.Validate(maxAge); err != tc.Err {
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

	maxAge := time.Hour
	code := VerificationCode{
		Code:          "12345678",
		LongCode:      "12345678",
		TestType:      "confirmed",
		ExpiresAt:     time.Now().Add(time.Hour),
		LongExpiresAt: time.Now().Add(time.Hour),
	}

	if err := db.SaveVerificationCode(&code, maxAge); err != nil {
		t.Fatal(err)
	}

	if err := db.DeleteVerificationCode("12345678"); err != nil {
		t.Fatal(err)
	}

	if _, err := db.FindVerificationCode("12345678"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatal(err)
	}
}

func TestVerificationCodesCleanup(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	now := time.Now()
	maxAge := time.Hour // not important to this test case

	realm, err := db.CreateRealm("realmy")
	if err != nil {
		t.Fatalf("couldn't create test realm: %v", realm)
	}

	cleanUpTo := 1

	testData := []*VerificationCode{
		{Code: "111111", LongCode: "111111", RealmID: realm.ID, TestType: "negative", ExpiresAt: now.Add(time.Second), LongExpiresAt: now.Add(time.Second)},
		{Code: "222222", LongCode: "222222", RealmID: realm.ID, TestType: "negative", ExpiresAt: now.Add(time.Second), LongExpiresAt: now.Add(time.Second)},
		// Cleanup line - will be cleaned above here.
		{Code: "333333", LongCode: "333333ABCDEF", RealmID: realm.ID, TestType: "negative", ExpiresAt: now.Add(time.Minute), LongExpiresAt: now.Add(time.Hour)},
	}
	for _, rec := range testData {
		if err := db.SaveVerificationCode(rec, maxAge); err != nil {
			t.Fatalf("can't save test data: %v", err)
		}
	}

	// Need to let some time lapse since we can't back date records through normal channels.
	time.Sleep(2 * time.Second)

	if count, err := db.RecycleVerificationCodes(time.Millisecond * 500); err != nil {
		t.Fatalf("error doing purge: %v", err)
	} else if count != 2 {
		t.Fatalf("purge record count mismatch, want: 2, got: %v", count)
	}

	// Find first two by UUID.
	for i, vc := range testData {
		got, err := realm.FindVerificationCodeByUUID(db, vc.UUID)
		if err != nil {
			t.Fatalf("can't read back code by UUID")
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
			if diff := cmp.Diff(testData[i], got, ApproxTime, cmpopts.IgnoreUnexported(VerificationCode{}), cmpopts.IgnoreUnexported(Errorable{})); diff != "" {
				t.Fatalf("mismatch (-want, +got):\n%s", diff)
			}
		}
	}
}

func TestStatDatesOnCreate(t *testing.T) {
	// Please note, this test is NOT exhaustive. A better engineer would try
	// all dates, and a bunch of corner cases. This is intended as a
	// smokescreen.
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	fmtString := "2006-01-02"
	now := time.Now()
	nowStr := now.Format(fmtString)
	maxAge := time.Hour

	tests := []struct {
		code     *VerificationCode
		statDate string
	}{
		{
			&VerificationCode{
				Code:              "111111",
				LongCode:          "111111",
				TestType:          "negative",
				ExpiresAt:         now.Add(time.Second),
				LongExpiresAt:     now.Add(time.Second),
				IssuingUserID:     100,        // need for RealmUserStats
				IssuingAppID:      200,        // need for AuthorizedAppStats
				IssuingExternalID: "aa-bb-cc", // need for ExternalIssuerStats
				RealmID:           300,        // need for RealmStats
			},
			nowStr,
		},
	}

	for i, test := range tests {
		if err := db.SaveVerificationCode(test.code, maxAge); err != nil {
			t.Fatalf("[%d] error saving code: %v", i, err)
		}

		{
			var stats []*RealmUserStat
			if err := db.db.
				Model(&UserStats{}).
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
			if f := stats[0].Date.Format(fmtString); f != test.statDate {
				t.Errorf("[%d] expected stat.Date = %s, expected %s", i, f, test.statDate)
			}
		}

		if len(test.code.IssuingExternalID) != 0 {
			var stats []*ExternalIssuerStat
			if err := db.db.
				Model(&ExternalIssuerStats{}).
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
			if f := stats[0].Date.Format(fmtString); f != test.statDate {
				t.Errorf("[%d] expected stat.Date = %s, expected %s", i, f, test.statDate)
			}
		}

		{
			var stats []*AuthorizedAppStats
			if err := db.db.
				Model(&AuthorizedAppStats{}).
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
			if f := stats[0].Date.Format(fmtString); f != test.statDate {
				t.Errorf("[%d] expected stat.Date = %s, expected %s", i, f, test.statDate)
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
			if f := stats[0].Date.Format(fmtString); f != test.statDate {
				t.Errorf("[%d] expected stat.Date = %s, expected %s", i, f, test.statDate)
			}
		}
	}
}
