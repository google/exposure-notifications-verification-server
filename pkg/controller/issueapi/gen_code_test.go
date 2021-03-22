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

package issueapi_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/issueapi"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

func TestGenerateCode(t *testing.T) {
	t.Parallel()

	// Run through a whole bunch of iterations.
	for j := 0; j < 1000; j++ {
		code, err := issueapi.GenerateCode(8)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got := len(code); got != 8 {
			t.Fatalf("code is wrong length want 8, got %v", got)
		}

		for i, c := range code {
			if c < '0' || c > '9' {
				t.Errorf("code[%v]: %v outside expected range 0-9", i, c)
			}
		}
	}
}

func TestGenerateAlphanumericCode(t *testing.T) {
	t.Parallel()

	// Run through a whole bunch of iterations.
	for j := 0; j < 1000; j++ {
		code, err := issueapi.GenerateAlphanumericCode(16)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got := len(code); got != 16 {
			t.Fatalf("code is wrong length want 16, got %v", got)
		}

		for i, c := range code {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'z')) {
				t.Errorf("code[%v]: %v outside expected range 0-9,a-z", i, c)
			}
		}
	}
}

func TestCommitCode(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServerConfig(t, testDatabaseInstance)
	db := harness.Database

	realm, err := db.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}
	ctx = controller.WithRealm(ctx, realm)

	c := issueapi.New(harness.Config, db, harness.RateLimiter, harness.KeyManager, harness.Renderer)

	numCodes := 100
	codes := make([]string, 0, numCodes)
	longCodes := make([]string, 0, numCodes)
	for i := 0; i < numCodes; i++ {
		vCode := &database.VerificationCode{
			ExpiresAt:     time.Now().Add(15 * time.Minute),
			LongExpiresAt: time.Now().Add(24 * time.Hour),
			TestType:      "confirmed",
		}
		if err := c.CommitCode(ctx, vCode, realm, 10); err != nil {
			t.Fatal(err)
		}
		if vCode.UUID == "" {
			t.Fatal("expected uuid from db, was empty")
		}
		codes = append(codes, vCode.Code)
		longCodes = append(longCodes, vCode.LongCode)
	}

	if got := len(codes); got != numCodes {
		t.Errorf("wrong number of codes, want: %v got %v", numCodes, got)
	}

	for _, code := range codes {
		VerCode, err := db.FindVerificationCode(code)
		if err != nil {
			t.Errorf("didn't find previously saved code")
		}
		if exp, codeType, err := db.IsCodeExpired(VerCode, code); exp || err != nil {
			t.Fatalf("loaded code doesn't match requested code, %v %v", exp, err)
		} else if codeType != database.CodeTypeShort {
			t.Errorf("wrong code type, want: %v got: %v", database.CodeTypeShort, codeType)
		}
	}

	for _, code := range longCodes {
		VerCode, err := db.FindVerificationCode(code)
		if err != nil {
			t.Errorf("didn't find previously saved code")
		}
		if exp, codeType, err := db.IsCodeExpired(VerCode, code); exp || err != nil {
			t.Fatalf("loaded code doesn't match requested code")
		} else if codeType != database.CodeTypeLong {
			t.Errorf("wrong code type, want: %v got: %v", database.CodeTypeLong, codeType)
		}
	}
}

func TestIssueCode(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	symptomDate := time.Now().UTC().Add(-48 * time.Hour)
	expires := time.Now().UTC().Add(48 * time.Hour)

	cases := []struct {
		name               string
		vCode              *database.VerificationCode
		enforceRealmQuotas bool
		responseErr        string
		httpStatusCode     int
	}{
		{
			name: "success",
			vCode: &database.VerificationCode{
				TestType:      "confirmed",
				SymptomDate:   &symptomDate,
				ExpiresAt:     expires,
				LongExpiresAt: expires,
			},
			httpStatusCode: http.StatusOK,
		},
		{
			name:  "db rejects",
			vCode: &database.VerificationCode{
				// type, date, and expiry are required
			},
			httpStatusCode: http.StatusInternalServerError,
		},
		{
			name: "rate limit exceeded",
			vCode: &database.VerificationCode{
				TestType:      "confirmed",
				SymptomDate:   &symptomDate,
				ExpiresAt:     expires,
				LongExpiresAt: expires,
			},
			enforceRealmQuotas: true,
			responseErr:        api.ErrQuotaExceeded,
			httpStatusCode:     http.StatusTooManyRequests,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			harness := envstest.NewServerConfig(t, testDatabaseInstance)
			db := harness.Database

			// Enable quota on the realm
			realm, err := db.FindRealm(1)
			if err != nil {
				t.Fatal(err)
			}
			realm.AbusePreventionEnabled = true
			if err := db.SaveRealm(realm, database.SystemTest); err != nil {
				t.Fatalf("failed to save realm: %v", err)
			}

			existingCode := &database.VerificationCode{
				RealmID:       realm.ID,
				Code:          "00000001",
				LongCode:      "00000001ABC",
				Claimed:       true,
				TestType:      "confirmed",
				ExpiresAt:     time.Now().Add(time.Hour),
				LongExpiresAt: time.Now().Add(time.Hour),
			}
			if err := db.SaveVerificationCode(existingCode, realm); err != nil {
				t.Fatal(err)
			}

			key, err := realm.QuotaKey(harness.Config.GetRateLimitConfig().HMACKey)
			if err != nil {
				t.Fatal(err)
			}
			if err := harness.RateLimiter.Set(ctx, key, 0, time.Hour); err != nil {
				t.Fatal(err)
			}

			c := issueapi.New(harness.Config, db, harness.RateLimiter, harness.KeyManager, harness.Renderer)

			harness.Config.Issue.EnforceRealmQuotas = tc.enforceRealmQuotas
			result := c.IssueCode(ctx, tc.vCode, realm)

			if tc.responseErr == "" {
				if tc.vCode.Code == "" {
					t.Fatal("Expected issued code.")
				}
				if tc.vCode.LongCode == "" {
					t.Fatal("Expected issued long code.")
				}
			} else {
				if result.HTTPCode != tc.httpStatusCode {
					t.Fatalf("incorrect error code. got %d, want %d", result.HTTPCode, tc.httpStatusCode)
				}
				if result.ErrorReturn.ErrorCode != tc.responseErr {
					t.Fatalf("did not receive expected errorCode. got %q, want %q", result.ErrorReturn.Error, tc.responseErr)
				}
			}
		})
	}
}
