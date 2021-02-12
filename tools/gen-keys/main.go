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

// Utility for creating keys using the Key Manager. The Key Manager must support
// creating keys.
package main

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/logging"

	"github.com/sethvargo/go-signalcontext"
)

func main() {
	flag.Parse()

	ctx, done := signalcontext.OnInterrupt()

	logger := logging.NewLoggerFromEnv().Named("gen-keys")
	ctx = logging.WithLogger(ctx, logger)

	err := realMain(ctx)
	done()

	if err != nil {
		logger.Fatal(err)
	}
}

func realMain(ctx context.Context) error {
	_, self, _, ok := runtime.Caller(1)
	if !ok {
		return fmt.Errorf("failed to get caller")
	}

	localDir := filepath.Join(filepath.Dir(self), "../../local")
	kms, err := keys.NewFilesystem(ctx, &keys.Config{
		FilesystemRoot: localDir,
	})
	if err != nil {
		return fmt.Errorf("failed to build certificate key manager: %w", err)
	}

	// Create certificate keys
	{
		kmst, ok := kms.(keys.SigningKeyManager)
		if !ok {
			return fmt.Errorf("key manager cannot sign: %T", kms)
		}

		parent, err := kmst.CreateSigningKey(ctx, "system", "certificate-signing")
		if err != nil {
			return fmt.Errorf("failed to create certificate signing key: %w", err)
		}
		list, err := kmst.SigningKeyVersions(ctx, parent)
		if err != nil {
			return fmt.Errorf("failed to list signing key versions: %w", err)
		}

		var latest string
		if len(list) == 0 {
			latest, err = kmst.CreateKeyVersion(ctx, parent)
			if err != nil {
				return fmt.Errorf("failed to create certificate signing key version: %w", err)
			}
		} else {
			latest = list[0].KeyID()
		}

		fmt.Printf("\nCertificate signing key version:\n\n")
		fmt.Printf("    export CERTIFICATE_SIGNING_KEY=\"%s\"\n", latest)
	}

	// Create database keys
	{
		kmst, ok := kms.(keys.EncryptionKeyManager)
		if !ok {
			return fmt.Errorf("key manager cannot sign: %T", kms)
		}

		parent, err := kmst.CreateEncryptionKey(ctx, "system", "database-encryption")
		if err != nil {
			return fmt.Errorf("failed to create database encryption key")
		}
		if _, err := kmst.CreateKeyVersion(ctx, parent); err != nil {
			return fmt.Errorf("failed to create database encryption key version: %w", err)
		}

		fmt.Printf("\nDatabase encryption key:\n\n")
		fmt.Printf("    export DB_ENCRYPTION_KEY=\"%s\"\n", parent)
	}

	// Print realm-specific certificate signing ring
	{
		fmt.Printf("\nDatabase key ring:\n\n")
		fmt.Printf("    export DB_KEYRING=\"%s\"\n", "/realm")
	}

	// Create token keys
	{
		kmst, ok := kms.(keys.SigningKeyManager)
		if !ok {
			return fmt.Errorf("key manager cannot sign: %T", kms)
		}

		parent, err := kmst.CreateSigningKey(ctx, "system", "token-signing")
		if err != nil {
			return fmt.Errorf("failed to create token signing key: %w", err)
		}
		list, err := kmst.SigningKeyVersions(ctx, parent)
		if err != nil {
			return fmt.Errorf("failed to list signing key versions: %w", err)
		}

		var latest string
		if len(list) == 0 {
			latest, err = kmst.CreateKeyVersion(ctx, parent)
			if err != nil {
				return fmt.Errorf("failed to create token signing key version: %w", err)
			}
		} else {
			latest = list[0].KeyID()
		}

		fmt.Printf("\nToken signing key version:\n\n")
		fmt.Printf("    export TOKEN_SIGNING_KEY=\"%s\"\n", latest)
	}

	return nil
}
