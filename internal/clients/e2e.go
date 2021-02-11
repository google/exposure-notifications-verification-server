// Copyright 2021 Google LLC
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

package clients

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/logging"
	enobs "github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-server/pkg/util"
	"github.com/google/exposure-notifications-server/pkg/verification"

	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"

	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

const (
	timeout        = 60 * time.Second
	oneDay         = 24 * time.Hour
	intervalLength = 10 * time.Minute
	maxInterval    = 144
)

const metricPrefix = observability.MetricRoot + "/e2e"

var (
	mLatencyMs = stats.Float64(metricPrefix+"/request", "request latency", stats.UnitMilliseconds)

	// The name of step in e2e test.
	stepTagKey = tag.MustNewKey("step")

	// The type of the e2e test.
	testTypeTagKey = tag.MustNewKey("test_type")
)

func init() {
	enobs.CollectViews([]*view.View{
		{
			Name:        metricPrefix + "/request_count",
			Measure:     mLatencyMs,
			Description: "Count of e2e requests",
			TagKeys:     append(observability.CommonTagKeys(), enobs.ResultTagKey, stepTagKey, testTypeTagKey),
			Aggregation: view.Count(),
		},
		{
			Name:        metricPrefix + "/request_latency",
			Measure:     mLatencyMs,
			Description: "Distribution of e2e requests latency in ms",
			TagKeys:     append(observability.CommonTagKeys(), stepTagKey, testTypeTagKey),
			Aggregation: ochttp.DefaultLatencyDistribution,
		},
	}...)
}

// RunEndToEnd - code that exercises the verification and key server, simulating a
// mobile device uploading TEKs.
func RunEndToEnd(ctx context.Context, cfg *config.E2ERunnerConfig) error {
	logger := logging.FromContext(ctx)

	adminAPIClient, err := NewAdminAPIServerClient(cfg.VerificationAdminAPIServer, cfg.VerificationAdminAPIKey,
		WithTimeout(timeout))
	if err != nil {
		return err
	}

	apiServerClient, err := NewAPIServerClient(cfg.VerificationAPIServer, cfg.VerificationAPIServerKey,
		WithTimeout(timeout))
	if err != nil {
		return err
	}

	testType := "confirmed"
	iterations := 1
	if cfg.DoRevise {
		testType = "likely"
		iterations++
	}
	symptomDate := time.Now().UTC().Add(-48 * time.Hour).Format(project.RFC3339Date)
	adminID := ""
	revisionToken := ""

	now := time.Now().UTC()
	curDayInterval := timeToInterval(now)
	nextInterval := curDayInterval

	teks := make([]verifyapi.ExposureKey, 0, 14)
	for i := 0; i < cap(teks); i++ {
		key, err := util.RandomExposureKey(nextInterval, maxInterval, 0)
		if err != nil {
			return fmt.Errorf("not enough entropy: %w", err)
		}
		teks[i] = key
		nextInterval -= maxInterval
	}

	result := enobs.ResultOK
	// Parameterize enobs.RecordLatency()
	recordLatency := func(ctx context.Context, start time.Time, step string) {
		stepMutator := tag.Upsert(stepTagKey, step)
		enobs.RecordLatency(ctx, start, mLatencyMs, &stepMutator, &result)
	}

	for i := 0; i < iterations; i++ {
		logger.Infof("Issuing verification code, iteration %d", i)

		ctx, err := tag.New(ctx, tag.Upsert(testTypeTagKey, testType))
		if err != nil {
			return fmt.Errorf("unable to create new context with additional tags: %w", err)
		}
		code, err := func() (*api.IssueCodeResponse, error) {
			defer recordLatency(ctx, time.Now(), "/api/issue")

			// Issue the verification code.
			codeReq := &api.IssueCodeRequest{
				TestType:         testType,
				SymptomDate:      symptomDate,
				TZOffset:         0,
				ExternalIssuerID: adminID,
			}

			codeResp, err := adminAPIClient.IssueCode(ctx, codeReq)
			defer logger.Debugw("issuing code", "request", codeReq, "response", codeResp)
			if err != nil {
				result = enobs.ResultNotOK
				return nil, fmt.Errorf("error issuing verification code: %w", err)
			} else if codeResp.Error != "" {
				result = enobs.ResultNotOK
				return nil, fmt.Errorf("issue API Error: %+v", codeResp)
			}
			return codeResp, nil
		}()
		if err != nil {
			return err
		}

		token, err := func() (*api.VerifyCodeResponse, error) {
			defer recordLatency(ctx, time.Now(), "/api/verify")
			// Get the verification token
			logger.Infof("Verifying code and getting token")
			tokenReq := &api.VerifyCodeRequest{
				VerificationCode: code.VerificationCode,
			}
			tokenResp, err := apiServerClient.Verify(ctx, tokenReq)
			logger.Debugw("verifying code", "request", tokenReq, "response", tokenResp)
			if err != nil {
				result = enobs.ResultNotOK
				return nil, fmt.Errorf("error verifying code: %w", err)
			} else if tokenResp.Error != "" {
				result = enobs.ResultNotOK
				return nil, fmt.Errorf("verification API Error %+v", tokenResp)
			}
			return tokenResp, nil
		}()
		if err != nil {
			return err
		}

		if err := func() error {
			defer recordLatency(ctx, time.Now(), "/api/verify")
			logger.Infof("Check code status")
			statusReq := &api.CheckCodeStatusRequest{
				UUID: code.UUID,
			}
			statusResp, err := adminAPIClient.CheckCodeStatus(ctx, statusReq)
			logger.Debugw("check code status", "request", statusReq, "response", statusResp)
			if err != nil {
				result = enobs.ResultNotOK
				return fmt.Errorf("error check code status: %w", err)
			} else if statusResp.Error != "" {
				result = enobs.ResultNotOK
				return fmt.Errorf("check code status Error: %+v", statusResp)
			}
			if !statusResp.Claimed {
				result = enobs.ResultNotOK
				return fmt.Errorf("expected claimed OTP code for %s", statusReq.UUID)
			}
			return nil
		}(); err != nil {
			return err
		}

		hmacSecret, hmacB64, err := func() ([]byte, string, error) {
			defer recordLatency(ctx, time.Now(), "hmac")
			logger.Infof("Calculating HMAC")
			hmacSecret := make([]byte, 32)
			if _, err := rand.Read(hmacSecret); err != nil {
				return nil, "", fmt.Errorf("error generating hmac secret")
			}
			hmacValue, err := verification.CalculateExposureKeyHMAC(teks, hmacSecret)
			if err != nil {
				result = enobs.ResultNotOK
				return nil, "", fmt.Errorf("error calculating tek HMAC: %w", err)
			}
			return hmacSecret, base64.StdEncoding.EncodeToString(hmacValue), nil
		}()
		if err != nil {
			return err
		}

		certificate, err := func() (*api.VerificationCertificateResponse, error) {
			defer recordLatency(ctx, time.Now(), "/api/certificate")
			logger.Infof("Getting verification certificate")
			certReq := &api.VerificationCertificateRequest{
				VerificationToken: token.VerificationToken,
				ExposureKeyHMAC:   hmacB64,
			}
			certResp, err := apiServerClient.Certificate(ctx, certReq)
			logger.Debugw("get certificate", "request", certReq, "response", certResp)
			if err != nil {
				result = enobs.ResultNotOK
				return nil, fmt.Errorf("error getting verification certificate: %w", err)
			} else if certResp.Error != "" {
				result = enobs.ResultNotOK
				return nil, fmt.Errorf("certificate API Error: %+v", certResp)
			}
			return certResp, nil
		}()
		if err != nil {
			return err
		}

		// Upload the TEKs
		response, err := func() (*verifyapi.PublishResponse, error) {
			defer recordLatency(ctx, time.Now(), "upload_to_key_server")
			// Make the publish request.
			logger.Infof("Publish TEKs to the key server")
			publishReq := &verifyapi.Publish{
				Keys:                teks,
				HealthAuthorityID:   cfg.HealthAuthorityCode,
				VerificationPayload: certificate.Certificate,
				HMACKey:             base64.StdEncoding.EncodeToString(hmacSecret),
				RevisionToken:       revisionToken,
			}

			client := &http.Client{
				Timeout: timeout,
			}

			var b bytes.Buffer
			if err := json.NewEncoder(&b).Encode(publishReq); err != nil {
				return nil, err
			}

			httpReq, err := http.NewRequestWithContext(ctx, "POST", cfg.KeyServer, &b)
			if err != nil {
				return nil, err
			}
			httpReq.Header.Set("Content-Type", "application/json")

			httpResp, err := client.Do(httpReq)
			if err != nil {
				result = enobs.ResultNotOK
				return nil, fmt.Errorf("error making request to publish teks: %w", err)
			}
			defer httpResp.Body.Close()

			var publishResp verifyapi.PublishResponse
			if err := json.NewDecoder(httpResp.Body).Decode(&publishResp); err != nil {
				return nil, err
			}
			defer logger.Debugw("publish", "request", publishReq, "response", publishResp)
			if publishResp.ErrorMessage != "" {
				result = enobs.ResultNotOK
				logger.Infow("failed to publish teks", "error", err, "keys", teks)
				return nil, fmt.Errorf("publish API error: %+v", publishResp)
			}
			logger.Infof("Inserted %v exposures", publishResp.InsertedExposures)
			return &publishResp, nil
		}()
		if err != nil {
			return err
		}

		if cfg.DoRevise {
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

func timeToInterval(t time.Time) int32 {
	return int32(t.UTC().Truncate(oneDay).Unix() / int64(intervalLength.Seconds()))
}
