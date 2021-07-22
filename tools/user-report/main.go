// Copyright 2021 the Exposure Notifications Verification Server authors
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

// Exchanges a verification code for a verification token.
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/clients"
	"github.com/google/exposure-notifications-verification-server/pkg/api"

	"github.com/google/exposure-notifications-server/pkg/logging"
)

var (
	nonceOnly   = flag.Bool("nonce-only", false, "just print out the nonce")
	nonceSize   = flag.Uint("nonce-size", 256, "size of the nonce to generate, in bytes")
	phoneNumber = flag.String("phone-number", "", "Phone number to send verification code to")
	testFlag    = flag.String("test-date", "", "Test date for code issue")
	onsetFlag   = flag.String("onset", "", "Symptom onset date, YYYY-MM-DD format")
	timeoutFlag = flag.Duration("timeout", 5*time.Second, "request time out duration in the format: 0h0m0s")
	apikeyFlag  = flag.String("apikey", "", "API Key to use")
	addrFlag    = flag.String("addr", "http://localhost:8080", "protocol, address and port on which to make the API call")
)

func main() {
	flag.Parse()

	ctx, done := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	if os.Getenv("LOG_LEVEL") == "" {
		os.Setenv("LOG_LEVEL", "DEBUG")
	}
	logger := logging.NewLoggerFromEnv().Named("user-report")
	ctx = logging.WithLogger(ctx, logger)

	err := realMain(ctx)
	done()

	if err != nil {
		logger.Fatal(err)
	}
}

func realMain(ctx context.Context) error {
	logger := logging.FromContext(ctx)

	nonce := make([]byte, *nonceSize)
	_, err := rand.Read(nonce)
	if err != nil {
		return err
	}

	if *nonceOnly {
		logger.Warnw("nonce only", "nonce", base64.URLEncoding.EncodeToString(nonce))
		return nil
	}

	client, err := clients.NewAPIServerClient(*addrFlag, *apikeyFlag,
		clients.WithTimeout(*timeoutFlag))
	if err != nil {
		return err
	}

	resp, err := client.UserReport(ctx, &api.UserReportRequest{
		TestDate:    *testFlag,
		SymptomDate: *onsetFlag,
		Phone:       *phoneNumber,
		Nonce:       base64.StdEncoding.EncodeToString(nonce),
	})
	if err != nil {
		return err
	}

	logger.Infow("success", "response", resp, "nonce", base64.StdEncoding.EncodeToString(nonce))
	return nil
}
