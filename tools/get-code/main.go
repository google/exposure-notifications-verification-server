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

// Exchanges a verification code for a verification token.
package main

import (
	"context"
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
	testFlag     = flag.String("type", "", "diagnosis test type: confirmed, likely, negative")
	onsetFlag    = flag.String("onset", "", "Symptom onset date, YYYY-MM-DD format")
	timeoutFlag  = flag.Duration("timeout", 5*time.Second, "request time out duration in the format: 0h0m0s")
	tzOffsetFlag = flag.Int("tzOffset", 0, "timezone adjustment (minutes) from UTC for request")
	apikeyFlag   = flag.String("apikey", "", "API Key to use")
	adminIDFlag  = flag.String("adminID", "", "AdminID for statistics tracking")
	addrFlag     = flag.String("addr", "http://localhost:8080", "protocol, address and port on which to make the API call")
	phoneFlag    = flag.String("phone-number", "", "If provided, phone number to send SMS to")
)

func main() {
	flag.Parse()

	ctx, done := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	if os.Getenv("LOG_LEVEL") == "" {
		os.Setenv("LOG_LEVEL", "DEBUG")
	}
	logger := logging.NewLoggerFromEnv().Named("get-code")
	ctx = logging.WithLogger(ctx, logger)

	err := realMain(ctx)
	done()

	if err != nil {
		logger.Fatal(err)
	}
}

func realMain(ctx context.Context) error {
	logger := logging.FromContext(ctx)

	client, err := clients.NewAdminAPIServerClient(*addrFlag, *apikeyFlag,
		clients.WithTimeout(*timeoutFlag))
	if err != nil {
		return err
	}

	resp, err := client.IssueCode(ctx, &api.IssueCodeRequest{
		TestType:         *testFlag,
		SymptomDate:      *onsetFlag,
		TZOffset:         float32(*tzOffsetFlag),
		ExternalIssuerID: *adminIDFlag,
		Phone:            *phoneFlag,
	})
	if err != nil {
		return err
	}

	logger.Infow("success", "response", resp)
	return nil
}
