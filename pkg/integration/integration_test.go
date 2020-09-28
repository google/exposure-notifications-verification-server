package integration

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/util"
	"github.com/google/exposure-notifications-server/pkg/verification"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
)

const (
	oneDay         = 24 * time.Hour
	intervalLength = 10 * time.Minute
	maxInterval    = 144
)

func TestIntegration(t *testing.T) {
	ctx := context.Background()
	suite := NewTestSuite(t, ctx)

	adminClient := suite.NewAdminAPIServer(ctx, t)
	apiClient := suite.NewAPIServer(ctx, t)

	testType := "confirmed"
	symptomDate := time.Now().UTC().Add(-48 * time.Hour).Format("2006-01-02")
	tzMinOffset := 0

	now := time.Now().UTC()
	curDayInterval := timeToInterval(now)
	nextInterval := curDayInterval

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
	if issueResp == nil || err != nil && issueResp.ErrorCode != "" {
		t.Fatalf("expected nil, got resp %+v, err %v", issueResp, err)
	}

	verifyRequest := api.VerifyCodeRequest{
		VerificationCode: issueResp.VerificationCode,
		AcceptTestTypes:  []string{"confirmed"},
	}

	verifyResp, err := apiClient.GetToken(verifyRequest)
	if err != nil || verifyResp.ErrorCode != "" {
		t.Errorf("expected nil, got resp %+v, err %v", verifyResp, err)
	}

	certRequest := api.VerificationCertificateRequest{
		VerificationToken: verifyResp.VerificationToken,
		ExposureKeyHMAC:   hmacB64,
	}
	certResp, err := apiClient.GetCertificate(certRequest)
	if err != nil || certResp.ErrorCode != "" {
		t.Errorf("expected nil, got resp %+v, err %v", verifyResp, err)
	}
	t.Logf("resp: %+v", certResp)
}

func timeToInterval(t time.Time) int32 {
	return int32(t.UTC().Truncate(oneDay).Unix() / int64(intervalLength.Seconds()))
}
