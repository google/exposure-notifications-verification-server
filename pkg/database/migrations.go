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
	"fmt"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/logging"

	"github.com/jinzhu/gorm"
	"gopkg.in/gormigrate.v1"
)

const initState = "00000-Init"

func (db *Database) getMigrations(ctx context.Context) *gormigrate.Gormigrate {
	logger := logging.FromContext(ctx)
	options := gormigrate.DefaultOptions
	options.UseTransaction = true
	return gormigrate.New(db.db, options, []*gormigrate.Migration{
		{
			ID: initState,
			Migrate: func(tx *gorm.DB) error {
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				return nil
			},
		},
		{
			ID: "00001-CreateUsers",
			Migrate: func(tx *gorm.DB) error {
				// This is "out of order" as it were, but is needed to bootstrap fresh systems.
				// Also in migration 8
				logger.Infof("db migrations: creating realms table")
				if err := tx.AutoMigrate(&Realm{}).Error; err != nil {
					return err
				}

				logger.Infof("db migrations: creating users table")
				return tx.AutoMigrate(&User{}).Error
			},
			Rollback: func(tx *gorm.DB) error {
				if err := tx.DropTable("users").Error; err != nil {
					return err
				}
				return tx.DropTable("realms").Error
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
		{
			ID: "00008-AddKeyTypes",
			Migrate: func(tx *gorm.DB) error {
				logger.Infof("db migrations: upgrading authorized_apps table.")
				return tx.AutoMigrate(&AuthorizedApp{}).Error
			},
			Rollback: func(tx *gorm.DB) error {
				if err := tx.Model(&AuthorizedApp{}).DropColumn("admin_key").Error; err != nil {
					return err
				}
				return nil
			},
		},
		{
			ID: "00009-AddIssuerColumns",
			Migrate: func(tx *gorm.DB) error {
				logger.Infof("db migrations: adding issuer columns to issued codes")
				return tx.AutoMigrate(&VerificationCode{}).Error
			},
			Rollback: func(tx *gorm.DB) error {
				if err := tx.Model(&AuthorizedApp{}).DropColumn("issuing_user").Error; err != nil {
					return err
				}
				if err := tx.Model(&AuthorizedApp{}).DropColumn("issuing_app").Error; err != nil {
					return err
				}
				return nil
			},
		},
		{
			ID: "00010-AddSMSConfig",
			Migrate: func(tx *gorm.DB) error {
				logger.Infof("db migrations: adding sms_configs table")
				return tx.AutoMigrate(&SMSConfig{}).Error
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.DropTable("sms_configs").Error
			},
		},
		{
			ID: "00011-AddRealms",
			Migrate: func(tx *gorm.DB) error {
				logger.Info("db migrations: create realms table")
				// Add the realms table.
				if err := tx.AutoMigrate(&Realm{}).Error; err != nil {
					return err
				}
				logger.Info("Create the DEFAULT realm")
				// Create the default realm.
				defaultRealm := Realm{
					Name: "Default",
				}
				if err := tx.Create(&defaultRealm).Error; err != nil {
					return err
				}

				// Add realm relations to the rest of the tables.
				logger.Info("Add RealmID to Users.")
				if err := tx.AutoMigrate(&User{}).Error; err != nil {
					return err
				}
				logger.Info("Join Users to Default Realm")
				var users []*User
				if err := tx.Unscoped().Find(&users).Error; err != nil {
					return err
				}
				for _, u := range users {
					logger.Infof("added user: %v to default realm", u.ID)
					defaultRealm.AddUser(u)
					if u.Admin {
						defaultRealm.AddAdminUser(u)
					}
				}

				logger.Info("Add RealmID to AuthorizedApps.")
				if err := tx.AutoMigrate(&AuthorizedApp{}).Error; err != nil {
					return err
				}
				var authApps []*AuthorizedApp
				if err := tx.Unscoped().Find(&authApps).Error; err != nil {
					return err
				}
				for _, a := range authApps {
					logger.Infof("added auth app: %v to default realm", a.Name)
					defaultRealm.AddAuthorizedApp(a)
				}

				if err := tx.Save(&defaultRealm).Error; err != nil {
					return err
				}

				logger.Info("Add RealmID to VerificationCodes.")
				if err := tx.AutoMigrate(&VerificationCode{}).Error; err != nil {
					return err
				}
				logger.Info("Join existing VerificationCodes to default realm")
				if err := tx.Exec("UPDATE verification_codes SET realm_id=?", defaultRealm.ID).Error; err != nil {
					return err
				}

				logger.Info("Add RealmID to Tokens.")
				if err := tx.AutoMigrate(&Token{}).Error; err != nil {
					return err
				}
				logger.Info("Join existing tokens to default realm")
				if err := tx.Exec("UPDATE tokens SET realm_id=?", defaultRealm.ID).Error; err != nil {
					return err
				}

				logger.Info("Add RealmID to SMSConfig.")
				if err := tx.AutoMigrate(&SMSConfig{}).Error; err != nil {
					return err
				}
				logger.Info("Join existing SMS config to default realm")
				if err := tx.Exec("UPDATE sms_configs SET realm_id=?", defaultRealm.ID).Error; err != nil {
					return err
				}

				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				ddl := []string{
					"ALTER TABLE sms_configs DROP COLUMN realm_id",
					"ALTER TABLE tokens DROP COUMN realm_id",
					"ALTER TABLE verification_codes DROP COLUMN realm_id",
					"ALTER TABLE authorized_apps DROP COLUMN realm_id",
					"DROP TABLE admin_realms",
					"DROP TABLE user_realms",
					"DROP TABLE realms",
				}
				for _, stmt := range ddl {
					if err := tx.Exec(stmt).Error; err != nil {
						return fmt.Errorf("unable to execute '%v': %w", stmt, err)
					}
				}
				return nil
			},
		},
	})
}

// MigrateTo migrates the database to a specific target migration ID.
func (db *Database) MigrateTo(ctx context.Context, target string, rollback bool) error {
	logger := logging.FromContext(ctx)
	m := db.getMigrations(ctx)
	logger.Infof("database migrations started")

	var err error
	if target == "" {
		if rollback {
			err = m.RollbackTo(initState)
		} else {
			err = m.Migrate() // run all remaining migrations.
		}
	} else {
		if rollback {
			err = m.RollbackTo(target)
		} else {
			err = m.MigrateTo(target)
		}
	}

	if err != nil {
		logger.Errorf("database migrations failed: %v", err)
		return nil
	}
	logger.Infof("database migrations completed")
	return nil
}

// RunMigrations will apply sequential, transactional migrations to the database
func (db *Database) RunMigrations(ctx context.Context) error {
	logger := logging.FromContext(ctx)
	m := db.getMigrations(ctx)
	logger.Infof("database migrations started")
	if err := m.Migrate(); err != nil {
		logger.Errorf("migrations failed: %v", err)
		return err
	}
	logger.Infof("database migrations completed")
	return nil
}
