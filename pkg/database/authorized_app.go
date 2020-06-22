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

package database

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/jinzhu/gorm"
)

const (
	apiKeyBytes = 64 // 64 bytes is 86 chararacters in non-padded base64.
)

// AuthorizedApp represents an application that is authorized to verify
// verification codes and perform token exchanges.
// This is controlled via a generated API key.
type AuthorizedApp struct {
	gorm.Model
	Name   string `gorm:"type:varchar(100);unique_index"`
	APIKey string `gorm:"type:varchar(100);unique_index"`
}

// TODO(mikehelmick): Implement revoke API key functionality.

// TableName definition for the authorized apps relation.
func (AuthorizedApp) TableName() string {
	return "authorized_apps"
}

// ListAuthorizedApps retrieves all of the configured Authorized apps.
// Done without pagination, as the expeicted number of Authorized apps
// is low signal digits.
func (db *Database) ListAuthorizedApps(includeDeleted bool) ([]*AuthorizedApp, error) {
	var apps []*AuthorizedApp

	scope := db.db
	if includeDeleted {
		scope = db.db.Unscoped()
	}
	if err := scope.Order("name ASC").Find(&apps).Error; err != nil {
		return nil, fmt.Errorf("query authorized apps: %w", err)
	}
	return apps, nil
}

// CreateAuthorizedApp generates a new APIKey and assignes it to the specified
// name.
func (db *Database) CreateAuthorizedApp(name string) (*AuthorizedApp, error) {
	buffer := make([]byte, apiKeyBytes)
	_, err := rand.Read(buffer)
	if err != nil {
		return nil, fmt.Errorf("rand.Read: %v", err)
	}

	app := AuthorizedApp{
		Name:   name,
		APIKey: base64.RawStdEncoding.EncodeToString(buffer),
	}
	if err := db.db.Create(&app).Error; err != nil {
		return nil, fmt.Errorf("unable to save authorized app: %w", err)
	}
	return &app, nil
}

// FindAuthoirizedAppByAPIKey located an authorized app based on API key.
func (db *Database) FindAuthoirizedAppByAPIKey(apiKey string) (*AuthorizedApp, error) {
	var app AuthorizedApp
	if err := db.db.Where("api_key = ?", apiKey).First(&app).Error; err != nil {
		return nil, err
	}
	return &app, nil
}
