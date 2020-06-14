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

type Database struct {
	db *gorm.DB
}

// Open created a DB connection through gorm.
func (c Config) Open() (*Database, error) {
	cstr := c.ConnectionString()
	fmt.Printf("Connecting to: %v", cstr)
	db, err := gorm.Open("postgres", c.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("database gorm.Open: %w", err)
	}
	return &Database{db}, nil
}
