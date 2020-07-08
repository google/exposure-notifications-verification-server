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

	"github.com/google/go-cmp/cmp"
)

func TestSaveVerCode(t *testing.T) {
	t.Parallel()
	db := NewTestDatabase(t)

	maxAge := time.Hour
	code := VerificationCode{
		Code:      "12345678",
		TestType:  "confirmed",
		ExpiresAt: time.Now().Add(time.Hour),
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

	code.Claimed = true
	if err := db.SaveVerificationCode(&code, maxAge); err != nil {
		t.Fatalf("error claiming verification code: %v", err)
	}

	got, err = db.FindVerificationCode(code.Code)
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
			Name: "code too short",
			Code: VerificationCode{Code: "1"},
			Err:  ErrCodeTooShort,
		},
		{
			Name: "invalid test type",
			Code: VerificationCode{
				Code:     "123456",
				TestType: "self-reported",
			},
			Err: ErrInvalidTestType,
		},
		{
			Name: "invalid test date",
			Code: VerificationCode{
				Code:        "123456",
				TestType:    "negative",
				SymptomDate: &oldTest,
			},
			Err: ErrTestTooOld,
		},
		{
			Name: "already expired",
			Code: VerificationCode{
				Code:      "123456",
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

func TestPurgeVerificationCodes(t *testing.T) {
	t.Parallel()
	db := NewTestDatabase(t)

	now := time.Now()
	maxAge := time.Hour // not important to this test case

	testData := []*VerificationCode{
		{Code: "111111", TestType: "negative", ExpiresAt: now.Add(time.Second)},
		{Code: "222222", TestType: "negative", ExpiresAt: now.Add(time.Second)},
		{Code: "333333", TestType: "negative", ExpiresAt: now.Add(time.Minute)},
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
