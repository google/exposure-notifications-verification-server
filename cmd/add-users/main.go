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
	"log"

	"github.com/google/exposure-notifications-verification-server/pkg/database"

	"github.com/jinzhu/gorm"
	"github.com/sethvargo/go-envconfig"
)

func main() {
	emailFlag := flag.String("email", "", "email for the user to add")
	nameFlag := flag.String("name", "", "name of the user to add")
	adminFlag := flag.Bool("admin", false, "true if user is admin user")
	disabledFlag := flag.Bool("disabled", false, "true if user should be disabled")
	realmID := flag.Int64("realm", -1, "realm to add the user to")
	realmAdminFlag := flag.Bool("admin-realm", false, "realm to add the user to")

	flag.Parse()

	if len(flag.Args()) > 0 {
		log.Fatal("Received unexpected arguments:", flag.Args())
	}

	if *emailFlag == "" {
		log.Fatal("--email must be passed and cannot be empty")
	}

	ctx := context.Background()
	var config database.Config
	if err := envconfig.Process(ctx, &config); err != nil {
		log.Fatalf("config error: %v", err)
	}

	db, err := config.Open(ctx)
	if err != nil {
		log.Fatalf("db connection failed: %v", err)
	}
	defer db.Close()

	userRealm, err := findRealm(db, *realmID)
	if err != nil {
		log.Fatalf("unable to find specified realmID: %v reason: %v", *realmID, err)
	}

	if userRealm == nil && !*adminFlag {
		log.Fatalf("Cannot create a non system admin user that is also not in any realms")
	}

	user, err := db.CreateUser(*emailFlag, *nameFlag, *adminFlag, *disabledFlag)
	if err == gorm.ErrRecordNotFound {
		log.Fatalf("unexpected error: %v", err)
	}
	log.Printf("saved user: %+v", user)

	if userRealm != nil {
		userRealm.AddUser(user)
		if *realmAdminFlag {
			userRealm.AddAdminUser(user)
		}
		if err := db.SaveRealm(userRealm); err != nil {
			log.Fatalf("failed to add user %v to realm %v; %v", user.Email, userRealm.Name, err)
		}
	}
}

func findRealm(db *database.Database, id int64) (*database.Realm, error) {
	if id < 0 {
		return nil, nil
	}
	return db.GetRealm(id)
}
