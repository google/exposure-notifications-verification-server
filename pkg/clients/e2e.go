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

// E2E test code that exercises the verification and key server, simulating a
// mobile device uploading TEKs.
package clients

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/jsonclient"

	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-server/pkg/util"
	"github.com/google/exposure-notifications-server/pkg/verification"
)

const (
	timeout        = 2 * time.Second
	oneDay         = 24 * time.Hour
	intervalLength = 10 * time.Minute
	maxInterval    = 144
)

func timeToInterval(t time.Time) int32 {
	return int32(t.UTC().Truncate(oneDay).Unix() / int64(intervalLength.Seconds()))
}

func RunEndToEnd(ctx context.Context, config *config.E2ETestConfig) error {
	logger := logging.FromContext(ctx)

	testType := "confirmed"
	iterations := 1
	if config.DoRevise {
		testType = "likely"
		iterations++
	}
	symptomDate := time.Now().UTC().Add(-48 * time.Hour).Format("2006-01-02")
	revisionToken := ""

	now := time.Now().UTC()
	curDayInterval := timeToInterval(now)
	nextInterval := curDayInterval

	teks := make([]verifyapi.ExposureKey, 14)
	for i := 0; i < len(teks); i++ {
		key, err := util.RandomExposureKey(nextInterval, maxInterval, 0)
		if err != nil {
			return fmt.Errorf("not enough entropy: %w", err)
		}
		teks[i] = key
		nextInterval -= maxInterval
	}

	for i := 0; i < iterations; i++ {
		// Issue the verification code.
		logger.Infof("Issuing verification code, iteration %d", i)
		codeRequest, code, err := IssueCode(ctx, config.VerificationAdminAPIServer, config.VerificationAdminAPIKey, testType, symptomDate, 0, timeout)
		if err != nil {
			return fmt.Errorf("error issuing verification code: %w", err)
		} else if code.Error != "" {
			return fmt.Errorf("issue API Error: %+v", code)
		}

		logger.Debugw("Issue Code",
			"request", codeRequest,
			"response", code,
		)

		// Get the verification token
		logger.Infof("Verifying code and getting token")
		tokenRequest, token, err := GetToken(ctx, config.VerificationAPIServer, config.VerificationAPIServerKey, code.VerificationCode, timeout)
		if err != nil {
			return fmt.Errorf("error verifying code: %w", err)
		} else if token.Error != "" {
			return fmt.Errorf("verification API Error %+v", token)
		}
		logger.Debugw("getting token",
			"request", tokenRequest,
			"response", token,
		)

		logger.Infof("Check code status")
		statusReq, codeStatus, err := CheckCodeStatus(ctx, config.VerificationAdminAPIServer, config.VerificationAdminAPIKey, code.UUID, timeout)
		if err != nil {
			return fmt.Errorf("error check code status: %w", err)
		} else if codeStatus.Error != "" {
			return fmt.Errorf("check code status Error: %+v", codeStatus)
		}
		logger.Debugw("check code status",
			"request", statusReq,
			"response", codeStatus,
		)
		if !codeStatus.Claimed {
			return fmt.Errorf("expected claimed OTP code for %s", statusReq.UUID)
		}

		// Get the verification certificate
		logger.Infof("Calculating HMAC and getting verification certificate")
		hmacSecret := make([]byte, 32)
		hmacValue, err := verification.CalculateExposureKeyHMAC(teks, hmacSecret)
		if err != nil {
			return fmt.Errorf("error calculating tek HMAC: %w", err)
		}
		hmacB64 := base64.StdEncoding.EncodeToString(hmacValue)
		certRequest, certificate, err := GetCertificate(ctx, config.VerificationAPIServer, config.VerificationAPIServerKey, token.VerificationToken, hmacB64, timeout)
		if err != nil {
			return fmt.Errorf("error getting verification certificate: %w", err)
		} else if certificate.Error != "" {
			return fmt.Errorf("certificate API Error: %+v", certificate)
		}
		logger.Debugw("get certificate",
			"request", certRequest,
			"response", certificate,
		)

		// Upload the TEKs
		publish := verifyapi.Publish{
			Keys:                teks,
			HealthAuthorityID:   config.HealthAuthorityCode,
			VerificationPayload: certificate.Certificate,
			HMACKey:             base64.StdEncoding.EncodeToString(hmacSecret),
			RevisionToken:       revisionToken,
		}

		// Make the publish request.
		logger.Infof("Publish TEKs to the key server")
		var response verifyapi.PublishResponse
		client := &http.Client{
			Timeout: timeout,
		}
		logger.Debugw("publish",
			"request", publish,
		)
		if err := jsonclient.MakeRequest(ctx, client, config.KeyServer, http.Header{}, &publish, &response); err != nil {
			return fmt.Errorf("error publishing teks: %w", err)
		} else if response.ErrorMessage != "" {
			return fmt.Errorf("publish API error: %+v", response)
		}
		logger.Infof("Inserted %v exposures", response.InsertedExposures)
		logger.Debugw("publish",
			"response", response,
		)

		if config.DoRevise {
			testType = "confirmed"
			revisionToken = response.RevisionToken

			// Generate 1 more TEK
			key, err := util.RandomExposureKey(curDayInterval, maxInterval, 0)
			if err != nil {
				return fmt.Errorf("not enough entropy: %w", err)
			}
			teks = append(teks, key)
		}
	}

	return nil
}
