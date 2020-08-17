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
			Name: "normal parse",
			Sub:  "confirmed.2020-07-07",
			Want: &Subject{
				TestType:    "confirmed",
				SymptomDate: &testDay,
			},
		},
		{
			Name: "no date",
			Sub:  "confirmed.",
			Want: &Subject{
				TestType:    "confirmed",
				SymptomDate: nil,
			},
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
	db := NewTestDatabase(t)

	codeAge := time.Hour
	symptomDate := time.Now().UTC().Truncate(24 * time.Hour)
	wrongSymptomDate := symptomDate.Add(-48 * time.Hour)

	acceptConfirmed := api.AcceptTypes{
		api.TestTypeConfirmed: struct{}{},
	}

	cases := []struct {
		Name         string
		Verification VerificationCode
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
			Verification: VerificationCode{
				Code:          "12345678",
				LongCode:      "12345678ABC",
				TestType:      "confirmed",
				SymptomDate:   &symptomDate,
				ExpiresAt:     time.Now().Add(time.Hour),
				LongExpiresAt: time.Now().Add(time.Hour),
			},
			Accept:   acceptConfirmed,
			Error:    "",
			TokenAge: time.Hour,
		},
		{
			Name: "long_code_token_issue",
			Verification: VerificationCode{
				Code:          "22332244",
				LongCode:      "abcd1234efgh5678",
				TestType:      "confirmed",
				SymptomDate:   &symptomDate,
				ExpiresAt:     time.Now().Add(5 * time.Second),
				LongExpiresAt: time.Now().Add(time.Hour),
			},
			Accept:      acceptConfirmed,
			UseLongCode: true,
			Error:       "",
			TokenAge:    time.Hour,
		},
		{
			Name: "already_claimed",
			Verification: VerificationCode{
				Code:          "00000001",
				LongCode:      "00000001ABC",
				Claimed:       true,
				TestType:      "confirmed",
				ExpiresAt:     time.Now().Add(time.Hour),
				LongExpiresAt: time.Now().Add(time.Hour),
			},
			Accept:   acceptConfirmed,
			Error:    ErrVerificationCodeUsed.Error(),
			TokenAge: time.Hour,
		},
		{
			Name: "code_expired",
			Verification: VerificationCode{
				Code:          "00000002",
				LongCode:      "00000002ABC",
				Claimed:       false,
				TestType:      "confirmed",
				ExpiresAt:     time.Now().Add(2 * time.Second),
				LongExpiresAt: time.Now().Add(2 * time.Second),
			},
			Accept:   acceptConfirmed,
			Delay:    2 * time.Second,
			Error:    ErrVerificationCodeExpired.Error(),
			TokenAge: time.Hour,
		},
		{
			Name: "token_expired",
			Verification: VerificationCode{
				Code:          "00000003",
				LongCode:      "00000003ABC",
				Claimed:       false,
				TestType:      "confirmed",
				ExpiresAt:     time.Now().Add(time.Hour),
				LongExpiresAt: time.Now().Add(time.Hour),
			},
			Accept:     acceptConfirmed,
			Delay:      time.Second,
			ClaimError: ErrTokenExpired.Error(),
			TokenAge:   time.Millisecond,
		},
		{
			Name: "wrong_test_type",
			Verification: VerificationCode{
				Code:          "00000005",
				LongCode:      "00000005ABC",
				Claimed:       false,
				TestType:      "confirmed",
				ExpiresAt:     time.Now().Add(time.Hour),
				LongExpiresAt: time.Now().Add(time.Hour),
			},
			Accept:     acceptConfirmed,
			ClaimError: ErrTokenMetadataMismatch.Error(),
			TokenAge:   time.Hour,
			Subject:    &Subject{"negative", nil},
		},
		{
			Name: "wrong_test_date",
			Verification: VerificationCode{
				Code:          "00000007",
				LongCode:      "00000007ABC",
				Claimed:       false,
				TestType:      "confirmed",
				SymptomDate:   &symptomDate,
				ExpiresAt:     time.Now().Add(time.Hour),
				LongExpiresAt: time.Now().Add(time.Hour),
			},
			Accept:     acceptConfirmed,
			ClaimError: ErrTokenMetadataMismatch.Error(),
			TokenAge:   time.Hour,
			Subject:    &Subject{"confirmed", &wrongSymptomDate},
		},
		{
			Name: "unsupported_test_type",
			Verification: VerificationCode{
				Code:          "00000008",
				LongCode:      "00000008ABC",
				Claimed:       false,
				TestType:      "likely",
				SymptomDate:   &symptomDate,
				ExpiresAt:     time.Now().Add(time.Hour),
				LongExpiresAt: time.Now().Add(time.Hour),
			},
			Accept: acceptConfirmed,
			Error:  ErrUnsupportedTestType.Error(),
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {

			realm, err := db.CreateRealm(fmt.Sprintf("test realm - %s", tc.Name))
			if err != nil {
				t.Fatalf("unable to create test realm")
			}

			tc.Verification.RealmID = realm.ID
			if err := db.SaveVerificationCode(&tc.Verification, codeAge); err != nil {
				t.Fatalf("error creating verification code: %v", err)
			}

			if tc.Delay > 0 {
				time.Sleep(tc.Delay)
			}

			code := tc.Verification.Code
			if tc.UseLongCode {
				code = tc.Verification.LongCode
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
				if tok.TestType != tc.Verification.TestType {
					t.Errorf("test type missmatch want: %v, got %v", tc.Verification.TestType, tok.TestType)
				}
				if tok.FormatSymptomDate() != tc.Verification.FormatSymptomDate() {
					t.Errorf("test date missmatch want: %v, got %v", tc.Verification.FormatSymptomDate(), tok.FormatSymptomDate())
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

				subject := &Subject{TestType: tc.Verification.TestType, SymptomDate: tc.Verification.SymptomDate}
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
