// Copyright 2020 the Exposure Notifications Verification Server authors
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
	userAgent   = flag.String("user-agent", "", "if present, will set as the user agent on the HTTP request")
	codeFlag    = flag.String("code", "", "verification code to exchange")
	apikeyFlag  = flag.String("apikey", "", "API Key to use")
	addrFlag    = flag.String("addr", "http://localhost:8080", "protocol, address and port on which to make the API call")
	nonceFlag   = flag.String("nonce", "", "optional, nonce to pass on verify call, base64 encoded")
	timeoutFlag = flag.Duration("timeout", 5*time.Second, "request time out duration in the format: 0h0m0s")
)

func main() {
	flag.Parse()

	ctx, done := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	if os.Getenv("LOG_LEVEL") == "" {
		os.Setenv("LOG_LEVEL", "DEBUG")
	}
	logger := logging.NewLoggerFromEnv().Named("get-token")
	ctx = logging.WithLogger(ctx, logger)

	err := realMain(ctx)
	done()

	if err != nil {
		logger.Fatal(err)
	}
}

func realMain(ctx context.Context) error {
	logger := logging.FromContext(ctx)

	opts := make([]clients.Option, 0, 2)
	opts = append(opts, clients.WithTimeout(*timeoutFlag))
	if ua := *userAgent; ua != "" {
		opts = append(opts, clients.WithUserAgent(ua))
	}

	client, err := clients.NewAPIServerClient(*addrFlag, *apikeyFlag, opts...)
	if err != nil {
		return err
	}

	request := &api.VerifyCodeRequest{
		VerificationCode: *codeFlag,
		AcceptTestTypes:  []string{api.TestTypeConfirmed, api.TestTypeLikely, api.TestTypeNegative, api.TestTypeUserReport},
	}
	if len(*nonceFlag) > 0 {
		request.Nonce = *nonceFlag
	}

	resp, err := client.Verify(ctx, request)
	if err != nil {
		return err
	}
	logger.Infow("success", "response", resp)
	return nil
}
