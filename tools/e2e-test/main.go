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

// Command line test that exercises the verification and key server,
// simulating a mobile device uploading TEKs.
//
//
package main

import (
	"context"
	"encoding/base64"
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-server/pkg/api/v1alpha1"
	"github.com/google/exposure-notifications-server/pkg/util"
	"github.com/google/exposure-notifications-server/pkg/verification"
	"github.com/google/exposure-notifications-verification-server/pkg/clients"
	"github.com/google/exposure-notifications-verification-server/pkg/jsonclient"
	"github.com/sethvargo/go-envconfig"
)

type Config struct {
	VerificationAdminAPIServer string `env:"VERIFICATION_ADMIN_API, default=http://localhost:8081"`
	VerificationAdminAPIKey    string `env:"VERIFICATION_ADMIN_API_KEY,required"`
	VerificationAPIServer      string `env:"VERIFICATION_SERVER_API, default=http://localhost:8082"`
	VerificationAPIServerKey   string `env:"VERIFICATION_SERVER_API_KEY,required"`
	KeyServer                  string `env:"KEY_SERVER, default=http://localhost:8080"`
	HealthAuthorityCode        string `env:"HEALTH_AUTHORITY_CODE,required"`

	// Publish config
	Region string `env:"REGION,default=US"`
}

const (
	timeout        = 2 * time.Second
	oneDay         = 24 * time.Hour
	intervalLength = 10 * time.Minute
	maxInterval    = 144
)

func timeToInterval(t time.Time) int32 {
	return int32(t.UTC().Truncate(oneDay).Unix() / int64(intervalLength.Seconds()))
}

func main() {
	ctx := context.Background()
	var config Config
	if err := envconfig.ProcessWith(ctx, &config, envconfig.OsLookuper()); err != nil {
		log.Fatalf("Unable to process environment: %v", err)
	}

	doRevision := flag.Bool("revise", false, "--revise means to do a likely diagnosis and then revise to confirmed. one new key is added in between.")
	verbose := flag.Bool("v", false, "ALL THE MESSAGES!")
	flag.Parse()

	reportType := "confirmed"
	iterations := 1
	if *doRevision {
		reportType = "likely"
		iterations++
	}
	symptomDate := time.Now().UTC().Add(-48 * time.Hour).Format("2006-01-02")
	revisionToken := ""

	now := time.Now().UTC()
	curDayInterval := timeToInterval(now)
	nextInterval := curDayInterval

	teks := make([]v1alpha1.ExposureKey, 14)
	for i := 0; i < len(teks); i++ {
		key, err := util.RandomExposureKey(nextInterval, maxInterval, 0)
		if err != nil {
			log.Fatalf("not enough entropy: %v", err)
		}
		teks[i] = key
		nextInterval -= maxInterval
	}

	for i := 0; i < iterations; i++ {
		// Issue the verification code.
		log.Printf("Issuing verification code")
		codeRequest, code, err := clients.IssueCode(config.VerificationAdminAPIServer, config.VerificationAdminAPIKey, reportType, symptomDate, timeout)
		if err != nil {
			log.Fatalf("Error issuing verification code: %v", err)
		} else if code.Error != "" {
			log.Fatalf("API Error: %+v", code)
		}
		if *verbose {
			log.Printf("Code Request: %+v", codeRequest)
			log.Printf("Code Response: %+v", code)
		}

		// Get the verification token
		log.Printf("Verifying code and getting token")
		tokenRequest, token, err := clients.GetToken(config.VerificationAPIServer, config.VerificationAPIServerKey, code.VerificationCode, timeout)
		if err != nil {
			log.Fatalf("Error verifying code: %v", err)
		} else if token.Error != "" {
			log.Fatalf("API Error %+v", token)
		}
		if *verbose {
			log.Printf("Token Request: %+v", tokenRequest)
			log.Printf("Token Response: %+v", token)
		}

		// Get the verification certificate
		log.Printf("Calculating HMAC and getting verification certificate")
		hmacSecret := make([]byte, 32)
		hmacValue, err := verification.CalculateExposureKeyHMAC(teks, hmacSecret)
		if err != nil {
			log.Fatalf("Error calculating tek HMAC: %v", err)
		}
		hmacB64 := base64.StdEncoding.EncodeToString(hmacValue)
		certRequest, certificate, err := clients.GetCertificate(config.VerificationAPIServer, config.VerificationAPIServerKey, token.VerificationToken, hmacB64, timeout)
		if err != nil {
			log.Fatalf("Error getting verification certificate: %v", err)
		} else if certificate.Error != "" {
			log.Fatalf("API Error: %+v", certificate)
		}
		if *verbose {
			log.Printf("Certificate Request: %+v", certRequest)
			log.Printf("Certificate Response: %+v", certificate)
		}

		// Upload the TEKs
		publish := v1alpha1.Publish{
			Keys:                teks,
			Regions:             []string{config.Region},
			AppPackageName:      config.HealthAuthorityCode,
			VerificationPayload: certificate.Certificate,
			HMACKey:             base64.StdEncoding.EncodeToString(hmacSecret),
			RevisionToken:       revisionToken,
		}

		// Make the publish request.
		log.Printf("Publish TEKs to the key server")
		var response v1alpha1.PublishResponse
		client := &http.Client{
			Timeout: timeout,
		}
		if *verbose {
			log.Printf("Publish request: %+v", publish)
		}
		if err := jsonclient.MakeRequest(client, config.KeyServer, http.Header{}, &publish, &response); err != nil {
			log.Fatalf("Error publishing teks: %v", err)
		} else if response.Error != "" {
			log.Fatalf("Publish API error: %+v", response)
		}
		log.Printf("Inserted %v exposures", response.InsertedExposures)
		if *verbose {
			log.Printf("Publish response: %+v", response)
		}

		if *doRevision {
			reportType = "confirmed"
			revisionToken = response.RevisionToken

			// Generate 1 more TEK
			key, err := util.RandomExposureKey(curDayInterval, maxInterval, 0)
			if err != nil {
				log.Fatalf("not enough entropy: %v", err)
			}
			teks = append(teks, key)
		}
	}
}
