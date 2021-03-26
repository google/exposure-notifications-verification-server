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
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"

	"github.com/jinzhu/gorm"
	"gopkg.in/gormigrate.v1"
)

const (
	initState                   = "00000-Init"
	VerCodesCodeUniqueIndex     = "uix_verification_codes_realm_code"
	VerCodesLongCodeUniqueIndex = "uix_verification_codes_realm_long_code"
)

func (db *Database) Migrations(ctx context.Context) []*gormigrate.Migration {
	logger := logging.FromContext(ctx)

	return []*gormigrate.Migration{
		{
			ID: initState,
			Migrate: func(tx *gorm.DB) error {
				// Create required extensions on new DB so AutoMigrate doesn't fail.
				return tx.Exec("CREATE EXTENSION IF NOT EXISTS hstore").Error
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
				if err := tx.AutoMigrate(&Realm{}).Error; err != nil {
					return err
				}
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
				return tx.AutoMigrate(&VerificationCode{}).Error
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.DropTable("verification_codes").Error
			},
		},
		{
			ID: "00003-CreateAuthorizedApps",
			Migrate: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&AuthorizedApp{}).Error
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.DropTable("authorized_apps").Error
			},
		},
		{
			ID: "00004-CreateTokens",
			Migrate: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&Token{}).Error
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.DropTable("tokens").Error
			},
		},
		{
			ID: "00005-CreateCleanups",
			Migrate: func(tx *gorm.DB) error {
				// CleanupStatus is the model that existed at the time of this
				// migration.
				type CleanupStatus struct {
					gorm.Model
					Type       string `gorm:"type:varchar(50);unique_index"`
					Generation uint
					NotBefore  time.Time
				}

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
				if err := tx.Model(&User{}).AddIndex("users_purge_index", "updated_at").Error; err != nil {
					return err
				}
				if err := tx.Model(&VerificationCode{}).AddIndex("ver_code_purge_index", "expires_at").Error; err != nil {
					return err
				}
				if err := tx.Model(&VerificationCode{}).AddIndex("token_purge_index", "expires_at").Error; err != nil {
					return err
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				if err := tx.Model(&User{}).RemoveIndex("users_purge_index").Error; err != nil {
					return err
				}
				if err := tx.Model(&VerificationCode{}).RemoveIndex("ver_code_purge_index").Error; err != nil {
					return err
				}
				if err := tx.Model(&VerificationCode{}).RemoveIndex("token_purge_index").Error; err != nil {
					return err
				}
				return nil
			},
		},
		{
			ID: "00007-AddSymptomOnset",
			Migrate: func(tx *gorm.DB) error {
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
				return tx.AutoMigrate(&VerificationCode{}).Error
			},
			Rollback: func(tx *gorm.DB) error {
				if err := tx.Model(&VerificationCode{}).DropColumn("issuing_user").Error; err != nil {
					return err
				}
				if err := tx.Model(&VerificationCode{}).DropColumn("issuing_app").Error; err != nil {
					return err
				}
				return nil
			},
		},
		{
			ID: "00010-AddSMSConfig",
			Migrate: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&SMSConfig{}).Error
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.DropTable("sms_configs").Error
			},
		},
		{
			ID: "00011-AddRealms",
			Migrate: func(tx *gorm.DB) error {
				// Add the realms table.
				if err := tx.AutoMigrate(&Realm{}).Error; err != nil {
					return err
				}
				// Create the default realm with all of the default settings.
				defaultRealm := NewRealmWithDefaults("Default")
				defaultRealm.RequireDate = false
				if err := tx.FirstOrCreate(defaultRealm).Error; err != nil {
					return err
				}

				// Add realm relations to the rest of the tables.
				if err := tx.AutoMigrate(&User{}).Error; err != nil {
					return err
				}
				var users []*User
				if err := tx.Find(&users).Error; err != nil {
					return err
				}
				for _, u := range users {
					permission := rbac.LegacyRealmUser
					if u.SystemAdmin {
						permission = rbac.LegacyRealmAdmin
					}
					if err := u.AddToRealm(db, defaultRealm, permission, System); err != nil {
						return err
					}
				}

				if err := tx.AutoMigrate(&AuthorizedApp{}).Error; err != nil {
					return err
				}
				var authApps []*AuthorizedApp
				if err := tx.Unscoped().Find(&authApps).Error; err != nil {
					return err
				}
				for _, a := range authApps {
					a.RealmID = defaultRealm.ID
					if err := tx.Save(a).Error; err != nil {
						return err
					}
				}

				if err := tx.AutoMigrate(&VerificationCode{}).Error; err != nil {
					return err
				}
				if err := tx.Exec("UPDATE verification_codes SET realm_id=?", defaultRealm.ID).Error; err != nil {
					return err
				}

				if err := tx.AutoMigrate(&Token{}).Error; err != nil {
					return err
				}
				if err := tx.Exec("UPDATE tokens SET realm_id=?", defaultRealm.ID).Error; err != nil {
					return err
				}

				if err := tx.AutoMigrate(&SMSConfig{}).Error; err != nil {
					return err
				}
				if err := tx.Exec("UPDATE sms_configs SET realm_id=?", defaultRealm.ID).Error; err != nil {
					return err
				}

				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				ddl := []string{
					"ALTER TABLE sms_configs DROP COLUMN IF EXISTS realm_id",
					"ALTER TABLE tokens DROP COLUMN IF EXISTS realm_id",
					"ALTER TABLE verification_codes DROP COLUMN IF EXISTS realm_id",
					"ALTER TABLE authorized_apps DROP COLUMN IF EXISTS realm_id",
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
		{
			ID: "00012-DropAuthorizedAppUniqueNameIndex",
			Migrate: func(tx *gorm.DB) error {
				sql := "DROP INDEX IF EXISTS uix_authorized_apps_name"
				if err := tx.Exec(sql).Error; err != nil {
					return err
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				return nil
			},
		},
		{
			ID: "00013-AddCompositeIndexOnAuthorizedApp",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(&AuthorizedApp{}).Error; err != nil {
					return err
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				return nil
			},
		},
		{
			ID: "00014-DropUserPurgeIndex",
			Migrate: func(tx *gorm.DB) error {
				sql := "DROP INDEX IF EXISTS users_purge_index"
				return tx.Exec(sql).Error
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Model(&User{}).AddIndex("users_purge_index", "updated_at").Error
			},
		},
		{
			ID: "00015-DropUserDisabled",
			Migrate: func(tx *gorm.DB) error {
				sql := "ALTER TABLE users DROP COLUMN IF EXISTS disabled"
				return tx.Exec(sql).Error
			},
			Rollback: func(tx *gorm.DB) error {
				sql := "ALTER TABLE users ADD COLUMN disabled bool NOT NULL DEFAULT true"
				return tx.Exec(sql).Error
			},
		},
		{
			ID: "00016-MigrateSMSConfigs",
			Migrate: func(tx *gorm.DB) error {
				var sms SMSConfig
				rows, err := tx.Model(&SMSConfig{}).Rows()
				if err != nil {
					return err
				}
				defer rows.Close()

				for rows.Next() {
					if err := tx.ScanRows(rows, &sms); err != nil {
						return err
					}

					// Convert from secret manager -> kms.
					if strings.HasPrefix(sms.TwilioAuthToken, "projects/") {
						// Get the secret
						val, err := db.secretManager.GetSecretValue(ctx, sms.TwilioAuthToken)
						if err != nil {
							return err
						}

						// Save the plaintext back on the model. The model's hook will
						// encrypt this with the KMS configuration.
						sms.TwilioAuthToken = val
						if err := db.SaveSMSConfig(&sms); err != nil {
							return err
						}
					}
				}

				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				return nil
			},
		},
		{
			ID: "00017-AddIssuerIDColumns",
			Migrate: func(tx *gorm.DB) error {
				err := tx.AutoMigrate(&VerificationCode{}, &UserStat{}, &AuthorizedAppStat{}).Error
				return err
			},
			Rollback: func(tx *gorm.DB) error {
				if tx.NewScope(&VerificationCode{}).HasColumn("issuing_user_id") {
					if err := tx.Model(&VerificationCode{}).DropColumn("issuing_user_id").Error; err != nil {
						return err
					}
				}
				if tx.NewScope(&VerificationCode{}).HasColumn("issuing_app_id") {
					if err := tx.Model(&VerificationCode{}).DropColumn("issuing_app_id").Error; err != nil {
						return err
					}
				}
				if err := tx.DropTableIfExists(&UserStat{}).Error; err != nil {
					return err
				}
				return nil
			},
		},
		{
			ID: "00018-IncreaseAPIKeySize",
			Migrate: func(tx *gorm.DB) error {
				sql := "ALTER TABLE authorized_apps ALTER COLUMN api_key TYPE varchar(512)"
				return tx.Exec(sql).Error
			},
			Rollback: func(tx *gorm.DB) error {
				sql := "ALTER TABLE authorized_apps ALTER COLUMN api_key TYPE varchar(100)"
				return tx.Exec(sql).Error
			},
		},
		{
			ID: "00019-AddAPIKeyPreviewAuthApp",
			Migrate: func(tx *gorm.DB) error {
				return tx.AutoMigrate(AuthorizedApp{}).Error
			},
			Rollback: func(tx *gorm.DB) error {
				return nil
			},
		},
		{
			ID: "00020-HMACAPIKeys",
			Migrate: func(tx *gorm.DB) error {
				var apps []AuthorizedApp
				if err := tx.Model(AuthorizedApp{}).Find(&apps).Error; err != nil {
					return err
				}

				for _, app := range apps {
					app := app

					// If the key has a preview, it's v2
					if app.APIKeyPreview != "" {
						continue
					}

					apiKeyPreview := app.APIKey[:6]
					newAPIKey, err := db.GenerateAPIKeyHMAC(app.APIKey)
					if err != nil {
						return fmt.Errorf("failed to hmac %v: %w", app.Name, err)
					}

					app.APIKey = newAPIKey
					app.APIKeyPreview = apiKeyPreview

					if err := db.db.Save(&app).Error; err != nil {
						return fmt.Errorf("failed to save %v: %w", app.Name, err)
					}
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				return nil
			},
		},
		{
			ID: "00021-AddUUIDExtension",
			Migrate: func(tx *gorm.DB) error {
				return tx.Exec("CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\"").Error
			},
			Rollback: func(tx *gorm.DB) error {
				return nil
			},
		},
		{
			ID: "00022-AddUUIDToVerificationCodes",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(VerificationCode{}).Error; err != nil {
					return fmt.Errorf("failed to auto migrate: %w", err)
				}

				if err := tx.Exec("ALTER TABLE verification_codes ALTER COLUMN uuid SET DEFAULT uuid_generate_v4()").Error; err != nil {
					return fmt.Errorf("failed to set default: %w", err)
				}

				if err := tx.Exec("UPDATE verification_codes SET uuid = uuid_generate_v4() WHERE uuid IS NULL").Error; err != nil {
					return fmt.Errorf("failed to add defaults: %w", err)
				}

				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				if err := tx.Exec("ALTER TABLE verification_codes ALTER COLUMN uuid DROP DEFAULT").Error; err != nil {
					return fmt.Errorf("failed to set default: %w", err)
				}

				return nil
			},
		},
		{
			ID: "00023-MakeUUIDVerificationCodesNotNull",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.Exec("ALTER TABLE verification_codes ALTER COLUMN uuid SET NOT NULL").Error; err != nil {
					return fmt.Errorf("failed to set null: %w", err)
				}

				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				if err := tx.Exec("ALTER TABLE verification_codes ALTER COLUMN uuid DROP NOT NULL").Error; err != nil {
					return fmt.Errorf("failed to set null: %w", err)
				}

				return nil
			},
		},
		{
			ID: "00024-AddTestTypesToRealms",
			Migrate: func(tx *gorm.DB) error {
				sql := fmt.Sprintf("ALTER TABLE realms ADD COLUMN IF NOT EXISTS allowed_test_types INTEGER DEFAULT %d",
					TestTypeConfirmed|TestTypeLikely|TestTypeNegative)
				if err := tx.Exec(sql).Error; err != nil {
					return err
				}

				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				if err := tx.Exec("ALTER TABLE realms DROP COLUMN IF EXISTS allowed_test_types").Error; err != nil {
					return err
				}

				return nil
			},
		},
		{
			ID: "00025-SetTestTypesNotNull",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.Exec("ALTER TABLE realms ALTER COLUMN allowed_test_types SET NOT NULL").Error; err != nil {
					return err
				}

				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				if err := tx.Exec("ALTER TABLE realms ALTER COLUMN allowed_test_types DROP NOT NULL").Error; err != nil {
					return err
				}

				return nil
			},
		},
		{
			ID: "00026-EnableExtension_citext",
			Migrate: func(tx *gorm.DB) error {
				return tx.Exec("CREATE EXTENSION citext").Error
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Exec("DROP EXTENSION citext").Error
			},
		},
		{
			ID: "00027-AlterColumns_citext",
			Migrate: func(tx *gorm.DB) error {
				sqls := []string{
					"ALTER TABLE authorized_apps ALTER COLUMN name TYPE CITEXT",
					"ALTER TABLE realms ALTER COLUMN name TYPE CITEXT",
					"ALTER TABLE users ALTER COLUMN email TYPE CITEXT",
				}

				for _, sql := range sqls {
					if err := tx.Exec(sql).Error; err != nil {
						return err
					}
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				sqls := []string{
					"ALTER TABLE authorized_apps ALTER COLUMN name TYPE TEXT",
					"ALTER TABLE realms ALTER COLUMN name TYPE TEXT",
					"ALTER TABLE users ALTER COLUMN email TYPE TEXT",
				}

				for _, sql := range sqls {
					if err := tx.Exec(sql).Error; err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			ID: "00028-AddSMSDeeplinkFields",
			Migrate: func(tx *gorm.DB) error {
				// long_code cannot be auto migrated because of unique index.
				// manually create long_code and long_expires_at and backfill with existing data.
				sqls := []string{
					"ALTER TABLE verification_codes ADD COLUMN IF NOT EXISTS long_code VARCHAR(20)",
					"UPDATE verification_codes SET long_code = code",
					"CREATE UNIQUE INDEX IF NOT EXISTS uix_verification_codes_long_code ON verification_codes(long_code)",
					"ALTER TABLE verification_codes ALTER COLUMN long_code SET NOT NULL",
					"ALTER TABLE verification_codes ADD COLUMN IF NOT EXISTS long_expires_at TIMESTAMPTZ",
					"UPDATE verification_codes SET long_expires_at = expires_at",
				}
				for _, stmt := range sqls {
					if err := tx.Exec(stmt).Error; err != nil {
						return fmt.Errorf("unable to execute '%v': %w", stmt, err)
					}
				}

				if err := tx.AutoMigrate(&Realm{}).Error; err != nil {
					return err
				}
				if err := tx.AutoMigrate(&VerificationCode{}).Error; err != nil {
					return err
				}

				if err := tx.Model(&VerificationCode{}).AddIndex("ver_code_long_purge_index", "long_expires_at").Error; err != nil {
					return err
				}

				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				dropColumns := []string{
					"long_code_length",
					"long_code_duration",
					"region_code",
					"code_length",
					"code_duration",
					"sms_text_template",
				}
				for _, col := range dropColumns {
					stmt := fmt.Sprintf("ALTER TABLE realms DROP COLUMN IF EXISTS %s", col)
					if err := tx.Exec(stmt).Error; err != nil {
						return fmt.Errorf("unable to execute '%v': %w", stmt, err)
					}
				}
				return nil
			},
		},
		{
			ID: "00029-IncreaseVerificationCodeSizes",
			Migrate: func(tx *gorm.DB) error {
				sqls := []string{
					"ALTER TABLE verification_codes ALTER COLUMN code TYPE varchar(512)",
					"ALTER TABLE verification_codes ALTER COLUMN long_code TYPE varchar(512)",
				}

				for _, sql := range sqls {
					if err := tx.Exec(sql).Error; err != nil {
						return err
					}
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				// No rollback for this, which would destroy data.
				return nil
			},
		},
		{
			ID: "00030-HMACCodes",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(&Realm{}).Error; err != nil {
					return err
				}

				var codes []VerificationCode
				if err := tx.Model(VerificationCode{}).Find(&codes).Error; err != nil {
					return err
				}

				for _, code := range codes {
					code := code
					changed := false

					// Sanity
					if len(code.Code) < 20 {
						h, err := db.GenerateVerificationCodeHMAC(code.Code)
						if err != nil {
							return err
						}
						code.Code = h
						changed = true
					}

					// Sanity
					if len(code.LongCode) < 20 {
						h, err := db.GenerateVerificationCodeHMAC(code.LongCode)
						if err != nil {
							return err
						}
						code.LongCode = h
						changed = true
					}

					if changed {
						if err := tx.Save(&code).Error; err != nil {
							return fmt.Errorf("failed to save code %v: %w", code.ID, err)
						}
					}
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				return nil
			},
		},
		{
			ID: "00031-AlterStatsColumns",
			Migrate: func(tx *gorm.DB) error {
				sqls := []string{
					// AuthorizedApps
					"CREATE UNIQUE INDEX IF NOT EXISTS idx_authorized_app_stats_date_authorized_app_id ON authorized_app_stats (date, authorized_app_id)",
					"DROP INDEX IF EXISTS idx_date_app_realm",
					"DROP INDEX IF EXISTS idx_authorized_app_stats_deleted_at",
					"CREATE INDEX IF NOT EXISTS idx_authorized_app_stats_date ON authorized_app_stats (date)",
					"ALTER TABLE authorized_app_stats DROP COLUMN IF EXISTS id",
					"ALTER TABLE authorized_app_stats DROP COLUMN IF EXISTS created_at",
					"ALTER TABLE authorized_app_stats DROP COLUMN IF EXISTS updated_at",
					"ALTER TABLE authorized_app_stats DROP COLUMN IF EXISTS deleted_at",
					"ALTER TABLE authorized_app_stats DROP COLUMN IF EXISTS realm_id",
					"ALTER TABLE authorized_app_stats ALTER COLUMN date TYPE date",
					"ALTER TABLE authorized_app_stats ALTER COLUMN date SET NOT NULL",
					"ALTER TABLE authorized_app_stats ALTER COLUMN authorized_app_id SET NOT NULL",
					"ALTER TABLE authorized_app_stats ALTER COLUMN codes_issued TYPE INTEGER",
					"ALTER TABLE authorized_app_stats ALTER COLUMN codes_issued SET DEFAULT 0",
					"ALTER TABLE authorized_app_stats ALTER COLUMN codes_issued SET NOT NULL",

					// Users
					"CREATE UNIQUE INDEX IF NOT EXISTS idx_user_stats_date_realm_id_user_id ON user_stats (date, realm_id, user_id)",
					"DROP INDEX IF EXISTS idx_date_user_realm",
					"DROP INDEX IF EXISTS idx_user_stats_deleted_at",
					"CREATE INDEX IF NOT EXISTS idx_user_stats_date ON user_stats (date)",
					"ALTER TABLE user_stats DROP COLUMN IF EXISTS id",
					"ALTER TABLE user_stats DROP COLUMN IF EXISTS created_at",
					"ALTER TABLE user_stats DROP COLUMN IF EXISTS updated_at",
					"ALTER TABLE user_stats DROP COLUMN IF EXISTS deleted_at",
					"ALTER TABLE user_stats ALTER COLUMN date TYPE date",
					"ALTER TABLE user_stats ALTER COLUMN date SET NOT NULL",
					"ALTER TABLE user_stats ALTER COLUMN realm_id SET NOT NULL",
					"ALTER TABLE user_stats ALTER COLUMN user_id SET NOT NULL",
					"ALTER TABLE user_stats ALTER COLUMN codes_issued TYPE INTEGER",
					"ALTER TABLE user_stats ALTER COLUMN codes_issued SET DEFAULT 0",
					"ALTER TABLE user_stats ALTER COLUMN codes_issued SET NOT NULL",
				}

				for _, sql := range sqls {
					if err := tx.Exec(sql).Error; err != nil {
						return fmt.Errorf("%s: %w", sql, err)
					}
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				return nil
			},
		},
		{
			ID: "00032-RegionCodeSize",
			Migrate: func(tx *gorm.DB) error {
				sqls := []string{
					"ALTER TABLE realms ALTER COLUMN region_code TYPE varchar(10)",
				}

				for _, sql := range sqls {
					if err := tx.Exec(sql).Error; err != nil {
						return err
					}
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				return nil
			},
		},
		{
			ID: "00033-PerlRealmSigningKeys",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(&Realm{}).Error; err != nil {
					return err
				}
				if err := tx.AutoMigrate(&SigningKey{}).Error; err != nil {
					return err
				}

				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				// SigningKeys table left in place so references to crypto keys aren't lost.
				return nil
			},
		},
		{
			ID: "00034-AddENExpressSettings",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(&Realm{}).Error; err != nil {
					return err
				}

				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				return nil
			},
		},
		{
			ID: "00035-AddMFARequiredToRealms",
			Migrate: func(tx *gorm.DB) error {
				return tx.Exec("ALTER TABLE realms ADD COLUMN IF NOT EXISTS mfa_mode INTEGER DEFAULT 0").Error
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Exec("ALTER TABLE realms DROP COLUMN IF EXISTS mfa_mode").Error
			},
		},
		{
			ID: "00036-AddRealmStats",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(&RealmStat{}).Error; err != nil {
					return err
				}
				statements := []string{
					"CREATE UNIQUE INDEX IF NOT EXISTS idx_realm_stats_stats_date_realm_id ON realm_stats (date, realm_id)",
					"CREATE INDEX IF NOT EXISTS idx_realm_stats_date ON realm_stats (date)",
				}
				for _, sql := range statements {
					if err := tx.Exec(sql).Error; err != nil {
						return err
					}
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				if err := tx.DropTable(&RealmStat{}).Error; err != nil {
					return err
				}
				return nil
			},
		},
		{
			ID: "00037-AddRealmRequireDate",
			Migrate: func(tx *gorm.DB) error {
				return tx.Exec("ALTER TABLE realms ADD COLUMN IF NOT EXISTS require_date bool DEFAULT false").Error
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Exec("ALTER TABLE realms DROP COLUMN IF EXISTS require_date").Error
			},
		},
		{
			ID: "00038-AddRealmRequireDateNotNull",
			Migrate: func(tx *gorm.DB) error {
				return tx.Exec("ALTER TABLE realms ALTER COLUMN require_date SET NOT NULL").Error
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Exec("ALTER TABLE realms ALTER COLUMN require_date SET NULL").Error
			},
		},
		{
			ID: "00039-RealmStatsToDate",
			Migrate: func(tx *gorm.DB) error {
				return tx.Exec("ALTER TABLE realm_stats ALTER COLUMN date TYPE date").Error
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Exec("ALTER TABLE realm_stats ALTER COLUMN date TYPE timestamp with time zone").Error
			},
		},
		{
			ID: "00040-BackfillRealmStats",
			Migrate: func(tx *gorm.DB) error {
				sqls := []string{
					`
					INSERT INTO realm_stats (
						SELECT date, realm_id, SUM(codes_issued) AS codes_issued
  					FROM user_stats
						WHERE user_stats.date < date('2020-09-15')
  					GROUP BY 1, 2
  					ORDER BY 1 DESC
					) ON CONFLICT(date, realm_id) DO UPDATE
						SET codes_issued = realm_stats.codes_issued + excluded.codes_issued
					`,
					`
					INSERT INTO realm_stats (
						SELECT date, authorized_apps.realm_id AS realm_id, SUM(codes_issued) AS codes_issued
						FROM authorized_app_stats
						JOIN authorized_apps
						ON authorized_app_stats.authorized_app_id = authorized_apps.id
						WHERE authorized_app_stats.date < date('2020-09-15')
						GROUP BY 1, 2
						ORDER BY 1 DESC
					) ON CONFLICT(date, realm_id) DO UPDATE
						SET codes_issued = realm_stats.codes_issued + excluded.codes_issued
					`,
				}

				for _, sql := range sqls {
					if err := tx.Exec(sql).Error; err != nil {
						return err
					}
				}

				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				return nil
			},
		},
		{
			ID: "00041-AddRealmAbusePrevention",
			Migrate: func(tx *gorm.DB) error {
				sqls := []string{
					`ALTER TABLE realms ADD COLUMN IF NOT EXISTS abuse_prevention_enabled bool NOT NULL DEFAULT false`,
					`ALTER TABLE realms ADD COLUMN IF NOT EXISTS abuse_prevention_limit integer NOT NULL DEFAULT 100`,
					`ALTER TABLE realms ADD COLUMN IF NOT EXISTS abuse_prevention_limit_factor numeric(8, 5) NOT NULL DEFAULT 1.0`,
				}

				for _, sql := range sqls {
					if err := tx.Exec(sql).Error; err != nil {
						return err
					}
				}

				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				sqls := []string{
					`ALTER TABLE realms DROP COLUMN IF EXISTS abuse_prevention_enabled`,
					`ALTER TABLE realms DROP COLUMN IF EXISTS abuse_prevention_limit`,
					`ALTER TABLE realms DROP COLUMN IF EXISTS abuse_prevention_limit_factor`,
				}

				for _, sql := range sqls {
					if err := tx.Exec(sql).Error; err != nil {
						return err
					}
				}

				return nil
			},
		},
		{
			ID: "00042-ChangeRealmAbusePreventionLimitDefault",
			Migrate: func(tx *gorm.DB) error {
				sqls := []string{
					`ALTER TABLE realms ALTER COLUMN abuse_prevention_limit SET DEFAULT 10`,
				}

				for _, sql := range sqls {
					if err := tx.Exec(sql).Error; err != nil {
						return err
					}
				}

				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				sqls := []string{
					`ALTER TABLE realms ALTER COLUMN abuse_prevention_limit SET DEFAULT 100`,
				}

				for _, sql := range sqls {
					if err := tx.Exec(sql).Error; err != nil {
						return err
					}
				}

				return nil
			},
		},
		{
			ID: "00043-CreateModelerStatus",
			Migrate: func(tx *gorm.DB) error {
				// ModelerStatus is the legacy definition that existed when this
				// migration first existed.
				type ModelerStatus struct {
					ID        uint `gorm:"primary_key"`
					NotBefore time.Time
				}

				if err := tx.AutoMigrate(&ModelerStatus{}).Error; err != nil {
					return err
				}
				if err := tx.Create(&ModelerStatus{}).Error; err != nil {
					return err
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.DropTable("modeler_statuses").Error
			},
		},
		{
			ID: "00044-AddEmailVerifiedRequiredToRealms",
			Migrate: func(tx *gorm.DB) error {
				return tx.Exec("ALTER TABLE realms ADD COLUMN IF NOT EXISTS email_verified_mode INTEGER DEFAULT 0").Error
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Exec("ALTER TABLE realms DROP COLUMN IF EXISTS email_verified_mode").Error
			},
		},
		{
			ID: "00045-BootstrapSystemAdmin",
			Migrate: func(tx *gorm.DB) error {
				// Only create the default system admin if there are no users. This
				// ensures people who are already running a system don't get a random
				// admin user.
				var user User
				err := db.db.Model(&User{}).First(&user).Error
				if err == nil {
					return nil
				}
				if !IsNotFound(err) {
					return err
				}

				user = User{
					Name:        "System admin",
					Email:       "super@example.com",
					SystemAdmin: true,
				}
				if err := tx.Save(&user).Error; err != nil {
					return err
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				return nil
			},
		},
		{
			ID: "00046-AddWelcomeMessageToRealm",
			Migrate: func(tx *gorm.DB) error {
				return tx.Exec("ALTER TABLE realms ADD COLUMN IF NOT EXISTS welcome_message text").Error
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Exec("ALTER TABLE realms DROP COLUMN IF EXISTS welcome_message").Error
			},
		},
		{
			ID: "00047-AddPasswordLastChangedToUsers",
			Migrate: func(tx *gorm.DB) error {
				return tx.Exec("ALTER TABLE users ADD COLUMN IF NOT EXISTS last_password_change DATE DEFAULT CURRENT_DATE").Error
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Exec("ALTER TABLE users DROP COLUMN IF EXISTS last_password_change").Error
			},
		},
		{
			ID: "00048-AddPasswordRotateToRealm",
			Migrate: func(tx *gorm.DB) error {
				sqls := []string{
					`ALTER TABLE realms ADD COLUMN IF NOT EXISTS password_rotation_period_days integer NOT NULL DEFAULT 0`,
					`ALTER TABLE realms ADD COLUMN IF NOT EXISTS password_rotation_warning_days integer NOT NULL DEFAULT 0`,
				}

				for _, sql := range sqls {
					if err := tx.Exec(sql).Error; err != nil {
						return err
					}
				}

				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				sqls := []string{
					`ALTER TABLE realms DROP COLUMN IF EXISTS password_rotation_period_days`,
					`ALTER TABLE realms DROP COLUMN IF EXISTS password_rotation_warning_days `,
				}

				for _, sql := range sqls {
					if err := tx.Exec(sql).Error; err != nil {
						return err
					}
				}

				return nil
			},
		},
		{
			ID: "00049-MakeRegionCodeUnique",
			Migrate: func(tx *gorm.DB) error {
				sqls := []string{
					// Make region code case insensitive and unique.
					"ALTER TABLE realms ALTER COLUMN region_code TYPE CITEXT",
					"ALTER TABLE realms ALTER COLUMN region_code DROP DEFAULT",
					"ALTER TABLE realms ALTER COLUMN region_code DROP NOT NULL",
				}
				for _, sql := range sqls {
					if err := tx.Exec(sql).Error; err != nil {
						return err
					}
				}

				// Make any existing empty string region codes to NULL. Without this,
				// the new unique constraint will fail.
				if err := tx.Exec("UPDATE realms SET region_code = NULL WHERE TRIM(region_code) = ''").Error; err != nil {
					return err
				}

				// Make all region codes uppercase.
				if err := tx.Exec("UPDATE realms SET region_code = UPPER(region_code) WHERE region_code IS NOT NULL").Error; err != nil {
					return err
				}

				// Find any existing duplicate region codes - this could be combined
				// into a much larger SQL statement with the next thing, but I'm
				// optimizing for readability here.
				var dupRegionCodes []string
				if err := tx.Model(&Realm{}).
					Unscoped().
					Select("UPPER(region_code) AS region_code").
					Where("region_code IS NOT NULL").
					Group("region_code").
					Having("COUNT(*) > 1").
					Pluck("region_code", &dupRegionCodes).
					Error; err != nil {
					return err
				}

				// Update any duplicate regions to not be duplicate anymore.
				for _, dupRegionCode := range dupRegionCodes {
					logger.Warn("de-duplicating region code %q", dupRegionCode)

					// I call this the "Microsoft method". For each duplicate realm,
					// append -N, starting with 1. If there are 3 realms with the region
					// code "PA", their new values will be "PA", "PA-1", and "PA-2"
					// respectively.
					sql := `
						UPDATE
							realms
						SET region_code = CONCAT(realms.region_code, '-', (z-1)::text)
						FROM (
							SELECT
								id,
								region_code,
								ROW_NUMBER() OVER (ORDER BY id ASC) AS z
							FROM realms
							WHERE UPPER(region_code) = UPPER($1)
						) AS sq
						WHERE realms.id = sq.id AND sq.z > 1`
					if err := tx.Exec(sql, dupRegionCode).Error; err != nil {
						return err
					}
				}

				sqls = []string{
					// There's already a runtime constraint and validation on names, this
					// is just an extra layer of protection at the database layer.
					"ALTER TABLE realms ALTER COLUMN name SET NOT NULL",

					// Alter the unique index on realm names to be a column constraint.
					"DROP INDEX IF EXISTS uix_realms_name",
					"ALTER TABLE realms ADD CONSTRAINT uix_realms_name UNIQUE (name)",

					// Now finally add a unique constraint on region codes.
					"ALTER TABLE realms ADD CONSTRAINT uix_realms_region_code UNIQUE (region_code)",
				}

				for _, sql := range sqls {
					if err := tx.Exec(sql).Error; err != nil {
						return err
					}
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				return nil
			},
		},
		{
			ID: "00050-CreateMobileApps",
			Migrate: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&MobileApp{}).Error
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.DropTable("mobile_apps").Error
			},
		},
		{
			ID: "00051-CreateSystemSMSConfig",
			Migrate: func(tx *gorm.DB) error {
				sqls := []string{
					// Add a new is_system boolean column and a constraint to ensure that
					// only one row can have a value of true.
					`ALTER TABLE sms_configs ADD COLUMN IF NOT EXISTS is_system BOOL`,
					`UPDATE sms_configs SET is_system = FALSE WHERE is_system IS NULL`,
					`ALTER TABLE sms_configs ALTER COLUMN is_system SET DEFAULT FALSE`,
					`ALTER TABLE sms_configs ALTER COLUMN is_system SET NOT NULL`,
					`CREATE UNIQUE INDEX IF NOT EXISTS uix_sms_configs_is_system_true ON sms_configs (is_system) WHERE (is_system IS TRUE)`,

					// Require realm_id be set on all rows except system configs, and
					// ensure that realm_id is unique.
					`ALTER TABLE sms_configs DROP CONSTRAINT IF EXISTS nn_sms_configs_realm_id`,
					`DROP INDEX IF EXISTS nn_sms_configs_realm_id`,
					`ALTER TABLE sms_configs ADD CONSTRAINT nn_sms_configs_realm_id CHECK (is_system IS TRUE OR realm_id IS NOT NULL)`,

					`ALTER TABLE sms_configs DROP CONSTRAINT IF EXISTS uix_sms_configs_realm_id`,
					`DROP INDEX IF EXISTS uix_sms_configs_realm_id`,
					`ALTER TABLE sms_configs ADD CONSTRAINT uix_sms_configs_realm_id UNIQUE (realm_id)`,

					// Realm option set by system admins to share the system SMS config
					// with the realm.
					`ALTER TABLE realms ADD COLUMN IF NOT EXISTS can_use_system_sms_config BOOL`,
					`UPDATE realms SET can_use_system_sms_config = FALSE WHERE can_use_system_sms_config IS NULL`,
					`ALTER TABLE realms ALTER COLUMN can_use_system_sms_config SET DEFAULT FALSE`,
					`ALTER TABLE realms ALTER COLUMN can_use_system_sms_config SET NOT NULL`,

					// If true, the realm is set to use the system SMS config.
					`ALTER TABLE realms ADD COLUMN IF NOT EXISTS use_system_sms_config BOOL`,
					`UPDATE realms SET use_system_sms_config = FALSE WHERE use_system_sms_config IS NULL`,
					`ALTER TABLE realms ALTER COLUMN use_system_sms_config SET DEFAULT FALSE`,
					`ALTER TABLE realms ALTER COLUMN use_system_sms_config SET NOT NULL`,
				}

				for _, sql := range sqls {
					if err := tx.Exec(sql).Error; err != nil {
						return err
					}
				}

				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				sqls := []string{
					`ALTER TABLE sms_configs DROP COLUMN IF EXISTS is_system`,
					`DROP INDEX IF EXISTS uix_sms_configs_is_system_true`,
					`ALTER TABLE sms_configs DROP CONSTRAINT IF EXISTS nn_sms_configs_realm_id`,
					`ALTER TABLE sms_configs DROP CONSTRAINT IF EXISTS uix_sms_configs_realm_id`,

					`ALTER TABLE realms DROP COLUMN IF EXISTS can_use_system_sms_config`,
					`ALTER TABLE realms DROP COLUMN IF EXISTS use_system_sms_config`,
				}

				for _, sql := range sqls {
					if err := tx.Exec(sql).Error; err != nil {
						return err
					}
				}

				return nil
			},
		},
		{
			ID: "00052-CreateRealmAllowedCIDRs",
			Migrate: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&Realm{}).Error
			},
			Rollback: func(tx *gorm.DB) error {
				sqls := []string{
					`ALTER TABLE realms DROP COLUMN IF EXISTS allowed_cidrs_adminapi`,
					`ALTER TABLE realms DROP COLUMN IF EXISTS allowed_cidrs_apiserver`,
					`ALTER TABLE realms DROP COLUMN IF EXISTS allowed_cidrs_server`,
				}

				for _, sql := range sqls {
					if err := tx.Exec(sql).Error; err != nil {
						return err
					}
				}

				return nil
			},
		},
		{
			ID: "00053-AddRealmSMSCountry",
			Migrate: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&Realm{}).Error
			},
			Rollback: func(tx *gorm.DB) error {
				sqls := []string{
					`ALTER TABLE realms DROP COLUMN IF EXISTS sms_country`,
				}

				for _, sql := range sqls {
					if err := tx.Exec(sql).Error; err != nil {
						return err
					}
				}

				return nil
			},
		},
		{
			ID: "00054-ChangeMobileAppNameUniqueness",
			Migrate: func(tx *gorm.DB) error {
				sqls := []string{
					`DROP INDEX IF EXISTS realm_app_name`,
					`ALTER TABLE mobile_apps ADD CONSTRAINT uix_name_realm_id_os UNIQUE (name, realm_id, os)`,
				}

				for _, sql := range sqls {
					if err := tx.Exec(sql).Error; err != nil {
						return err
					}
				}

				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				sqls := []string{
					`CREATE UNIQUE INDEX realm_app_name ON mobile_apps (name, realm_id)`,
					`ALTER TABLE mobile_apps DROP CONSTRAINT uix_name_realm_id_os UNIQUE (name, realm_id, os)`,
				}

				for _, sql := range sqls {
					if err := tx.Exec(sql).Error; err != nil {
						return err
					}
				}

				return nil
			},
		},
		{
			ID: "00055-AddAuditEntries",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(&AuditEntry{}).Error; err != nil {
					return err
				}

				sqls := []string{
					`CREATE INDEX IF NOT EXISTS idx_audit_entries_realm_id ON audit_entries (realm_id)`,
					`CREATE INDEX IF NOT EXISTS idx_audit_entries_actor_id ON audit_entries (actor_id)`,
					`CREATE INDEX IF NOT EXISTS idx_audit_entries_target_id ON audit_entries (target_id)`,
					`CREATE INDEX IF NOT EXISTS idx_audit_entries_created_at ON audit_entries (created_at)`,
				}

				for _, sql := range sqls {
					if err := tx.Exec(sql).Error; err != nil {
						return err
					}
				}

				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Exec(`DROP TABLE audit_entries`).Error
			},
		},
		{
			ID: "00056-AuthorzedAppsAPIKeyTypeBasis",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(&AuditEntry{}).Error; err != nil {
					return err
				}

				sqls := []string{
					`ALTER TABLE authorized_apps ALTER COLUMN api_key_type DROP DEFAULT`,
					`ALTER TABLE authorized_apps ALTER COLUMN api_key_type SET NOT NULL`,
				}

				for _, sql := range sqls {
					if err := tx.Exec(sql).Error; err != nil {
						return err
					}
				}

				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				return nil
			},
		},
		{
			ID: "00057-AddMFARequiredGracePeriod",
			Migrate: func(tx *gorm.DB) error {
				return tx.Exec("ALTER TABLE realms ADD COLUMN IF NOT EXISTS mfa_required_grace_period BIGINT DEFAULT 0").Error
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Exec("ALTER TABLE realms DROP COLUMN IF EXISTS mfa_required_grace_period").Error
			},
		},
		{
			ID: "00058-AddAppStoreLink",
			Migrate: func(tx *gorm.DB) error {
				return tx.Exec("ALTER TABLE mobile_apps ADD COLUMN IF NOT EXISTS url TEXT").Error
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Exec("ALTER TABLE realms DROP COLUMN IF EXISTS url").Error
			},
		},
		{
			ID: "00059-AddVerCodeIndexes",
			Migrate: func(tx *gorm.DB) error {
				sqls := []string{
					`CREATE INDEX IF NOT EXISTS idx_vercode_recent ON verification_codes(realm_id, issuing_user_id)`,
				}

				for _, sql := range sqls {
					if err := tx.Exec(sql).Error; err != nil {
						return err
					}
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				sqls := []string{
					`DROP INDEX IF EXISTS idx_vercode_recent`,
				}

				for _, sql := range sqls {
					if err := tx.Exec(sql).Error; err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			ID: "00060-AddEmailConfig",
			Migrate: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&EmailConfig{}).Error
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.DropTable("email_configs").Error
			},
		},
		{
			ID: "00061-CreateSystemEmailConfig",
			Migrate: func(tx *gorm.DB) error {
				sqls := []string{
					// Add a new is_system boolean column and a constraint to ensure that
					// only one row can have a value of true.
					`CREATE UNIQUE INDEX IF NOT EXISTS uix_email_configs_is_system_true ON email_configs (is_system) WHERE (is_system IS TRUE)`,

					// Require realm_id be set on all rows except system configs, and
					// ensure that realm_id is unique.
					`ALTER TABLE email_configs DROP CONSTRAINT IF EXISTS nn_email_configs_realm_id`,
					`DROP INDEX IF EXISTS nn_email_configs_realm_id`,
					`ALTER TABLE email_configs ADD CONSTRAINT nn_email_configs_realm_id CHECK (is_system IS TRUE OR realm_id IS NOT NULL)`,

					`ALTER TABLE email_configs DROP CONSTRAINT IF EXISTS uix_email_configs_realm_id`,
					`DROP INDEX IF EXISTS uix_email_configs_realm_id`,
					`ALTER TABLE email_configs ADD CONSTRAINT uix_email_configs_realm_id UNIQUE (realm_id)`,

					// Realm option set by system admins to share the system Email config
					// with the realm.
					`ALTER TABLE realms ADD COLUMN IF NOT EXISTS can_use_system_email_config BOOL`,
					`UPDATE realms SET can_use_system_email_config = FALSE WHERE can_use_system_email_config IS NULL`,
					`ALTER TABLE realms ALTER COLUMN can_use_system_email_config SET DEFAULT FALSE`,
					`ALTER TABLE realms ALTER COLUMN can_use_system_email_config SET NOT NULL`,

					// If true, the realm is set to use the system Email config.
					`ALTER TABLE realms ADD COLUMN IF NOT EXISTS use_system_email_config BOOL`,
					`UPDATE realms SET use_system_email_config = FALSE WHERE use_system_email_config IS NULL`,
					`ALTER TABLE realms ALTER COLUMN use_system_email_config SET DEFAULT FALSE`,
					`ALTER TABLE realms ALTER COLUMN use_system_email_config SET NOT NULL`,

					// Add templates
					`ALTER TABLE realms ADD COLUMN IF NOT EXISTS email_invite_template text`,
					`ALTER TABLE realms ADD COLUMN IF NOT EXISTS email_password_reset_template text`,
					`ALTER TABLE realms ADD COLUMN IF NOT EXISTS email_verify_template text`,
				}

				for _, sql := range sqls {
					if err := tx.Exec(sql).Error; err != nil {
						return err
					}
				}

				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				sqls := []string{
					`ALTER TABLE email_configs DROP COLUMN IF EXISTS is_system`,
					`DROP INDEX IF EXISTS uix_email_configs_is_system_true`,
					`ALTER TABLE email_configs DROP CONSTRAINT IF EXISTS nn_email_configs_realm_id`,
					`ALTER TABLE email_configs DROP CONSTRAINT IF EXISTS uix_email_configs_realm_id`,

					`ALTER TABLE realms DROP COLUMN IF EXISTS can_use_system_email_config`,
					`ALTER TABLE realms DROP COLUMN IF EXISTS use_system_email_config`,

					`ALTER TABLE realms DROP COLUMN IF EXISTS email_invite_template`,
					`ALTER TABLE realms DROP COLUMN IF EXISTS email_password_reset_template`,
					`ALTER TABLE realms DROP COLUMN IF EXISTS email_verify_template`,
				}

				for _, sql := range sqls {
					if err := tx.Exec(sql).Error; err != nil {
						return err
					}
				}

				return nil
			},
		},
		{
			ID: "00062-AddTestDate",
			Migrate: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&VerificationCode{}).Error
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Exec("ALTER TABLE verification_codes DROP COLUMN IF EXISTS test_date").Error
			},
		},
		{
			ID: "00063-AddTokenTestDate",
			Migrate: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&Token{}).Error
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Exec("ALTER TABLE tokens DROP COLUMN IF EXISTS test_date").Error
			},
		},
		{
			ID: "00064-RescopeVerificationCodeIndices",
			Migrate: func(tx *gorm.DB) error {
				sqls := []string{
					"ALTER TABLE verification_codes ALTER COLUMN long_code DROP NOT NULL",
					fmt.Sprintf("CREATE UNIQUE INDEX IF NOT EXISTS %s ON verification_codes (realm_id,code) WHERE code != ''", VerCodesCodeUniqueIndex),
					fmt.Sprintf("CREATE UNIQUE INDEX IF NOT EXISTS %s ON verification_codes (realm_id,long_code) WHERE long_code != ''", VerCodesLongCodeUniqueIndex),
					"DROP INDEX IF EXISTS uix_verification_codes_code",
					"DROP INDEX IF EXISTS uix_verification_codes_long_code",
				}

				for _, sql := range sqls {
					if err := tx.Exec(sql).Error; err != nil {
						return err
					}
				}

				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				sqls := []string{
					"DELETE FROM verification_codes WHERE code = '' or long_code = ''",
				}

				for _, sql := range sqls {
					if err := tx.Exec(sql).Error; err != nil {
						return err
					}
				}

				return nil
			},
		},
		{
			ID: "00065-RenameUserAdminToSystemAdmin",
			Migrate: func(tx *gorm.DB) error {
				sqls := []string{
					`
					DO $$
					BEGIN
						IF EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name = 'users' AND column_name = 'admin')
						THEN
							ALTER TABLE users RENAME COLUMN admin TO system_admin;
						END IF;
					END $$;
					`,

					`CREATE INDEX idx_users_system_admin ON users(system_admin)`,
				}

				for _, sql := range sqls {
					if err := tx.Exec(sql).Error; err != nil {
						return err
					}
				}

				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				sqls := []string{
					`ALTER TABLE users RENAME COLUMN system_admin TO admin`,
					`DROP INDEX IF EXISTS idx_users_system_admin`,
				}

				for _, sql := range sqls {
					if err := tx.Exec(sql).Error; err != nil {
						return err
					}
				}

				return nil
			},
		},
		{
			ID: "00066-AddVerCodeUUIDUniqueIndexe",
			Migrate: func(tx *gorm.DB) error {
				sqls := []string{
					`DROP INDEX IF EXISTS idx_vercode_uuid`,
					`CREATE UNIQUE INDEX IF NOT EXISTS idx_vercode_uuid_unique ON verification_codes(uuid)`,
				}

				for _, sql := range sqls {
					if err := tx.Exec(sql).Error; err != nil {
						return err
					}
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				sqls := []string{
					`DROP INDEX IF EXISTS idx_vercode_uuid_unique`,
					`CREATE INDEX IF NOT EXISTS idx_vercode_uuid ON verification_codes(uuid)`,
				}

				for _, sql := range sqls {
					if err := tx.Exec(sql).Error; err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			ID: "00067-AddRealmAllowBulkUpload",
			Migrate: func(tx *gorm.DB) error {
				sqls := []string{
					`ALTER TABLE realms ADD COLUMN IF NOT EXISTS allow_bulk_upload BOOL`,
					`UPDATE realms SET allow_bulk_upload = FALSE WHERE allow_bulk_upload IS NULL`,
					`ALTER TABLE realms ALTER COLUMN allow_bulk_upload SET DEFAULT FALSE`,
					`ALTER TABLE realms ALTER COLUMN allow_bulk_upload SET NOT NULL`,
				}

				for _, sql := range sqls {
					if err := tx.Exec(sql).Error; err != nil {
						return err
					}
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Exec("ALTER TABLE realms DROP COLUMN IF EXISTS allow_bulk_upload").Error
			},
		},
		{
			ID: "00068-EnablePGAudit",
			Migrate: func(tx *gorm.DB) error {
				_ = tx.Exec(`CREATE EXTENSION pgaudit`).Error
				return nil
			},
		},
		{
			ID: "00069-AddExternalIssuerStats",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(&ExternalIssuerStat{}).Error; err != nil {
					return err
				}
				sql := `CREATE UNIQUE INDEX IF NOT EXISTS idx_external_issuer_stats_date_issuer_id_realm_id ON external_issuer_stats (date, issuer_id, realm_id)`
				return tx.Exec(sql).Error
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.DropTable(&ExternalIssuerStat{}).Error
			},
		},
		{
			ID: "00070-AddExternalIssuerIDToVerificationCode",
			Migrate: func(tx *gorm.DB) error {
				sql := `ALTER TABLE verification_codes ADD COLUMN IF NOT EXISTS issuing_external_id VARCHAR(255)`
				return tx.Exec(sql).Error
			},
			Rollback: func(tx *gorm.DB) error {
				sql := `ALTER TABLE verification_codes DROP COLUMN IF EXISTS issuing_external_id`
				return tx.Exec(sql).Error
			},
		},
		{
			ID: "00071-AddDailyActiveUsersToRealmStats",
			Migrate: func(tx *gorm.DB) error {
				sql := `ALTER TABLE realm_stats ADD COLUMN IF NOT EXISTS daily_active_users INTEGER DEFAULT 0`
				return tx.Exec(sql).Error
			},
			Rollback: func(tx *gorm.DB) error {
				sql := `ALTER TABLE realm_stats DROP COLUMN IF EXISTS daily_active_users`
				return tx.Exec(sql).Error
			},
		},
		{
			ID: "00072-ChangeSMSTemplateType",
			Migrate: func(tx *gorm.DB) error {
				return tx.Exec("ALTER TABLE realms ALTER COLUMN sms_text_template TYPE text").Error
			},
			Rollback: func(tx *gorm.DB) error {
				// No rollback for this, as there is no reason to do so.
				return nil
			},
		},
		{
			ID: "00073-AddSMSFromNumbers",
			Migrate: func(tx *gorm.DB) error {
				sqls := []string{
					`CREATE TABLE sms_from_numbers (
						id SERIAL PRIMARY KEY NOT NULL,
						label CITEXT UNIQUE,
						value TEXT UNIQUE
					)`,
					`ALTER TABLE realms ADD COLUMN IF NOT EXISTS sms_from_number_id INTEGER`,
					`ALTER TABLE realms ADD CONSTRAINT fk_sms_from_number FOREIGN KEY (sms_from_number_id) REFERENCES sms_from_numbers(id)`,
				}

				for _, sql := range sqls {
					if err := tx.Exec(sql).Error; err != nil {
						return err
					}
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				sql := `DROP TABLE IF EXISTS sms_from_numbers`
				return tx.Exec(sql).Error
			},
		},
		{
			ID: "00074-MigrateSystemSMSConfig",
			Migrate: func(tx *gorm.DB) error {
				systemSMSConfig, err := db.SystemSMSConfig()
				if err != nil {
					// There are no system sms configs, so there's no migration needed.
					if IsNotFound(err) {
						return nil
					}
					return fmt.Errorf("failed to find system sms config: %w", err)
				}

				// Create an SMS from number for that entry.
				smsFromNumber := &SMSFromNumber{
					Label: "Default",
					Value: systemSMSConfig.TwilioFromNumber,
				}
				if err := db.CreateOrUpdateSMSFromNumbers([]*SMSFromNumber{smsFromNumber}); err != nil {
					return fmt.Errorf("failed to create sms from number: %w", err)
				}

				// Update anyone using the system config.
				sql := `UPDATE realms SET sms_from_number_id = ? WHERE use_system_sms_config IS TRUE`
				return tx.Exec(sql, smsFromNumber.ID).Error
			},
			Rollback: func(tx *gorm.DB) error {
				return nil
			},
		},
		{
			ID: "00075-CreateRBAC",
			Migrate: func(tx *gorm.DB) error {
				// These tables will already exist for existing apps, but it will NOT
				// exist for apps running migrations for the first time. The reason is
				// because Gorm's automigrate created the join tables automagically in
				// the past because the models were annotated. But the models are no
				// longer annotated, so the join table is not created.
				if err := tx.Exec(`CREATE TABLE IF NOT EXISTS user_realms (
					realm_id INTEGER,
					user_id INTEGER,
					PRIMARY KEY (realm_id, user_id)
				)`).Error; err != nil {
					return err
				}
				if err := tx.Exec(`CREATE TABLE IF NOT EXISTS admin_realms (
					realm_id INTEGER,
					user_id INTEGER,
					PRIMARY KEY (realm_id, user_id)
				)`).Error; err != nil {
					return err
				}

				// We are about to add foreign key references for these fields. However,
				// in the past, it was possible for de-association to occur. We need to
				// delete any orphaned user_realm and admin_realm associations so the
				// foreign key constraint can be properly applied in the next step.
				if err := tx.Exec(`DELETE FROM user_realms ur WHERE NOT EXISTS (SELECT FROM users u WHERE u.id = ur.user_id)`).Error; err != nil {
					return err
				}
				if err := tx.Exec(`DELETE FROM admin_realms ar WHERE NOT EXISTS (SELECT FROM users u WHERE u.id = ar.user_id)`).Error; err != nil {
					return err
				}

				// Update existing columns defaults.
				if err := tx.Exec(`ALTER TABLE user_realms ALTER COLUMN realm_id SET NOT NULL`).Error; err != nil {
					return err
				}
				if err := tx.Exec(`ALTER TABLE user_realms ALTER COLUMN user_id SET NOT NULL`).Error; err != nil {
					return err
				}

				// Convert realm and user into real foreign keys. If the parent record
				// is deleted, also delete the membership record.
				if err := tx.Exec(`ALTER TABLE user_realms DROP CONSTRAINT IF EXISTS fk_realm`).Error; err != nil {
					return err
				}
				if err := tx.Exec(`ALTER TABLE user_realms ADD CONSTRAINT fk_realm FOREIGN KEY (realm_id) REFERENCES realms(id) ON DELETE CASCADE`).Error; err != nil {
					return err
				}
				if err := tx.Exec(`ALTER TABLE user_realms DROP CONSTRAINT IF EXISTS fk_user`).Error; err != nil {
					return err
				}
				if err := tx.Exec(`ALTER TABLE user_realms ADD CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE`).Error; err != nil {
					return err
				}

				// Create new column in user_realms.
				if err := tx.Exec(`ALTER TABLE user_realms ADD COLUMN IF NOT EXISTS permissions BIGINT`).Error; err != nil {
					return err
				}

				// Update all existing permissions to be RealmUser.
				if err := tx.Exec(
					`UPDATE user_realms SET permissions = $1`,
					int64(rbac.LegacyRealmUser),
				).Error; err != nil {
					return err
				}

				// Set the default for new permissions to 0 (none) and disallow NULL.
				if err := tx.Exec(`ALTER TABLE user_realms ALTER COLUMN permissions SET DEFAULT 0`).Error; err != nil {
					return err
				}
				if err := tx.Exec(`ALTER TABLE user_realms ALTER COLUMN permissions SET NOT NULL`).Error; err != nil {
					return err
				}

				// Migrate existing admin_realms permissions into user_realms with the
				// legacy realm admin permission.
				if err := tx.Exec(
					`INSERT INTO user_realms(user_id, realm_id, permissions)
						SELECT user_id, realm_id, $1 FROM admin_realms
						ON CONFLICT (user_id, realm_id) DO UPDATE SET permissions = $1`,
					int64(rbac.LegacyRealmAdmin|rbac.LegacyRealmUser),
				).Error; err != nil {
					return err
				}

				// Rename user_realms.
				if err := tx.Exec(`ALTER TABLE user_realms RENAME TO memberships`).Error; err != nil {
					return err
				}
				if err := tx.Exec(`ALTER INDEX user_realms_pkey RENAME TO memberships_pkey`).Error; err != nil {
					return err
				}

				// Add indexes on realm_id and user_id individually for lookups.
				if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_memberships_realm_id ON memberships(realm_id)`).Error; err != nil {
					return err
				}
				if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_memberships_user_id ON memberships(user_id)`).Error; err != nil {
					return err
				}

				// Drop admin_realms.
				if err := tx.Exec(`DROP TABLE IF EXISTS admin_realms`).Error; err != nil {
					return err
				}

				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				// No rollback for this
				return nil
			},
		},
		{
			ID: "00076-EnableExtension_hstore",
			Migrate: func(tx *gorm.DB) error {
				return tx.Exec("CREATE EXTENSION IF NOT EXISTS hstore").Error
			},
		},
		{
			ID: "00077-AddAlternateSMSTemplates",
			Migrate: func(tx *gorm.DB) error {
				return tx.Exec(`ALTER TABLE realms ADD COLUMN IF NOT EXISTS alternate_sms_templates hstore`).Error
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Exec(`ALTER TABLE realms DROP COLUMN IF EXISTS alternate_sms_templates`).Error
			},
		},
		{
			ID: "00078-AddEnableDailyActiveUsersToRealm",
			Migrate: func(tx *gorm.DB) error {
				return tx.Exec(`ALTER TABLE realms ADD COLUMN IF NOT EXISTS daily_active_users_enabled BOOL DEFAULT false NOT NULL`).Error
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Exec(`ALTER TABLE realms DROP COLUMN IF EXISTS daily_active_users_enabled`).Error
			},
		},
		{
			ID: "00079-AddMembershipsDefaultSMSTemplate",
			Migrate: func(tx *gorm.DB) error {
				sqls := []string{
					`ALTER TABLE memberships ADD COLUMN IF NOT EXISTS default_sms_template_label VARCHAR(255)`,
				}

				for _, sql := range sqls {
					if err := tx.Exec(sql).Error; err != nil {
						return err
					}
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				sqls := []string{
					`ALTER TABLE memberships DROP COLUMN IF EXISTS default_sms_template_label`,
				}

				for _, sql := range sqls {
					if err := tx.Exec(sql).Error; err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			ID: "00080-AddDisableRedirectToMobileApps",
			Migrate: func(tx *gorm.DB) error {
				return multiExec(tx,
					`ALTER TABLE mobile_apps ADD COLUMN IF NOT EXISTS disable_redirect BOOL DEFAULT false NOT NULL`)
			},
			Rollback: func(tx *gorm.DB) error {
				return multiExec(tx,
					`ALTER TABLE mobile_apps DROP COLUMN IF EXISTS disable_redirect`)
			},
		},
		{
			ID: "00081-AddTimestampsToMemberships",
			Migrate: func(tx *gorm.DB) error {
				return multiExec(tx,
					`ALTER TABLE memberships ADD COLUMN IF NOT EXISTS created_at TIMESTAMP WITH TIME ZONE`,
					`ALTER TABLE memberships ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP WITH TIME ZONE`,
					`UPDATE memberships SET created_at = NOW() WHERE created_at IS NULL`,
					`UPDATE memberships SET updated_at = NOW() WHERE updated_at IS NULL`)
			},
			Rollback: func(tx *gorm.DB) error {
				return multiExec(tx,
					`ALTER TABLE memberships DROP COLUMN IF EXISTS created_at`,
					`ALTER TABLE memberships DROP COLUMN IF EXISTS updated_at`)
			},
		},
		{
			ID: "00082-AddMoreStats",
			Migrate: func(tx *gorm.DB) error {
				return multiExec(tx,
					`ALTER TABLE realm_stats ADD COLUMN IF NOT EXISTS codes_invalid INTEGER NOT NULL DEFAULT 0`,
					`ALTER TABLE realm_stats ADD COLUMN IF NOT EXISTS tokens_claimed INTEGER NOT NULL DEFAULT 0`,
					`ALTER TABLE realm_stats ADD COLUMN IF NOT EXISTS tokens_invalid INTEGER NOT NULL DEFAULT 0`,
					`ALTER TABLE authorized_app_stats ADD COLUMN IF NOT EXISTS codes_claimed INTEGER NOT NULL DEFAULT 0`,
					`ALTER TABLE authorized_app_stats ADD COLUMN IF NOT EXISTS codes_invalid INTEGER NOT NULL DEFAULT 0`,
					`ALTER TABLE authorized_app_stats ADD COLUMN IF NOT EXISTS tokens_claimed INTEGER NOT NULL DEFAULT 0`,
					`ALTER TABLE authorized_app_stats ADD COLUMN IF NOT EXISTS tokens_invalid INTEGER NOT NULL DEFAULT 0`)
			},
			Rollback: func(tx *gorm.DB) error {
				return multiExec(tx,
					`ALTER TABLE realm_stats DROP COLUMN IF EXISTS codes_invalid`,
					`ALTER TABLE realm_stats DROP COLUMN IF EXISTS tokens_claimed`,
					`ALTER TABLE realm_stats DROP COLUMN IF EXISTS tokens_invalid`,
					`ALTER TABLE authorized_app_stats DROP COLUMN IF EXISTS codes_claimed`,
					`ALTER TABLE authorized_app_stats DROP COLUMN IF EXISTS codes_invalid`,
					`ALTER TABLE authorized_app_stats DROP COLUMN IF EXISTS tokens_claimed`,
					`ALTER TABLE authorized_app_stats DROP COLUMN IF EXISTS tokens_invalid`)
			},
		},
		{
			ID: "00083-ExpandTwilioFrom",
			Migrate: func(tx *gorm.DB) error {
				return multiExec(tx,
					`ALTER TABLE sms_configs ALTER COLUMN twilio_from_number TYPE varchar(255)`)
			},
			Rollback: func(tx *gorm.DB) error {
				// No rollback for this, which would destroy data.
				return nil
			},
		},
		{
			ID: "00084-DropDAU",
			Migrate: func(tx *gorm.DB) error {
				return multiExec(tx,
					`ALTER TABLE realms DROP COLUMN IF EXISTS daily_active_users_enabled`,
					`ALTER TABLE realm_stats DROP COLUMN IF EXISTS daily_active_users`)
			},
			Rollback: func(tx *gorm.DB) error {
				return multiExec(tx,
					`ALTER TABLE realms ADD COLUMN IF NOT EXISTS daily_active_users_enabled BOOL DEFAULT false NOT NULL`,
					`ALTER TABLE realm_stats ADD COLUMN IF NOT EXISTS daily_active_users INTEGER DEFAULT 0`)
			},
		},
		{
			ID: "00085-DeleteUsers",
			Migrate: func(tx *gorm.DB) error {
				return multiExec(tx,
					`DELETE FROM users WHERE deleted_at IS NOT NULL`)
			},
		},
		{
			ID: "00086-AddRealmAutoRotateSetting",
			Migrate: func(tx *gorm.DB) error {
				return multiExec(tx,
					`ALTER TABLE realms ADD COLUMN IF NOT EXISTS auto_rotate_certificate_key BOOLEAN DEFAULT false`,
					`ALTER TABLE realms ALTER COLUMN auto_rotate_certificate_key SET NOT NULL`)
			},
			Rollback: func(tx *gorm.DB) error {
				return multiExec(tx,
					`ALTER TABLE realms DROP COLUMN IF EXISTS auto_rotate_certificate_key`)
			},
		},
		{
			ID: "00087-AddTokenSigningKeys",
			Migrate: func(tx *gorm.DB) error {
				return multiExec(tx,
					`CREATE TABLE token_signing_keys (
						id BIGSERIAL,
						key_version_id TEXT NOT NULL,
						is_active BOOL NOT NULL DEFAULT FALSE,
						created_at TIMESTAMP WITH TIME ZONE,
						updated_at TIMESTAMP WITH TIME ZONE,
						PRIMARY KEY (id))`,
					`CREATE UNIQUE INDEX uix_token_signing_keys_is_active ON token_signing_keys (is_active) WHERE (is_active IS TRUE)`,
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return multiExec(tx,
					`DROP TABLE IF EXISTS token_signing_keys`)
			},
		},
		{
			ID: "00088-AddUUIDToTokenSigningKeys",
			Migrate: func(tx *gorm.DB) error {
				return multiExec(tx,
					`ALTER TABLE token_signing_keys ADD COLUMN uuid UUID NOT NULL UNIQUE DEFAULT uuid_generate_v4()`)
			},
			Rollback: func(tx *gorm.DB) error {
				return multiExec(tx,
					`DROP TABLE IF EXISTS token_signing_keys`)
			},
		},
		{
			ID: "00089-KeyServerStats",
			Migrate: func(tx *gorm.DB) error {
				return multiExec(tx,
					`CREATE TABLE key_server_stats (
						realm_id INTEGER,
						key_server_url_override TEXT,
						key_server_audience_override TEXT,
						PRIMARY KEY (realm_id))`,
					`CREATE TABLE key_server_stats_days (
						realm_id INTEGER,
						day TIMESTAMP WITH TIME ZONE,
						publish_requests BIGINT[],
						total_teks_published BIGINT NOT NULL DEFAULT 0,
						revision_requests BIGINT NOT NULL DEFAULT 0,
						tek_age_distribution BIGINT[],
						onset_to_upload_distribution BIGINT[],
						request_missing_onset_date BIGINT NOT NULL DEFAULT 0,
						PRIMARY KEY (realm_id, day))`,
				)
			},
			Rollback: func(tx *gorm.DB) error {
				if err := tx.DropTableIfExists(&KeyServerStats{}, &KeyServerStatsDay{}).Error; err != nil {
					return err
				}
				return nil
			},
		},
		{
			ID: "00090-AddSMSSigningKeys",
			Migrate: func(tx *gorm.DB) error {
				return multiExec(tx,
					`CREATE TABLE sms_signing_keys (
						id BIGSERIAL,
						created_at TIMESTAMP WITH TIME ZONE,
						updated_at TIMESTAMP WITH TIME ZONE,
						deleted_at TIMESTAMP WITH TIME ZONE,
						realm_id INTEGER NOT NULL,
						key_id TEXT NOT NULL,
						active BOOLEAN NOT NULL DEFAULT false,
						PRIMARY KEY (id))`,
					`CREATE INDEX idx_sms_signing_keys_realm ON sms_signing_keys (realm_id)`,
					`CREATE UNIQUE INDEX uix_sms_signing_keys_active ON sms_signing_keys (realm_id, active) WHERE (active IS TRUE)`,
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return multiExec(tx,
					`DROP TABLE IF EXISTS sms_signing_keys`)
			},
		},
		{
			ID: "00091-AddRealmAAuthenticatedSMSSetting",
			Migrate: func(tx *gorm.DB) error {
				return multiExec(tx,
					`ALTER TABLE realms ADD COLUMN IF NOT EXISTS use_authenticated_sms BOOLEAN DEFAULT false`,
					`ALTER TABLE realms ALTER COLUMN use_authenticated_sms SET NOT NULL`)
			},
			Rollback: func(tx *gorm.DB) error {
				return multiExec(tx,
					`ALTER TABLE realms DROP COLUMN IF EXISTS use_authenticated_sms`)
			},
		},
		{
			ID: "00092-AddClaimDistributionToRealmStats",
			Migrate: func(tx *gorm.DB) error {
				return multiExec(tx,
					`ALTER TABLE realm_stats ADD COLUMN IF NOT EXISTS code_claim_age_distribution INTEGER[]`)
			},
			Rollback: func(tx *gorm.DB) error {
				return multiExec(tx,
					`ALTER TABLE realm_stats DROP COLUMN IF EXISTS code_claim_age_distribution`)
			},
		},
		{
			ID: "00093-AddClaimMeanAgeToRealmStats",
			Migrate: func(tx *gorm.DB) error {
				return multiExec(tx,
					`ALTER TABLE realm_stats ADD COLUMN IF NOT EXISTS code_claim_mean_age BIGINT NOT NULL DEFAULT 0`)
			},
			Rollback: func(tx *gorm.DB) error {
				return multiExec(tx,
					`ALTER TABLE realm_stats DROP COLUMN IF EXISTS code_claim_mean_age`)
			},
		},
		{
			ID: "00094-AddRealmChaffEvents",
			Migrate: func(tx *gorm.DB) error {
				return multiExec(tx,
					`CREATE TABLE realm_chaff_events (
						realm_id INTEGER NOT NULL REFERENCES realms(id),
						date DATE NOT NULL,
						PRIMARY KEY (realm_id, date)
					)`,
					`CREATE INDEX idx_realm_chaff_events_realm_id ON realm_chaff_events(realm_id)`,
					`CREATE INDEX idx_realm_chaff_events_date ON realm_chaff_events(date)`)
			},
			Rollback: func(tx *gorm.DB) error {
				return multiExec(tx,
					`DROP TABLE IF EXISTS realm_chaff_events`,
					`DROP INDEX IF EXISTS idx_realm_chaff_events_realm_id`,
					`DROP INDEX IF EXISTS idx_realm_chaff_events_date`)
			},
		},
		{
			ID: "00095-DropModelerStatuses",
			Migrate: func(tx *gorm.DB) error {
				return multiExec(tx,
					`DROP TABLE IF EXISTS modeler_statuses`)
			},
			Rollback: func(tx *gorm.DB) error {
				return fmt.Errorf("cannot rollback")
			},
		},
		{
			ID: "00096-RenameCleanupStatus",
			Migrate: func(tx *gorm.DB) error {
				return multiExec(tx,
					`ALTER TABLE cleanup_statuses RENAME TO lock_statuses`)
			},
			Rollback: func(tx *gorm.DB) error {
				return multiExec(tx,
					`ALTER TABLE lock_statuses RENAME TO cleanup_statuses`)
			},
		},
		{
			ID: "00097-AddUserReport",
			Migrate: func(tx *gorm.DB) error {
				return multiExec(tx,
					`CREATE TABLE user_reports (
						id BIGSERIAL,
						phone_hash TEXT NOT NULL,
						nonce TEXT NOT NULL,
						code_claimed BOOLEAN NOT NULL DEFAULT false,
						created_at TIMESTAMP WITH TIME ZONE NOT NULL,
						updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
						PRIMARY KEY (id))`,
					`CREATE UNIQUE INDEX uix_user_report_phone_hash ON user_reports (phone_hash)`,
					`CREATE INDEX IF NOT EXISTS idx_user_report_created_at ON user_reports (created_at)`,
					`ALTER TABLE verification_codes
						ADD COLUMN IF NOT EXISTS user_report_id INTEGER`,
					`ALTER TABLE verification_codes ADD CONSTRAINT fk_user_report_id FOREIGN KEY(user_report_id) REFERENCES user_reports(id) ON DELETE SET NULL`)
			},
			Rollback: func(tx *gorm.DB) error {
				return multiExec(tx,
					`ALTER TABLE verification_codes DROP COLUMN IF EXISTS user_report_id`,
					`DROP TABLE IF EXISTS user_reports`)
			},
		},
		{
			ID: "00098-AddUserReportStats",
			Migrate: func(tx *gorm.DB) error {
				return multiExec(tx,
					`ALTER TABLE realm_stats
						ADD COLUMN IF NOT EXISTS user_reports_issued INTEGER DEFAULT 0,
						ADD COLUMN IF NOT EXISTS user_reports_claimed INTEGER DEFAULT 0,
						ADD COLUMN IF NOT EXISTS user_report_tokens_claimed INTEGER DEFAULT 0`,
					`ALTER TABLE realm_stats
						ALTER COLUMN user_reports_issued SET NOT NULL,
						ALTER COLUMN user_reports_claimed SET NOT NULL,
						ALTER COLUMN user_report_tokens_claimed SET NOT NULL`)
			},
			Rollback: func(tx *gorm.DB) error {
				return multiExec(tx,
					`ALTER TABLE realm_stats
						DROP COLUMN user_reports_issued,
						DROP COLUMN user_reports_claimed,
						DROP COLUMN user_report_tokens_claimed`)
			},
		},
		{
			ID: "00099-AdminSelfReportSettings",
			Migrate: func(tx *gorm.DB) error {
				return multiExec(tx,
					`ALTER TABLE user_reports
						ADD COLUMN IF NOT EXISTS nonce_required BOOLEAN DEFAULT TRUE`,
					`ALTER TABLE user_reports
						ALTER COLUMN nonce_required SET NOT NULL`,
					`ALTER TABLE realms
						ADD COLUMN IF NOT EXISTS allow_admin_user_report BOOLEAN DEFAULT false`,
					`ALTER TABLE realms
						ALTER COLUMN allow_admin_user_report SET NOT NULL`)
			},
			Rollback: func(tx *gorm.DB) error {
				return multiExec(tx,
					`ALTER TABLE realms
						DROP COLUMN allow_admin_user_report`)
			},
		},
	}
}

// MigrateTo migrates the database to a specific target migration ID.
func (db *Database) MigrateTo(ctx context.Context, target string, rollback bool) error {
	options := gormigrate.DefaultOptions
	migrations := db.Migrations(ctx)
	m := gormigrate.New(db.db, options, migrations)

	if rollback {
		if target == "" {
			return fmt.Errorf("rollback requires a target")
		}
		return m.RollbackTo(target)
	}

	if target != "" {
		return m.MigrateTo(target)
	}
	return m.Migrate()
}

// multiExec is a helper that executes the given sql clauses against the tx.
func multiExec(tx *gorm.DB, sqls ...string) error {
	return tx.Transaction(func(tx *gorm.DB) error {
		for _, sql := range sqls {
			if err := tx.Exec(sql).Error; err != nil {
				return fmt.Errorf("failed to execute %q: %w", sql, err)
			}
		}
		return nil
	})
}
