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
	"os"
	"strconv"

	"github.com/google/exposure-notifications-verification-server/pkg/config"
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

	if len(flag.Args()) > 0 {
		return fmt.Errorf("unexpected arguments: %v", flag.Args())
	}

	if *emailFlag == "" {
		return fmt.Errorf("--email must be passed and cannot be empty")
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

	if userRealm != nil {
		user.AddRealm(userRealm)
		if *realmAdminFlag {
			user.AddRealmAdmin(userRealm)
		}
	}

	if err := db.SaveUser(user); err != nil {
		return fmt.Errorf("failed to save user: %w: %v", err, user.ErrorMessages())
	}
	logger.Infow("saved user", "user", user)

	return nil
}

func findRealm(db *database.Database, id uint) (*database.Realm, error) {
	if id == 0 {
		return nil, nil
	}
	return db.GetRealm(id)
}
