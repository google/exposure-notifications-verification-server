package integration

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/util"
	"github.com/google/exposure-notifications-server/pkg/verification"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
)

const (
	oneDay         = 24 * time.Hour
	intervalLength = 10 * time.Minute
	maxInterval    = 144
)

func TestIntegration(t *testing.T) {
	cases := []struct {
		Name   string
		expire bool
		ErrMsg string
	}{
		{
			Name: "valid token",
		},
		{
			Name:   "expired token",
			expire: true,
			ErrMsg: "verification token expired",
		},
	}

	ctx := context.Background()
	suite := NewTestSuite(t, ctx)

	adminClient := suite.NewAdminAPIServer(ctx, t)
	apiClient := suite.NewAPIServer(ctx, t)

	for _, tc := range cases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

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
