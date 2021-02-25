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

package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/logging"
)

const (
	authPrefix = "Authentication:"
)

var (
	flagPublicKey = flag.String("public-key", "", "public key in pem format")
	flagMessage   = flag.String("message", "", "full sms body including signature")
	flagPhone     = flag.String("phone", "", "phone number in e.164 format")
	flagDate      = flag.String("date", "", "message date")
	flagType      = flag.String("type", "EN Report", "signature type")
)

func main() {
	flag.Parse()

	ctx, done := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	logger := logging.NewLoggerFromEnv().Named("verify-sms-signature")
	ctx = logging.WithLogger(ctx, logger)

	err := realMain(ctx)
	done()

	if err != nil {
		fmt.Fprintf(os.Stderr, "✘ %s\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "✔ Signature is valid\n")
}

func realMain(ctx context.Context) error {
	logger := logging.FromContext(ctx)

	if *flagPublicKey == "" {
		return fmt.Errorf("-public-key is required")
	}
	publicKey, err := keys.ParseECDSAPublicKey(*flagPublicKey)
	if err != nil {
		return fmt.Errorf("failed to parse public key: %w", err)
	}

	if *flagMessage == "" {
		return fmt.Errorf("-message is required")
	}

	if *flagPhone == "" {
		return fmt.Errorf("-phone is required")
	}
	if (*flagPhone)[0] != '+' {
		return fmt.Errorf("-phone must be e.164 format")
	}

	if *flagDate == "" {
		return fmt.Errorf("-date is required")
	}

	if *flagType == "" {
		return fmt.Errorf("-type is required")
	}

	authIndex := strings.LastIndex(*flagMessage, authPrefix)
	if authIndex == -1 {
		return fmt.Errorf("message is not signed")
	}

	div := authIndex + len(authPrefix)

	body := (*flagMessage)[0:div]
	signingString := *flagType + "." + *flagPhone + "." + *flagDate + "." + body

	logger.Debugw("body", "value", body)
	logger.Debugw("signing string", "value", signingString)

	suffix := (*flagMessage)[div:]
	parts := strings.Split(suffix, ":")
	if len(parts) < 1 {
		return fmt.Errorf("invalid suffix %q", suffix)
	}
	encodedSignature := parts[len(parts)-1]

	logger.Debugw("extracted signature", "value", encodedSignature)

	sig, err := base64.RawStdEncoding.DecodeString(encodedSignature)
	if err != nil {
		return fmt.Errorf("invalid signature base64: %w", err)
	}

	digest := sha256.Sum256([]byte(signingString))
	logger.Debugw("computed digest", "value", digest[:])
	if !ecdsa.VerifyASN1(publicKey, digest[:], sig) {
		return fmt.Errorf("did not verify")
	}
	return nil
}
