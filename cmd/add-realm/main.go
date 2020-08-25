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

	"github.com/google/exposure-notifications-server/pkg/logging"

	"github.com/sethvargo/go-envconfig"
	"github.com/sethvargo/go-signalcontext"
)

var (
	nameFlag            = flag.String("name", "", "name of the realm to add")
	useSystemSigningKey = flag.Bool("use-system-signing-key", false, "if set, the system signing key will be used, otherwise a per-realm signing key will be created.")
	issFlag             = flag.String("iss", "", "name is the issuer (iss) for the verification certificatates for this realm")
	audFlag             = flag.String("aud", "", "name is the audience (aud) for the verification certificatates for this realm")
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

	if !*useSystemSigningKey {
		if *issFlag == "" {
			return fmt.Errorf("-iss must be passed and cannot be empty when not using the system signing keys")
		}
		if *audFlag == "" {
			return fmt.Errorf("-aud must be passed and cannot be empty when not using the system signing keys")
		}
	}

	var cfg database.Config
	if err := config.ProcessWith(ctx, &cfg, envconfig.OsLookuper()); err != nil {
		return fmt.Errorf("failed to process config: %w", err)
	}

	db, err := cfg.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load database config: %w", err)
	}
	if err := db.Open(ctx); err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// See if realm exists.
	realm, err := db.FindRealmByName(*nameFlag)
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

	if *useSystemSigningKey {
		logger.Info("use-system-signing-key was passed, skipping creation of per-realm key.")
	} else {
		// Upgrade the realm to custom keys.
		realm.UseRealmCertificateKey = true
		realm.CertificateIssuer = *issFlag
		realm.CertificateAudience = *audFlag
		if err := db.SaveRealm(realm); err != nil {
			return fmt.Errorf("error upgrading realm to custom signing keys: %w", err)
		}

		kid, err := realm.CreateNewSigningKeyVersion(ctx, db)
		if err != nil {
			return fmt.Errorf("error creating signing keys for realm: %w", err)
		}
		logger.Info("created signing key for realm", "keyID", kid)
	}

	return nil
}
