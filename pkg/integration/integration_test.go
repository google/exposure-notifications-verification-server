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

// Package integration runs integration tests. These tests could be internal
// (all the servers are spun up in memory) or it could be via the e2e test which
// communicate across services deployed at distinct URLs.
package integration_test

import (
	"encoding/base64"
	"testing"
	"time"

	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/util"
	"github.com/google/exposure-notifications-server/pkg/verification"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

const (
	oneDay         = 24 * time.Hour
	intervalLength = 10 * time.Minute
	maxInterval    = 144
)

var testDatabaseInstance *database.TestInstance

func TestMain(m *testing.M) {
	testDatabaseInstance = database.MustTestInstance()
	defer testDatabaseInstance.MustClose()
	m.Run()
}

func TestIntegration(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	integrationSuite := envstest.NewIntegrationSuite(t, testDatabaseInstance)
	adminAPIClient := integrationSuite.AdminAPIServerClient()
	apiServerClient := integrationSuite.APIServerClient()

	now := time.Now().UTC()
	curDayInterval := timeToInterval(now)
	nextInterval := curDayInterval
	symptomDate := time.Now().UTC().Add(-48 * time.Hour).Format(project.RFC3339Date)
	testType := "confirmed"

	// Test issuing a single code
	t.Run("issue", func(t *testing.T) {
		t.Parallel()

		// Generate HMAC
		tekHMAC := generateHMAC(t, nextInterval)

		// Issue a code
		issueReq := &api.IssueCodeRequest{
			TestType:    testType,
			SymptomDate: symptomDate,
		}
		if !project.SkipE2ESMS {
			issueReq.Phone = project.TestPhoneNumber
		}
		issueResp, err := adminAPIClient.IssueCode(ctx, issueReq)
		if err != nil {
			t.Fatalf("failed to issue code: %#v\n  req: %#v\n  resp: %#v", err, issueReq, issueResp)
		}
		if len(issueResp.Padding) == 0 {
			t.Error("expected response padding")
		}

		// Invalid code should fail to verify
		{
			verifyReq := &api.VerifyCodeRequest{
				VerificationCode: "NOT-A-REAL-CODE",
				AcceptTestTypes:  []string{testType},
			}
			verifyResp, err := apiServerClient.Verify(ctx, verifyReq)
			if err == nil {
				t.Fatalf("expected code to fail to verify (invalid), got no error\n  req: %#v\n  resp: %#v", verifyReq, verifyResp)
			}
		}

		// Valid code verifies
		verifyReq := &api.VerifyCodeRequest{
			VerificationCode: issueResp.VerificationCode,
			AcceptTestTypes:  []string{testType},
		}
		verifyResp, err := apiServerClient.Verify(ctx, verifyReq)
		if err != nil {
			t.Fatalf("failed to verify code: %#v\n  req: %#v\n  resp: %#v", err, verifyReq, verifyResp)
		}

		// Valid code verifies only once
		{
			verifyReq := &api.VerifyCodeRequest{
				VerificationCode: issueResp.VerificationCode,
				AcceptTestTypes:  []string{testType},
			}
			verifyResp, err := apiServerClient.Verify(ctx, verifyReq)
			if err == nil {
				t.Fatalf("expected code to fail to verify (already used), got no error\n  req: %#v\n  resp: %#v", verifyReq, verifyResp)
			}
		}

		// Invalid token should fail to issue a certificate
		{
			certReq := &api.VerificationCertificateRequest{
				VerificationToken: "STILL-TOTALLY-NOT-REAL",
				ExposureKeyHMAC:   tekHMAC,
			}
			certResp, err := apiServerClient.Certificate(ctx, certReq)
			if err == nil {
				t.Fatalf("expected certificate to failed to issue (invalid): got no error\n  req: %#v\n  resp: %#v", certReq, certResp)
			}
		}

		// Get a certificate
		certReq := &api.VerificationCertificateRequest{
			VerificationToken: verifyResp.VerificationToken,
			ExposureKeyHMAC:   tekHMAC,
		}
		certResp, err := apiServerClient.Certificate(ctx, certReq)
		if err != nil {
			t.Fatalf("failed to get certificate: %#v\n  req: %#v\n  resp: %#v", err, certReq, certResp)
		}

		// Valid token issues only once
		{
			certReq := &api.VerificationCertificateRequest{
				VerificationToken: verifyResp.VerificationToken,
				ExposureKeyHMAC:   tekHMAC,
			}
			certResp, err := apiServerClient.Certificate(ctx, certReq)
			if err == nil {
				t.Fatalf("expected certificate to failed to issue (already used): got no error\n  req: %#v\n  resp: %#v", certReq, certResp)
			}
		}
	})

	t.Run("issue_batch", func(t *testing.T) {
		t.Parallel()

		// Generate HMAC
		tekHMAC := generateHMAC(t, nextInterval)

		// Issue 2 codes
		issueReq := &api.BatchIssueCodeRequest{
			Codes: []*api.IssueCodeRequest{
				{
					TestType:    testType,
					SymptomDate: symptomDate,
				},
				{
					TestType:    testType,
					SymptomDate: symptomDate,
				},
			},
		}
		outerIssueResp, err := adminAPIClient.BatchIssueCode(ctx, issueReq)
		if err != nil {
			t.Fatalf("failed to issue code: %#v\n  req: %#v\n  resp: %#v", err, issueReq, outerIssueResp)
		}
		if len(outerIssueResp.Padding) == 0 {
			t.Error("expected response padding")
		}

		// Invalid code should fail to verify
		{
			verifyReq := &api.VerifyCodeRequest{
				VerificationCode: "NOT-A-REAL-CODE",
				AcceptTestTypes:  []string{testType},
			}
			verifyResp, err := apiServerClient.Verify(ctx, verifyReq)
			if err == nil {
				t.Fatalf("expected code to fail to verify (invalid), got no error\n  req: %#v\n  resp: %#v", verifyReq, verifyResp)
			}
		}

		// Verify all codes in batch.
		for _, issueResp := range outerIssueResp.Codes {
			if len(issueResp.Padding) != 0 {
				t.Errorf("batch does not expect inner response padding, got %s", string(issueResp.Padding))
			}

			verifyReq := &api.VerifyCodeRequest{
				VerificationCode: issueResp.VerificationCode,
				AcceptTestTypes:  []string{testType},
			}
			verifyResp, err := apiServerClient.Verify(ctx, verifyReq)
			if err != nil {
				t.Fatalf("apiClient.GetToken(%+v) = expected nil, got resp %+v, err %v", verifyReq, verifyResp, err)
			}
			if err != nil {
				t.Fatalf("failed to verify code: %#v\n  req: %#v\n  resp: %#v", err, verifyReq, verifyResp)
			}

			// Valid code verifies only once
			{
				verifyReq := &api.VerifyCodeRequest{
					VerificationCode: issueResp.VerificationCode,
					AcceptTestTypes:  []string{testType},
				}
				verifyResp, err := apiServerClient.Verify(ctx, verifyReq)
				if err == nil {
					t.Fatalf("expected code to fail to verify (already used), got no error\n  req: %#v\n  resp: %#v", verifyReq, verifyResp)
				}
			}

			// Invalid token should fail to issue a certificate
			{
				certReq := &api.VerificationCertificateRequest{
					VerificationToken: "STILL-TOTALLY-NOT-REAL",
					ExposureKeyHMAC:   tekHMAC,
				}
				certResp, err := apiServerClient.Certificate(ctx, certReq)
				if err == nil {
					t.Fatalf("expected certificate to failed to issue (invalid): got no error\n  req: %#v\n  resp: %#v", certReq, certResp)
				}
			}

			certReq := &api.VerificationCertificateRequest{
				VerificationToken: verifyResp.VerificationToken,
				ExposureKeyHMAC:   tekHMAC,
			}
			certResp, err := apiServerClient.Certificate(ctx, certReq)
			if err != nil {
				t.Fatalf("failed to get certificate: %#v\n  req: %#v\n  resp: %#v", err, certReq, certResp)
			}

			// Valid token issues only once
			{
				certReq := &api.VerificationCertificateRequest{
					VerificationToken: verifyResp.VerificationToken,
					ExposureKeyHMAC:   tekHMAC,
				}
				certResp, err := apiServerClient.Certificate(ctx, certReq)
				if err == nil {
					t.Fatalf("expected certificate to failed to issue (already used): got no error\n  req: %#v\n  resp: %#v", certReq, certResp)
				}
			}
		}
	})
}

// generateHMAC generates an HMAC of TEKs.
func generateHMAC(tb testing.TB, nextInterval int32) string {
	teks := make([]verifyapi.ExposureKey, 14)
	for i := 0; i < len(teks); i++ {
		key, err := util.RandomExposureKey(nextInterval, maxInterval, 0)
		if err != nil {
			tb.Fatal(err)
		}
		teks[i] = key
		nextInterval -= maxInterval
	}

	hmacSecret, err := project.RandomBytes(32)
	if err != nil {
		tb.Fatal(err)
	}
	hmacValue, err := verification.CalculateExposureKeyHMAC(teks, hmacSecret)
	if err != nil {
		tb.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(hmacValue)
}

func timeToInterval(t time.Time) int32 {
	return int32(t.UTC().Truncate(oneDay).Unix() / int64(intervalLength.Seconds()))
}
