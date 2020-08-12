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

// Adds a user or enables that user if they record already exists
package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/google/exposure-notifications-verification-server/pkg/database"

	"github.com/google/exposure-notifications-server/pkg/logging"

	"github.com/sethvargo/go-envconfig"
	"github.com/sethvargo/go-signalcontext"
)

var (
	emailFlag      = flag.String("email", "", "email for the user to add")
	nameFlag       = flag.String("name", "", "name of the user to add")
	adminFlag      = flag.Bool("admin", false, "true if user is admin user")
	realmID        = flag.Uint("realm", 0, "realm to add the user to")
	realmAdminFlag = flag.Bool("admin-realm", false, "realm to add the user to")
)

func main() {
	flag.Parse()

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
	logger := logging.FromContext(ctx)

	if len(flag.Args()) > 0 {
		return fmt.Errorf("unexpected arguments: %v", flag.Args())
	}

	if *emailFlag == "" {
		return fmt.Errorf("--email must be passed and cannot be empty")
	}

	var config database.Config
	if err := envconfig.Process(ctx, &config); err != nil {
		return fmt.Errorf("failed to process config: %w", err)
	}

	db, err := config.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load database config: %w", err)
	}
	if err := db.Open(ctx); err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	userRealm, err := findRealm(db, *realmID)
	if err != nil {
		return fmt.Errorf("unable to find specified realmID: %v reason: %w", *realmID, err)
	}

	if userRealm == nil && !*adminFlag {
		return fmt.Errorf("cannot create a non system admin user that is also not in any realms")
	}

	user := &database.User{
		Name:  *nameFlag,
		Email: *emailFlag,
		Admin: *adminFlag,
	}

	if err := db.SaveUser(user); err != nil {
		return fmt.Errorf("failed to save user: %w", err)
	}
	logger.Debugw("saved user", "user", user)

	if userRealm != nil {
		userRealm.AddUser(user)
		if *realmAdminFlag {
			userRealm.AddAdminUser(user)
		}
		if err := db.SaveRealm(userRealm); err != nil {
			return fmt.Errorf("failed to add user %v to realm %v; %w", user.Email, userRealm.Name, err)
		}
	}

	return nil
}

func findRealm(db *database.Database, id uint) (*database.Realm, error) {
	if id == 0 {
		return nil, nil
	}
	return db.GetRealm(id)
}
