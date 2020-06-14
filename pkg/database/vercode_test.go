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

	code := VerificationCode{
		Code:      "12345678",
		TestType:  "confirmed",
		ExpiresAt: time.Now().Add(time.Hour),
	}

	if err := db.SaveVerificationCode(&code); err != nil {
		t.Fatalf("error creating verification code: %v", err)
	}

	got, err := db.FindVerificationCode(code.Code)
	if err != nil {
		t.Fatalf("error reading code from db: %v", err)
	}
	if diff := cmp.Diff(code, *got); diff != "" {
		t.Fatalf("mismatch (-want, +got):\n%s", diff)
	}

	code.Claimed = true
	if err := db.SaveVerificationCode(&code); err != nil {
		t.Fatalf("error claiming verification code: %v", err)
	}

	got, err = db.FindVerificationCode(code.Code)
	if err != nil {
		t.Fatalf("error reading code from db: %v", err)
	}
	if diff := cmp.Diff(code, *got); diff != "" {
		t.Fatalf("mismatch (-want, +got):\n%s", diff)
	}
}

func TestVerCodeValidate(t *testing.T) {
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
				Code:     "123456",
				TestType: "negative",
				TestDate: &oldTest,
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
			if err := tc.Code.Validate(); err != tc.Err {
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
