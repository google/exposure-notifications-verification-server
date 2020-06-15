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

// Package database manages database connections and ORM integration.
package database

import (
	"fmt"

	"github.com/jinzhu/gorm"
	// ensure the postgres dialiect is compiled in.
	_ "github.com/jinzhu/gorm/dialects/postgres"
)

// Database is a handle to the database layer for the Exposure Notifications
// Verification Server.
type Database struct {
	db     *gorm.DB
	config *Config
}

// Open created a DB connection through gorm.
func (c *Config) Open() (*Database, error) {
	db, err := gorm.Open("postgres", c.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("database gorm.Open: %w", err)
	}
	return &Database{db, c}, nil
}

// Close will close the database connection. Should be deferred right after Open.
func (db *Database) Close() error {
	return db.db.Close()
}
