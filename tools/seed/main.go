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
	"os"

	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/sethvargo/go-envconfig"
	"github.com/sethvargo/go-signalcontext"
)

func main() {
	ctx, done := signalcontext.OnInterrupt()
	err := realMain(ctx)
	done()

	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(1)
	}
}

func realMain(ctx context.Context) error {
	var config database.Config
	if err := envconfig.Process(ctx, &config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	db, err := config.Open(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Create two users
	user := &database.User{Email: "user@example.com", Name: "Demo User"}
	if err := db.SaveUser(user); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	admin := &database.User{Email: "admin@example.com", Name: "Admin User"}
	if err := db.SaveUser(admin); err != nil {
		return fmt.Errorf("failed to create admin: %w", err)
	}

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

	// Create another realm
	realm2 := &database.Realm{
		Name:           "Wonderland",
		AuthorizedApps: nil,
		RealmUsers:     []*database.User{user},
	}
	if err := db.SaveRealm(realm2); err != nil {
		return fmt.Errorf("failed to create realm")
	}

	return nil
}
