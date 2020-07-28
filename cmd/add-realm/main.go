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
	"log"

	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/sethvargo/go-envconfig/pkg/envconfig"
)

func main() {
	name := flag.String("name", "", "name of the realm to add")
	flag.Parse()

	if *name == "" {
		log.Fatalf("--name must be passed and cannot be empty")
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

	realm := database.Realm{
		Name: *name,
	}
	if err := db.SaveRealm(&realm); err != nil {
		log.Fatalf("unable to create realm: %v", err)
	}
	log.Printf("created new realm: %v (%v)", realm.Name, realm.ID)
}
