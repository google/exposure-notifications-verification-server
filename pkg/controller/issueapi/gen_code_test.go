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

package issueapi

import (
	"context"
	"testing"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/envstest/testconfig"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

func TestGenerateCode(t *testing.T) {
	t.Parallel()

	// Run through a whole bunch of iterations.
	for j := 0; j < 1000; j++ {
		code, err := generateCode(8)
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
		code, err := generateAlphanumericCode(16)
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

func TestIssue(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tc := testconfig.NewServerConfig(t, TestDatabaseInstance)
	db := tc.Database

	realm := database.NewRealmWithDefaults("Test Realm")
	ctx = controller.WithRealm(ctx, realm)
	if err := db.SaveRealm(realm, database.SystemTest); err != nil {
		t.Fatalf("failed to save realm: %v", err)
	}

	c := New(tc.Config, db, tc.RateLimiter, nil)

	numCodes := 100
	codes := make([]string, 0, numCodes)
	longCodes := make([]string, 0, numCodes)
	for i := 0; i < numCodes; i++ {
		vCode := &database.VerificationCode{
			ExpiresAt:     time.Now().Add(15 * time.Minute),
			LongExpiresAt: time.Now().Add(24 * time.Hour),
			TestType:      "confirmed",
		}
		if err := c.issue(ctx, vCode, realm, 10); err != nil {
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
		verCode, err := db.FindVerificationCode(code)
		if err != nil {
			t.Errorf("didn't find previously saved code")
		}
		if exp, codeType, err := db.IsCodeExpired(verCode, code); exp || err != nil {
			t.Fatalf("loaded code doesn't match requested code, %v %v", exp, err)
		} else if codeType != database.CodeTypeShort {
			t.Errorf("wrong code type, want: %v got: %v", database.CodeTypeShort, codeType)
		}
	}

	for _, code := range longCodes {
		verCode, err := db.FindVerificationCode(code)
		if err != nil {
			t.Errorf("didn't find previously saved code")
		}
		if exp, codeType, err := db.IsCodeExpired(verCode, code); exp || err != nil {
			t.Fatalf("loaded code doesn't match requested code")
		} else if codeType != database.CodeTypeLong {
			t.Errorf("wrong code type, want: %v got: %v", database.CodeTypeLong, codeType)
		}
	}
}
