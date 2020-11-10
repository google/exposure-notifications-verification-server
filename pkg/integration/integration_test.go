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

package integration

import (
	"context"
	"encoding/base64"
	"flag"
	"strings"
	"testing"
	"time"

	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/util"
	"github.com/google/exposure-notifications-server/pkg/verification"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/testsuite"
)

const (
	oneDay         = 24 * time.Hour
	intervalLength = 10 * time.Minute
	maxInterval    = 144
)

var (
	isE2E = flag.Bool("is_e2e", false, "Set to true when run as E2E tests.")
)

func TestIntegration(t *testing.T) {
	t.Parallel()
	cases := []struct {
		Name    string
		expire  bool
		ErrMsg  string
		SkipE2E bool
	}{
		{
			Name: "valid token",
		},
		{
			Name:    "expired token",
			expire:  true,
			ErrMsg:  "verification token expired",
			SkipE2E: true,
		},
	}

	ctx := context.Background()
	testSuite := testsuite.NewTestSuite(t, ctx, *isE2E)
	adminClient, err := testSuite.NewAdminAPIClient(ctx, t)
	if err != nil {
		t.Fatalf("failed to create admin API client, err: %v", err)
	}
	apiClient, err := testSuite.NewAPIClient(ctx, t)
	if err != nil {
		t.Fatalf("failed to create API client, err: %v", err)
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			if *isE2E && tc.SkipE2E {
				t.Skip("Skip in E2E test mode.")
			}

			now := time.Now().UTC()
			curDayInterval := timeToInterval(now)
			nextInterval := curDayInterval
			symptomDate := time.Now().UTC().Add(-48 * time.Hour).Format("2006-01-02")
			testType := "confirmed"
			tzMinOffset := 0

			teks := make([]verifyapi.ExposureKey, 14)
			for i := 0; i < len(teks); i++ {
				key, err := util.RandomExposureKey(nextInterval, maxInterval, 0)
				if err != nil {
					t.Fatalf("not enough entropy: %v", err)
				}
				teks[i] = key
				nextInterval -= maxInterval
			}

			hmacSecret := make([]byte, 32)
			hmacValue, err := verification.CalculateExposureKeyHMAC(teks, hmacSecret)
			if err != nil {
				t.Fatalf("error calculating tek HMAC: %v", err)
			}
			hmacB64 := base64.StdEncoding.EncodeToString(hmacValue)

			issueRequest := api.IssueCodeRequest{
				TestType:    testType,
				SymptomDate: symptomDate,
				TZOffset:    float32(tzMinOffset),
			}

			issueResp, err := adminClient.IssueCode(issueRequest)
			if issueResp == nil || err != nil {
				t.Fatalf("adminClient.IssueCode(%+v) = expected nil, got resp %+v, err %v", issueRequest, issueResp, err)
			}

			verifyRequest := api.VerifyCodeRequest{
				VerificationCode: issueResp.VerificationCode,
				AcceptTestTypes:  []string{testType},
			}

			verifyResp, err := apiClient.GetToken(verifyRequest)
			if err != nil {
				t.Fatalf("apiClient.GetToken(%+v) = expected nil, got resp %+v, err %v", verifyRequest, verifyResp, err)
			}

			if tc.expire {
				time.Sleep(config.VerificationTokenDuration)
			}

			certRequest := api.VerificationCertificateRequest{
				VerificationToken: verifyResp.VerificationToken,
				ExposureKeyHMAC:   hmacB64,
			}
			_, err = apiClient.GetCertificate(certRequest)
			if tc.expire {
				if err == nil || !strings.Contains(err.Error(), tc.ErrMsg) {
					t.Errorf("apiClient.GetCertificate(%+v) = expected %v, got err %v", certRequest, tc.ErrMsg, err)
				}
				return
			}

			if err != nil {
				t.Errorf("apiClient.GetCertificate(%+v) = expected nil, got err %v", certRequest, err)
			}
		})
	}
}

func timeToInterval(t time.Time) int32 {
	return int32(t.UTC().Truncate(oneDay).Unix() / int64(intervalLength.Seconds()))
}
