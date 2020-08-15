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
	"github.com/jinzhu/gorm"
)

func TestSaveVerCode(t *testing.T) {
	t.Parallel()
	db := NewTestDatabase(t)

	uuid := "5148c75c-2bc5-4874-9d1c-f9185d0e1b8a"

	maxAge := time.Hour
	code := VerificationCode{
		UUID:          uuid,
		Code:          "12345678",
		LongCode:      "abcdefgh12345678",
		TestType:      "confirmed",
		ExpiresAt:     time.Now().Add(time.Hour),
		LongExpiresAt: time.Now().Add(2 * time.Hour),
	}

	if err := db.SaveVerificationCode(&code, maxAge); err != nil {
		t.Fatalf("error creating verification code: %v", err)
	}

	got, err := db.FindVerificationCode(code.Code)
	if err != nil {
		t.Fatalf("error reading code from db: %v", err)
	}
	if diff := cmp.Diff(code, *got, approxTime); diff != "" {
		t.Fatalf("mismatch (-want, +got):\n%s", diff)
	}

	// find by long code
	{
		got, err := db.FindVerificationCode(code.LongCode)
		if err != nil {
			t.Fatalf("error reading code from db: %v", err)
		}
		if diff := cmp.Diff(code, *got, approxTime); diff != "" {
			t.Fatalf("mismatch (-want, +got):\n%s", diff)
		}
	}

	code.Claimed = true
	if err := db.SaveVerificationCode(&code, maxAge); err != nil {
		t.Fatalf("error claiming verification code: %v", err)
	}

	got, err = db.FindVerificationCodeByUUID(uuid)
	if err != nil {
		t.Fatalf("error reading code from db: %v", err)
	}
	if diff := cmp.Diff(code, *got, approxTime); diff != "" {
		t.Fatalf("mismatch (-want, +got):\n%s", diff)
	}
}

func TestVerCodeValidate(t *testing.T) {
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
			Name: "invalid_test_date",
			Code: VerificationCode{
				Code:        "123456",
				LongCode:    "123456",
				TestType:    "negative",
				SymptomDate: &oldTest,
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
		t.Run(tc.Name, func(t *testing.T) {
			if err := tc.Code.Validate(maxAge); err != tc.Err {
				t.Fatalf("wrong error, want %v, got: %v", tc.Err, err)
			}
		})
	}
}

func TestVerCodeIsExpired(t *testing.T) {
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
	db := NewTestDatabase(t)

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

	_, err := db.FindVerificationCode("12345678")
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatal(err)
	}
}

func TestPurgeVerificationCodes(t *testing.T) {
	t.Parallel()
	db := NewTestDatabase(t)

	now := time.Now()
	maxAge := time.Hour // not important to this test case

	testData := []*VerificationCode{
		{Code: "111111", LongCode: "111111", TestType: "negative", ExpiresAt: now.Add(time.Second), LongExpiresAt: now.Add(time.Second)},
		{Code: "222222", LongCode: "222222", TestType: "negative", ExpiresAt: now.Add(time.Second), LongExpiresAt: now.Add(time.Second)},
		{Code: "333333", LongCode: "333333ABCDEF", TestType: "negative", ExpiresAt: now.Add(time.Minute), LongExpiresAt: now.Add(time.Hour)},
	}
	for _, rec := range testData {
		if err := db.SaveVerificationCode(rec, maxAge); err != nil {
			t.Fatalf("can't save test data: %v", err)
		}
	}

	// Need to let some time lapse since we can't back date records through normal channels.
	time.Sleep(2 * time.Second)

	if count, err := db.PurgeVerificationCodes(time.Millisecond * 500); err != nil {
		t.Fatalf("error doing purge: %v", err)
	} else if count != 2 {
		t.Fatalf("purge record count mismatch, want: 2, got: %v", count)
	}
}
