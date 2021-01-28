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
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/pkg/timeutils"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jinzhu/gorm"
	"github.com/lib/pq"
)

func TestSubject(t *testing.T) {
	t.Parallel()

	testDay, err := time.Parse(project.RFC3339Date, "2020-07-07")
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
		tc := tc

		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

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

	symptomDate := timeutils.UTCMidnight(time.Now())
	wrongSymptomDate := symptomDate.Add(-48 * time.Hour)

	acceptConfirmed := api.AcceptTypes{
		api.TestTypeConfirmed: struct{}{},
	}

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	realm, err := db.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}

	authApp := &AuthorizedApp{
		RealmID: realm.ID,
		Name:    "Appy",
	}
	if _, err := realm.CreateAuthorizedApp(db, authApp, SystemTest); err != nil {
		t.Fatal(err)
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

			now := time.Now().UTC()

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

			if err := db.SaveVerificationCode(verification, realm); err != nil {
				t.Fatalf("error creating verification code: %v", err)
			}

			if tc.Delay > 0 {
				time.Sleep(tc.Delay)
			}

			tok, err := db.VerifyCodeAndIssueToken(now, authApp, code, tc.Accept, tc.TokenAge)
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

				if diff := cmp.Diff(tok, got, ApproxTime); diff != "" {
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
				if err := db.ClaimToken(now, authApp, got.TokenID, subject); err != nil && tc.ClaimError == "" {
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

func TestUpdateStatsAgeDistrib(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)
	app := &AuthorizedApp{
		RealmID: 1,
	}

	now := time.Now().UTC()
	nowStr := now.Format(project.RFC3339Date)

	cases := []struct {
		name          string
		codes         []*VerificationCode
		statDate      string
		expectAverage time.Duration
		expectBuckets pq.Int32Array
	}{
		{
			"test day old",
			[]*VerificationCode{
				{
					Model: gorm.Model{
						CreatedAt: now.Add(-24 * time.Hour),
					},
					Code: "111111",
				},
				{
					Model: gorm.Model{
						CreatedAt: now.Add(-48 * time.Hour),
					},
					Code: "222222",
				},
				{
					Model: gorm.Model{
						CreatedAt: now.Add(-4 * time.Hour),
					},
					Code: "333333",
				},
				{
					Model: gorm.Model{
						CreatedAt: now,
					},
					Code: "444444",
				},
			},
			nowStr,
			(24 + 48 + 4 + 0) * time.Hour / 4,
			pq.Int32Array([]int32{1, 0, 0, 0, 0, 0, 0, 1, 0, 1, 1}),
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			for _, code := range tc.codes {
				db.updateStatsAgeDistrib(now, app, code)
			}

			{
				var stats []*RealmStat
				if err := db.db.
					Model(&RealmStats{}).
					Select("*").
					Scan(&stats).
					Error; err != nil {
					if IsNotFound(err) {
						t.Fatalf("error grabbing realm stats %v", err)
					}
				}

				if len(stats) != 1 {
					t.Fatalf("expected one user stat")
				}
				stat := stats[0] // we're only expecting one

				if f := stat.Date.Format(project.RFC3339Date); f != tc.statDate {
					t.Errorf("expected stat.Date got = %s, expected %s", f, tc.statDate)
				}
				if stat.CodeClaimMeanAge.Duration != tc.expectAverage {
					t.Errorf("expected stat.CodeClaimMeanAge = %d, expected %d",
						stat.CodeClaimMeanAge.Duration, tc.expectAverage)
				}

				if !reflect.DeepEqual(stat.CodeClaimAgeDistribution, tc.expectBuckets) {
					t.Errorf("expected stat.CodeClaimAgeDistribution got = %v, expected %v", stat.CodeClaimAgeDistribution, tc.expectBuckets)
				}

				if int(stat.CodesClaimed) != len(tc.codes) {
					t.Errorf("expected stat.CodesClaimed got = %v, expected %v", stat.CodesClaimed, len(tc.codes))
				}
			}
		})
	}
}

func TestPurgeTokens(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

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
