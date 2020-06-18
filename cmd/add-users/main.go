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
	"strings"

	"github.com/google/exposure-notifications-verification-server/pkg/database"

	"github.com/jinzhu/gorm"
	"github.com/sethvargo/go-envconfig/pkg/envconfig"
)

func main() {
	emailFlag := flag.String("email", "", "email for the user to add")
	nameFlag := flag.String("name", "", "name of the user to add")
	adminFlag := flag.Bool("admin", false, "true if user is admin user")
	disabledFlag := flag.Bool("disabled", false, "true if user should be disabled")
	flag.Parse()

	if *emailFlag == "" {
		log.Fatal("--email must be passed and cannot be empty")
	}

	parts := strings.Split(*emailFlag, "@")
	if len(parts) != 2 {
		log.Fatalf("provide email address may not be valid, double check: '%v'", *emailFlag)
	}

	name := *nameFlag
	if name == "" {
		name = parts[0]
	}

	ctx := context.Background()
	var config database.Config
	if err := envconfig.Process(ctx, &config); err != nil {
		log.Fatalf("config error: %v", err)
	}

	db, err := config.Open()
	if err != nil {
		log.Fatalf("db connection failed: %v", err)
	}
	defer db.Close()

	user, err := db.FindUser(*emailFlag)
	if err == gorm.ErrRecordNotFound {
		// New record.
		user = &database.User{}
	} else if err != nil {
		log.Fatalf("unexpected error: %v", err)
	}

	// Update fields
	user.Email = *emailFlag
	user.Name = name
	user.Admin = *adminFlag
	user.Disabled = *disabledFlag

	if err := db.SaveUser(user); err != nil {
		log.Fatalf("error saving user: %v", err)
	}
	log.Printf("saved user: %+v", user)
}
