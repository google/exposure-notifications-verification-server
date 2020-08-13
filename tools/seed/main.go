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

// Package main provides a utility that bootstraps the initial database with
// users and realms.
package main

import (
	"context"
	"fmt"

	"github.com/google/exposure-notifications-verification-server/pkg/database"

	"github.com/google/exposure-notifications-server/pkg/logging"

	"github.com/sethvargo/go-envconfig"
	"github.com/sethvargo/go-signalcontext"
)

func main() {
	ctx, done := signalcontext.OnInterrupt()

	logger := logging.NewLogger(true)
	ctx = logging.WithLogger(ctx, logger)

	err := realMain(ctx)
	done()

	if err != nil {
		logger.Fatal(err)
	}
}

func realMain(ctx context.Context) error {
	logger := logging.FromContext(ctx).Named("seed")

	var config database.Config
	if err := envconfig.Process(ctx, &config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	db, err := config.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load database config: %w", err)
	}
	if err := db.Open(ctx); err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Create users
	user := &database.User{Email: "user@example.com", Name: "Demo User"}
	if err := db.SaveUser(user); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	logger.Infow("created user", "user", user)

	admin := &database.User{Email: "admin@example.com", Name: "Admin User"}
	if err := db.SaveUser(admin); err != nil {
		return fmt.Errorf("failed to create admin: %w", err)
	}
	logger.Infow("created admin", "admin", admin)

	super := &database.User{Email: "super@example.com", Name: "Super User", Admin: true}
	if err := db.SaveUser(super); err != nil {
		return fmt.Errorf("failed to create super: %w", err)
	}
	logger.Infow("created super", "super", super)

	// Create a realm
	realm1 := &database.Realm{
		Name:           "Narnia",
		AuthorizedApps: nil,
		RealmUsers:     []*database.User{user, admin},
		RealmAdmins:    []*database.User{admin},
	}
	if err := db.SaveRealm(realm1); err != nil {
		return fmt.Errorf("failed to create realm")
	}
	logger.Infow("created realm", "realm", realm1)

	// Create another realm
	realm2 := &database.Realm{
		Name:           "Wonderland",
		AuthorizedApps: nil,
		RealmUsers:     []*database.User{user},
	}
	if err := db.SaveRealm(realm2); err != nil {
		return fmt.Errorf("failed to create realm")
	}
	logger.Infow("created realm", "realm", realm2)

	// Create a device API key
	deviceAPIKey, _, err := db.CreateAuthorizedApp(realm1.ID, "Corona Capture", database.APIUserTypeDevice)
	if err != nil {
		return fmt.Errorf("failed to create device api key")
	}
	logger.Infow("created device api key", "key", deviceAPIKey)

	// Create an admin API key
	adminAPIKey, _, err := db.CreateAuthorizedApp(realm1.ID, "Tracing Tracker", database.APIUserTypeAdmin)
	if err != nil {
		return fmt.Errorf("failed to create admin api key")
	}
	logger.Infow("created device api key", "key", adminAPIKey)

	return nil
}
