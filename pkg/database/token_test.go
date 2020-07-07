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
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestIssueToken(t *testing.T) {
	t.Parallel()
	db := NewTestDatabase(t)

	codeAge := time.Hour
	testDate := time.Now().UTC().Truncate(24 * time.Hour)

	cases := []struct {
		Name         string
		Verification VerificationCode
		Error        string
		Delay        time.Duration
		TokenAge     time.Duration
		Subject      string
		ClaimError   string
	}{
		{
			Name: "normal token issue",
			Verification: VerificationCode{
				Code:      "12345678",
				TestType:  "confirmed",
				TestDate:  &testDate,
				ExpiresAt: time.Now().Add(time.Hour),
			},
			Error:    "",
			TokenAge: time.Hour,
		},
		{
			Name: "already claimed",
			Verification: VerificationCode{
				Code:      "00000001",
				Claimed:   true,
				TestType:  "confirmed",
				ExpiresAt: time.Now().Add(time.Hour),
			},
			Error:    ErrVerificationCodeUsed.Error(),
			TokenAge: time.Hour,
		},
		{
			Name: "code expired",
			Verification: VerificationCode{
				Code:      "00000002",
				Claimed:   false,
				TestType:  "confirmed",
				ExpiresAt: time.Now().Add(time.Second),
			},
			Delay:    time.Second,
			Error:    ErrVerificationCodeExpired.Error(),
			TokenAge: time.Hour,
		},
		{
			Name: "token expired",
			Verification: VerificationCode{
				Code:      "00000003",
				Claimed:   false,
				TestType:  "confirmed",
				ExpiresAt: time.Now().Add(time.Hour),
			},
			Delay:      time.Second,
			ClaimError: ErrTokenExpired.Error(),
			TokenAge:   time.Millisecond,
		},
		{
			Name: "invalid subject format",
			Verification: VerificationCode{
				Code:      "00000004",
				Claimed:   false,
				TestType:  "confirmed",
				ExpiresAt: time.Now().Add(time.Hour),
			},
			ClaimError: ErrTokenMetadataMismatch.Error(),
			TokenAge:   time.Hour,
			Subject:    "to.many.parts",
		},
		{
			Name: "wrong test type",
			Verification: VerificationCode{
				Code:      "00000005",
				Claimed:   false,
				TestType:  "confirmed",
				ExpiresAt: time.Now().Add(time.Hour),
			},
			ClaimError: ErrTokenMetadataMismatch.Error(),
			TokenAge:   time.Hour,
			Subject:    "negative.",
		},
		{
			Name: "unparsable test date",
			Verification: VerificationCode{
				Code:      "00000006",
				Claimed:   false,
				TestType:  "confirmed",
				TestDate:  &testDate,
				ExpiresAt: time.Now().Add(time.Hour),
			},
			ClaimError: ErrTokenMetadataMismatch.Error(),
			TokenAge:   time.Hour,
			Subject:    "confirmed.December 25, 2019",
		},
		{
			Name: "wrong test date",
			Verification: VerificationCode{
				Code:      "00000007",
				Claimed:   false,
				TestType:  "confirmed",
				TestDate:  &testDate,
				ExpiresAt: time.Now().Add(time.Hour),
			},
			ClaimError: ErrTokenMetadataMismatch.Error(),
			TokenAge:   time.Hour,
			Subject:    "confirmed." + testDate.Add(48*time.Hour).Format("2006-01-02"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {

			if err := db.SaveVerificationCode(&tc.Verification, codeAge); err != nil {
				t.Fatalf("error creating verification code: %v", err)
			}

			if tc.Delay > 0 {
				time.Sleep(tc.Delay)
			}

			tok, err := db.VerifyCodeAndIssueToken(tc.Verification.Code, tc.TokenAge)
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
				if tok.FormatTestDate() != tc.Verification.FormatTestDate() {
					t.Errorf("test date missmatch want: %v, got %v", tc.Verification.FormatTestDate(), tok.FormatTestDate())
				}

				got, err := db.FindTokenByID(tok.TokenID)
				if err != nil {
					t.Fatalf("error reading token from db: %v", err)
				}

				if diff := cmp.Diff(tok, got, cmp.Options{cmpopts.EquateApproxTime(time.Second)}); diff != "" {
					t.Fatalf("mismatch (-want, +got):\n%s", diff)
				}

				if tc.Delay > 0 {
					time.Sleep(tc.Delay)
				}

				subject := tc.Verification.TestType + "." + tc.Verification.FormatTestDate()
				if tc.Subject != "" {
					subject = tc.Subject
				}
				if err := db.ClaimToken(got.TokenID, subject); err != nil && tc.ClaimError == "" {
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
