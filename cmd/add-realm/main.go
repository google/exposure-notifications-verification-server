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

// Adds a new realm.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/gcpkms"

	"github.com/google/exposure-notifications-server/pkg/logging"

	"github.com/sethvargo/go-signalcontext"
)

var (
	nameFlag = flag.String("name", "", "name of the realm to add")
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

	if *nameFlag == "" {
		return fmt.Errorf("--name must be passed and cannot be empty")
	}

	cfg, err := config.NewToolConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to process environment: %w", err)
	}

	db, err := cfg.Database.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load database config: %w", err)
	}
	if err := db.Open(ctx); err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Create the KMS client.
	keys, err := gcpkms.New(ctx, &cfg.GCPKMS)
	if err != nil {
		return fmt.Errorf("unable to connect to key manager: %w", err)
	}

	// See if realm exists.
	realm, err := db.GetRealmByName(*nameFlag)
	if err != nil {
		logger.Infow("realm alredy exists, skipping create", "realm", realm)
	}

	if realm == nil {
		logger.Info("creating realm")
		realm = database.NewRealmWithDefaults(*nameFlag)
		if err := db.SaveRealm(realm); err != nil {
			return fmt.Errorf("failed to create realm: %w", err)
		}
		logger.Infow("created realm", "realm", realm)
	}

	// Ensure the realm has a signing key.
	realmKeys, err := realm.ListSigningKeys(db)
	if err != nil {
		return fmt.Errorf("unable to list signing keys for realm: %w", err)
	}
	if len(realmKeys) > 0 {
		logger.Infow("realm has signing keys already")
		return nil
	}

	versions, err := keys.GetSigningKeyVersions(ctx, cfg.CertificateSigningKeyRing, realm.SigningKeyID())
	if err != nil {
		return fmt.Errorf("unable to list signing keys on kms: %w", err)
	}
	if len(versions) > 0 {
		logger.Infow("relam has signing keys in the KMS", "keyRing", cfg.CertificateSigningKeyRing, "keyID", realm.SigningKeyID())
		return nil
	}

	id, err := keys.CreateSigningKeyVersion(ctx, cfg.CertificateSigningKeyRing, realm.SigningKeyID())
	if err != nil {
		return fmt.Errorf("unable to create signing key for realm: %w", err)
	}
	logger.Infow("provisioned certificate signing key for realm", "keyID", id)

	return nil
}
