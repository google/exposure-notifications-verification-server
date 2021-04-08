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

// Does the two step call to the webview to initiate a user report.
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"net/http/cookiejar"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/internal/clients"
	"golang.org/x/net/publicsuffix"
)

var (
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
	logger := logging.NewLoggerFromEnv().Named("user-report-web")
	ctx = logging.WithLogger(ctx, logger)

	err := realMain(ctx)
	done()

	if err != nil {
		logger.Fatal(err)
	}
}

func realMain(ctx context.Context) error {
	logger := logging.FromContext(ctx)

	nonceBytes := make([]byte, *nonceSize)
	_, err := rand.Read(nonceBytes)
	if err != nil {
		return err
	}
	nonce := base64.URLEncoding.EncodeToString(nonceBytes)

	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return err
	}

	client, err := clients.NewENXRedirectClient(*addrFlag,
		clients.WithCookieJar(jar),
		clients.WithTimeout(*timeoutFlag))
	if err != nil {
		return fmt.Errorf("unable to create client: %w", err)
	}

	if err := client.SendUserReportIndex(ctx, *apikeyFlag, nonce); err != nil {
		return err
	}
	logger.Debugw("session established")

	if err := client.SendUserReportIssue(ctx, *testFlag, *onsetFlag, *phoneNumber, "true"); err != nil {
		return err
	}

	logger.Infow("code issued", "nonce", nonce)

	return nil
}
