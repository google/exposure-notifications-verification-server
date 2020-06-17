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
)

func TestIssueToken(t *testing.T) {
	t.Parallel()
	db := NewTestDatabase(t)

	maxAge := time.Hour

	cases := []struct {
		Name         string
		Verification VerificationCode
		Error        string
		Delay        time.Duration
	}{
		{
			Name: "normal token issue",
			Verification: VerificationCode{
				Code:      "12345678",
				TestType:  "confirmed",
				ExpiresAt: time.Now().Add(time.Hour),
			},
			Error: "",
		},
		{
			Name: "already claimed",
			Verification: VerificationCode{
				Code:      "ABC123",
				Claimed:   true,
				TestType:  "confirmed",
				ExpiresAt: time.Now().Add(time.Hour),
			},
			Error: ErrVerificationCodeUsed.Error(),
		},
		{
			Name: "expired",
			Verification: VerificationCode{
				Code:      "ABC123-2",
				Claimed:   false,
				TestType:  "confirmed",
				ExpiresAt: time.Now().Add(time.Millisecond * 250),
			},
			Delay: 250 * time.Millisecond,
			Error: ErrVerificationCodeExpired.Error(),
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {

			if err := db.SaveVerificationCode(&tc.Verification, maxAge); err != nil {
				t.Fatalf("error creating verification code: %v", err)
			}

			if tc.Delay > 0 {
				time.Sleep(tc.Delay)
			}

			tok, err := db.IssueToken(tc.Verification.Code, maxAge)
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
				if tok.TestDate != tc.Verification.TestDate {
					t.Errorf("test date missmatch want: %v, got %v", tc.Verification.TestType, tok.TestDate)
				}

				got, err := db.FindTokenById(tok.TokenID)
				if err != nil {
					t.Fatalf("error reading token from db: %v", err)
				}
				if diff := cmp.Diff(tok, got); diff != "" {
					t.Fatalf("mismatch (-want, +got):\n%s", diff)
				}
			}
		})
	}
}
