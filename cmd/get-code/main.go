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
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/clients"

	"github.com/google/exposure-notifications-server/pkg/logging"

	"github.com/sethvargo/go-signalcontext"
)

var (
	testFlag    = flag.String("type", "", "diagnosis test type: confirmed, likely, negative")
	onsetFlag   = flag.String("onset", "", "Symptom onset date, YYYY-MM-DD format")
	apikeyFlag  = flag.String("apikey", "", "API Key to use")
	addrFlag    = flag.String("addr", "http://localhost:8080", "protocol, address and port on which to make the API call")
	timeoutFlag = flag.Duration("timeout", 5*time.Second, "request time out duration in the format: 0h0m0s")
)

func main() {
	flag.Parse()

	ctx, done := signalcontext.OnInterrupt()

	debug, _ := strconv.ParseBool(os.Getenv("LOG_DEBUG"))
	logger := logging.NewLogger(debug)
	ctx = logging.WithLogger(ctx, logger)

	err := realMain(ctx)
	done()

	if err != nil {
		logger.Fatal(err)
	}
}

func realMain(ctx context.Context) error {
	logger := logging.FromContext(ctx)

	request, response, err := clients.IssueCode(ctx, *addrFlag, *apikeyFlag, *testFlag, *onsetFlag, *timeoutFlag)
	logger.Infow("sent request", "request", request)
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}
	logger.Infow("got response", "response", response)
	return nil
}
