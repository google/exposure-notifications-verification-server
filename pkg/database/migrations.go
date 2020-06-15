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
	"context"

	"github.com/google/exposure-notifications-verification-server/pkg/logging"

	"github.com/jinzhu/gorm"
	"gopkg.in/gormigrate.v1"
)

// RunMigrations will apply sequential, transactional migrations to the database
func (db *Database) RunMigrations(ctx context.Context) error {
	logger := logging.FromContext(ctx)
	m := gormigrate.New(db.db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "00001-CreateUsers",
			Migrate: func(tx *gorm.DB) error {
				logger.Infof("db migrations: creating users table")
				return tx.AutoMigrate(&User{}).Error
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.DropTable("users").Error
			},
		},
		{
			ID: "00002-CreateVerificationCodes",
			Migrate: func(tx *gorm.DB) error {
				logger.Infof("db migrations: creating verification codes table")
				return tx.AutoMigrate(&VerificationCode{}).Error
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.DropTable("verification_codes").Error
			},
		},
		{
			ID: "00003-CreateAuthorizedApps",
			Migrate: func(tx *gorm.DB) error {
				logger.Infof("db migrations: creating authorized apps table")
				return tx.AutoMigrate(&AuthorizedApp{}).Error
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.DropTable("authorized_apps").Error
			},
		},
	})

	logger.Infof("database migrations complete")

	return m.Migrate()
}
