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
	"time"

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
		{
			ID: "00004-CreateTokens",
			Migrate: func(tx *gorm.DB) error {
				logger.Infof("db migrations: creating tokens table")
				return tx.AutoMigrate(&Token{}).Error
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.DropTable("tokens").Error
			},
		},
		{
			ID: "00005-CreateCleanups",
			Migrate: func(tx *gorm.DB) error {
				logger.Infof("db migrations: creating cleanup status table")
				if err := tx.AutoMigrate(&CleanupStatus{}).Error; err != nil {
					return err
				}
				// Seed database w/ cleanup record.
				if err := tx.Create(&CleanupStatus{Type: "cleanup", Generation: 1, NotBefore: time.Now()}).Error; err != nil {
					return err
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.DropTable("cleanup_statuses").Error
			},
		},
		{
			ID: "00006-AddIndexes",
			Migrate: func(tx *gorm.DB) error {
				logger.Infof("db migrations: add users purge index")
				if err := tx.Model(&User{}).AddIndex("users_purge_index", "disabled", "updated_at").Error; err != nil {
					return err
				}
				logger.Infof("db migrations: add verification code purge index")
				if err := tx.Model(&VerificationCode{}).AddIndex("ver_code_purge_index", "expires_at").Error; err != nil {
					return err
				}
				logger.Infof("db migrations: add tokens purge index")
				if err := tx.Model(&VerificationCode{}).AddIndex("token_purge_index", "expires_at").Error; err != nil {
					return err
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				logger.Infof("db migrations: drop users purge index")
				if err := tx.Model(&User{}).RemoveIndex("users_purge_index").Error; err != nil {
					return err
				}
				logger.Infof("db migrations: drop verification code purge index")
				if err := tx.Model(&VerificationCode{}).RemoveIndex("ver_code_purge_index").Error; err != nil {
					return err
				}
				logger.Infof("db migrations: drop tokens purge index")
				if err := tx.Model(&VerificationCode{}).RemoveIndex("token_purge_index").Error; err != nil {
					return err
				}
				return nil
			},
		},
		{
			ID: "00007-AddSymptomOnset",
			Migrate: func(tx *gorm.DB) error {
				logger.Info("db migrations: rename test_date to symptom_date")
				// AutoMigrate will add missing fields.
				if err := tx.AutoMigrate(&VerificationCode{}).Error; err != nil {
					return err
				}
				// If not upgrading from an old version, this column will have never been created.
				if tx.NewScope(&VerificationCode{}).HasColumn("test_date") {
					if err := tx.Model(&VerificationCode{}).DropColumn("test_date").Error; err != nil {
						return err
					}
				}

				if err := tx.AutoMigrate(&Token{}).Error; err != nil {
					return err
				}
				// If not upgrading from an old version, this column will have never been created.
				if tx.NewScope(&Token{}).HasColumn("test_date") {
					if err := tx.Model(&Token{}).DropColumn("test_date").Error; err != nil {
						return err
					}
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				logger.Info("db migrations: rename symptom_date to test_date")
				if err := tx.Model(&VerificationCode{}).DropColumn("symptom_date").Error; err != nil {
					return err
				}
				if err := tx.Model(&Token{}).DropColumn("symptom_date").Error; err != nil {
					return err
				}
				return nil
			},
		},
	})

	logger.Infof("database migrations complete")

	return m.Migrate()
}
