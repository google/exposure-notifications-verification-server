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
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/pkg/timeutils"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestSubject(t *testing.T) {
	testDay, err := time.Parse("2006-01-02", "2020-07-07")
	if err != nil {
		t.Fatalf("test setup error: %v", err)
	}

	cases := []struct {
		Name  string
		Sub   string
		Want  *Subject
		Error string
	}{
		{
			Name: "legacy_version",
			Sub:  "confirmed.2020-07-07",
			Want: &Subject{
				TestType:    "confirmed",
				SymptomDate: &testDay,
			},
		},
		{
			Name: "legacy_version_no_date",
			Sub:  "confirmed.",
			Want: &Subject{
				TestType:    "confirmed",
				SymptomDate: nil,
			},
		},
		{
			Name: "current_version_no_test_date",
			Sub:  "confirmed.2020-07-07.",
			Want: &Subject{
				TestType:    "confirmed",
				SymptomDate: &testDay,
			},
		},
		{
			Name: "current_version_no_symptom_date",
			Sub:  "confirmed..2020-07-07",
			Want: &Subject{
				TestType: "confirmed",
				TestDate: &testDay,
			},
		},
		{
			Name: "all_fields",
			Sub:  "confirmed.2020-07-07.2020-07-07",
			Want: &Subject{
				TestType:    "confirmed",
				SymptomDate: &testDay,
				TestDate:    &testDay,
			},
		},
		{
			Name:  "invalid_segments",
			Sub:   "confirmed",
			Want:  nil,
			Error: "subject must contain 2 or 3 parts, got: 1",
		},
		{
			Name:  "too_many_segments",
			Sub:   "confirmed.date.date.whomp",
			Want:  nil,
			Error: "subject must contain 2 or 3 parts, got: 4",
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			got, err := ParseSubject(tc.Sub)
			if err != nil {
				if tc.Error == "" {
					t.Fatalf("unexpected error: %v", err)
				} else if !strings.Contains(err.Error(), tc.Error) {
					t.Fatalf("wrong error, want: %v got: %v", tc.Error, err.Error())
				}
			} else if tc.Error != "" {
				t.Fatalf("wanted error: %v got: nil", tc.Error)
			}

			if diff := cmp.Diff(tc.Want, got, cmp.Options{cmpopts.EquateApproxTime(time.Second)}); diff != "" {
				t.Fatalf("mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestIssueToken(t *testing.T) {
	t.Parallel()

	codeAge := time.Hour
	symptomDate := timeutils.UTCMidnight(time.Now())
	wrongSymptomDate := symptomDate.Add(-48 * time.Hour)

	acceptConfirmed := api.AcceptTypes{
		api.TestTypeConfirmed: struct{}{},
	}

	cases := []struct {
		Name         string
		Verification func() *VerificationCode
		Accept       api.AcceptTypes
		UseLongCode  bool
		Error        string
		Delay        time.Duration
		TokenAge     time.Duration
		Subject      *Subject
		ClaimError   string
	}{
		{
			Name: "normal_token_issue",
			Verification: func() *VerificationCode {
				return &VerificationCode{
					Code:          "12345678",
					LongCode:      "12345678ABC",
					TestType:      "confirmed",
					SymptomDate:   &symptomDate,
					ExpiresAt:     time.Now().Add(time.Hour),
					LongExpiresAt: time.Now().Add(time.Hour),
				}
			},
			Accept:   acceptConfirmed,
			Error:    "",
			TokenAge: time.Hour,
		},
		{
			Name: "long_code_token_issue",
			Verification: func() *VerificationCode {
				return &VerificationCode{
					Code:          "22332244",
					LongCode:      "abcd1234efgh5678",
					TestType:      "confirmed",
					SymptomDate:   &symptomDate,
					ExpiresAt:     time.Now().Add(5 * time.Second),
					LongExpiresAt: time.Now().Add(time.Hour),
				}
			},
			Accept:      acceptConfirmed,
			UseLongCode: true,
			Error:       "",
			TokenAge:    time.Hour,
		},
		{
			Name: "already_claimed",
			Verification: func() *VerificationCode {
				return &VerificationCode{
					Code:          "00000001",
					LongCode:      "00000001ABC",
					Claimed:       true,
					TestType:      "confirmed",
					ExpiresAt:     time.Now().Add(time.Hour),
					LongExpiresAt: time.Now().Add(time.Hour),
				}
			},
			Accept:   acceptConfirmed,
			Error:    ErrVerificationCodeUsed.Error(),
			TokenAge: time.Hour,
		},
		{
			Name: "code_expired",
			Verification: func() *VerificationCode {
				return &VerificationCode{
					Code:          "00000002",
					LongCode:      "00000002ABC",
					Claimed:       false,
					TestType:      "confirmed",
					ExpiresAt:     time.Now().Add(1 * time.Second),
					LongExpiresAt: time.Now().Add(1 * time.Second),
				}
			},
			Accept:   acceptConfirmed,
			Delay:    2 * time.Second,
			Error:    ErrVerificationCodeExpired.Error(),
			TokenAge: time.Hour,
		},
		{
			Name: "token_expired",
			Verification: func() *VerificationCode {
				return &VerificationCode{
					Code:          "00000003",
					LongCode:      "00000003ABC",
					Claimed:       false,
					TestType:      "confirmed",
					ExpiresAt:     time.Now().Add(time.Hour),
					LongExpiresAt: time.Now().Add(time.Hour),
				}
			},
			Accept:     acceptConfirmed,
			Delay:      time.Second,
			ClaimError: ErrTokenExpired.Error(),
			TokenAge:   time.Millisecond,
		},
		{
			Name: "wrong_test_type",
			Verification: func() *VerificationCode {
				return &VerificationCode{
					Code:          "00000005",
					LongCode:      "00000005ABC",
					Claimed:       false,
					TestType:      "confirmed",
					ExpiresAt:     time.Now().Add(time.Hour),
					LongExpiresAt: time.Now().Add(time.Hour),
				}
			},
			Accept:     acceptConfirmed,
			ClaimError: ErrTokenMetadataMismatch.Error(),
			TokenAge:   time.Hour,
			Subject:    &Subject{"negative", nil, nil},
		},
		{
			Name: "wrong_test_date",
			Verification: func() *VerificationCode {
				return &VerificationCode{
					Code:          "00000007",
					LongCode:      "00000007ABC",
					Claimed:       false,
					TestType:      "confirmed",
					SymptomDate:   &symptomDate,
					ExpiresAt:     time.Now().Add(time.Hour),
					LongExpiresAt: time.Now().Add(time.Hour),
				}
			},
			Accept:     acceptConfirmed,
			ClaimError: ErrTokenMetadataMismatch.Error(),
			TokenAge:   time.Hour,
			Subject:    &Subject{"confirmed", &wrongSymptomDate, nil},
		},
		{
			Name: "unsupported_test_type",
			Verification: func() *VerificationCode {
				return &VerificationCode{
					Code:          "00000008",
					LongCode:      "00000008ABC",
					Claimed:       false,
					TestType:      "likely",
					SymptomDate:   &symptomDate,
					ExpiresAt:     time.Now().Add(time.Hour),
					LongExpiresAt: time.Now().Add(time.Hour),
				}
			},
			Accept: acceptConfirmed,
			Error:  ErrUnsupportedTestType.Error(),
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			db := NewTestDatabase(t)

			realm := NewRealmWithDefaults(fmt.Sprintf("TestIssueToken/%s", tc.Name))
			if err := db.SaveRealm(realm, System); err != nil {
				t.Fatal(err)
			}

			// Create the verification. We do this here instead of inside the test
			// struct to mitigate as much time drift as possible. It also ensures we
			// get a new VerificationCode on each invocation.
			verification := tc.Verification()
			verification.RealmID = realm.ID

			// Extract the code before saving. After saving, the code on the struct
			// will be the HMAC.
			code := verification.Code
			if tc.UseLongCode {
				code = verification.LongCode
			}

			if err := db.SaveVerificationCode(verification, codeAge); err != nil {
				t.Fatalf("error creating verification code: %v", err)
			}

			if tc.Delay > 0 {
				time.Sleep(tc.Delay)
			}

			tok, err := db.VerifyCodeAndIssueToken(realm.ID, code, tc.Accept, tc.TokenAge)
			if err != nil {
				if tc.Error == "" {
					t.Fatalf("error issuing token: %v", err)
				} else if !strings.Contains(err.Error(), tc.Error) {
					t.Fatalf("wrong error, want: '%v', got '%v'", tc.Error, err.Error())
				}
			} else if tc.Error != "" {
				t.Fatalf("missing error, want: '%v', got: nil", tc.Error)
			}

			if tc.Error == "" {
				if tok.TestType != verification.TestType {
					t.Errorf("test type missmatch want: %v, got %v", verification.TestType, tok.TestType)
				}
				if tok.FormatSymptomDate() != verification.FormatSymptomDate() {
					t.Errorf("test date missmatch want: %v, got %v", verification.FormatSymptomDate(), tok.FormatSymptomDate())
				}

				got, err := db.FindTokenByID(tok.TokenID)
				if err != nil {
					t.Fatalf("error reading token from db: %v", err)
				}

				if diff := cmp.Diff(tok, got, approxTime); diff != "" {
					t.Fatalf("mismatch (-want, +got):\n%s", diff)
				}

				if tc.Delay > 0 {
					time.Sleep(tc.Delay)
				}

				subject := &Subject{TestType: verification.TestType, SymptomDate: verification.SymptomDate}
				if tc.Subject != nil {
					subject = tc.Subject
				}
				if err != nil {
					t.Fatalf("unable to parse subject: %v", err)
				}
				if err := db.ClaimToken(realm.ID, got.TokenID, subject); err != nil && tc.ClaimError == "" {
					t.Fatalf("unexpected error claiming token: %v", err)
				} else if tc.ClaimError != "" {
					if err == nil {
						t.Fatalf("wanted error '%v' got: nil", tc.ClaimError)
					} else if !strings.Contains(err.Error(), tc.ClaimError) {
						t.Fatalf("wrong error, want: '%v', got '%v'", tc.ClaimError, err)
					}
				}

				if tc.ClaimError == "" {
					got, err = db.FindTokenByID(tok.TokenID)
					if err != nil {
						t.Fatalf("error reading token from db: %v", err)
					}
					if !got.Used {
						t.Fatalf("claimed token is not marked as used")
					}
				}
			}
		})
	}
}

func TestPurgeTokens(t *testing.T) {
	t.Parallel()
	db := NewTestDatabase(t)

	now := time.Now()
	testData := []*Token{
		{TokenID: "111111", TestType: "negative", ExpiresAt: now.Add(time.Second)},
		{TokenID: "222222", TestType: "negative", ExpiresAt: now.Add(time.Second)},
		{TokenID: "333333", TestType: "negative", ExpiresAt: now.Add(time.Minute)},
	}
	for _, rec := range testData {
		if err := db.db.Save(rec).Error; err != nil {
			t.Fatalf("can't save test data: %v", err)
		}
	}

	// Need to let some time lapse since we can't back date records through normal channels.
	time.Sleep(2 * time.Second)

	if count, err := db.PurgeTokens(time.Millisecond * 500); err != nil {
		t.Fatalf("error doing purge: %v", err)
	} else if count != 2 {
		t.Fatalf("purge record count mismatch, want: 2, got: %v", count)
	}
}
