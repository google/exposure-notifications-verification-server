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

	"github.com/jinzhu/gorm"
	"gopkg.in/gormigrate.v1"
)

const initState = "00000-Init"

func (db *Database) getMigrations(ctx context.Context) *gormigrate.Gormigrate {
	logger := logging.FromContext(ctx)
	options := gormigrate.DefaultOptions

	// Each migration runs in its own transacton already. Setting to true forces
	// all unrun migrations to run in a _single_ transaction which is probably
	// undesirable.
	options.UseTransaction = false

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
				logger.Debugw("db migrations: creating realms table")
				if err := tx.AutoMigrate(&Realm{}).Error; err != nil {
					return err
				}

				logger.Debugw("db migrations: creating users table")
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
				logger.Debugw("db migrations: creating verification codes table")
				return tx.AutoMigrate(&VerificationCode{}).Error
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.DropTable("verification_codes").Error
			},
		},
		{
			ID: "00003-CreateAuthorizedApps",
			Migrate: func(tx *gorm.DB) error {
				logger.Debugw("db migrations: creating authorized apps table")
				return tx.AutoMigrate(&AuthorizedApp{}).Error
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.DropTable("authorized_apps").Error
			},
		},
		{
			ID: "00004-CreateTokens",
			Migrate: func(tx *gorm.DB) error {
				logger.Debugw("db migrations: creating tokens table")
				return tx.AutoMigrate(&Token{}).Error
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.DropTable("tokens").Error
			},
		},
		{
			ID: "00005-CreateCleanups",
			Migrate: func(tx *gorm.DB) error {
				logger.Debugw("db migrations: creating cleanup status table")
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
				logger.Debugw("db migrations: add users purge index")
				if err := tx.Model(&User{}).AddIndex("users_purge_index", "updated_at").Error; err != nil {
					return err
				}
				logger.Debugw("db migrations: add verification code purge index")
				if err := tx.Model(&VerificationCode{}).AddIndex("ver_code_purge_index", "expires_at").Error; err != nil {
					return err
				}
				logger.Debugw("db migrations: add tokens purge index")
				if err := tx.Model(&VerificationCode{}).AddIndex("token_purge_index", "expires_at").Error; err != nil {
					return err
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				logger.Debugw("db migrations: drop users purge index")
				if err := tx.Model(&User{}).RemoveIndex("users_purge_index").Error; err != nil {
					return err
				}
				logger.Debugw("db migrations: drop verification code purge index")
				if err := tx.Model(&VerificationCode{}).RemoveIndex("ver_code_purge_index").Error; err != nil {
					return err
				}
				logger.Debugw("db migrations: drop tokens purge index")
				if err := tx.Model(&VerificationCode{}).RemoveIndex("token_purge_index").Error; err != nil {
					return err
				}
				return nil
			},
		},
		{
			ID: "00007-AddSymptomOnset",
			Migrate: func(tx *gorm.DB) error {
				logger.Debugw("db migrations: rename test_date to symptom_date")
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
				logger.Debugw("db migrations: rename symptom_date to test_date")
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
				logger.Debugw("db migrations: upgrading authorized_apps table.")
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
				logger.Debugw("db migrations: adding issuer columns to issued codes")
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
				logger.Debugw("db migrations: adding sms_configs table")
				return tx.AutoMigrate(&SMSConfig{}).Error
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.DropTable("sms_configs").Error
			},
		},
		{
			ID: "00011-AddRealms",
			Migrate: func(tx *gorm.DB) error {
				logger.Debugw("db migrations: create realms table")
				// Add the realms table.
				if err := tx.AutoMigrate(&Realm{}).Error; err != nil {
					return err
				}
				logger.Debugw("Create the DEFAULT realm")
				// Create the default realm with all of the default settings.
				defaultRealm := NewRealmWithDefaults("Default")
				if err := tx.FirstOrCreate(defaultRealm).Error; err != nil {
					return err
				}

				// Add realm relations to the rest of the tables.
				logger.Debugw("Add RealmID to Users.")
				if err := tx.AutoMigrate(&User{}).Error; err != nil {
					return err
				}
				logger.Debugw("Join Users to Default Realm")
				var users []*User
				if err := tx.Find(&users).Error; err != nil {
					return err
				}
				for _, u := range users {
					logger.Debugw("added user: %v to default realm", u.ID)

					u.AddRealm(defaultRealm)
					if u.Admin {
						u.AddRealmAdmin(defaultRealm)
					}

					if err := tx.Save(u).Error; err != nil {
						return err
					}
				}

				logger.Debugw("Add RealmID to AuthorizedApps.")
				if err := tx.AutoMigrate(&AuthorizedApp{}).Error; err != nil {
					return err
				}
				var authApps []*AuthorizedApp
				if err := tx.Unscoped().Find(&authApps).Error; err != nil {
					return err
				}
				for _, a := range authApps {
					logger.Debugw("added auth app: %v to default realm", a.Name)
					a.RealmID = defaultRealm.ID
					if err := tx.Save(a).Error; err != nil {
						return err
					}
				}

				logger.Debugw("Add RealmID to VerificationCodes.")
				if err := tx.AutoMigrate(&VerificationCode{}).Error; err != nil {
					return err
				}
				logger.Debugw("Join existing VerificationCodes to default realm")
				if err := tx.Exec("UPDATE verification_codes SET realm_id=?", defaultRealm.ID).Error; err != nil {
					return err
				}

				logger.Debugw("Add RealmID to Tokens.")
				if err := tx.AutoMigrate(&Token{}).Error; err != nil {
					return err
				}
				logger.Debugw("Join existing tokens to default realm")
				if err := tx.Exec("UPDATE tokens SET realm_id=?", defaultRealm.ID).Error; err != nil {
					return err
				}

				logger.Debugw("Add RealmID to SMSConfig.")
				if err := tx.AutoMigrate(&SMSConfig{}).Error; err != nil {
					return err
				}
				logger.Debugw("Join existing SMS config to default realm")
				if err := tx.Exec("UPDATE sms_configs SET realm_id=?", defaultRealm.ID).Error; err != nil {
					return err
				}

				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				ddl := []string{
					"ALTER TABLE sms_configs DROP COLUMN realm_id",
					"ALTER TABLE tokens DROP COLUMN realm_id",
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
		{
			ID: "00012-DropAuthorizedAppUniqueNameIndex",
			Migrate: func(tx *gorm.DB) error {
				logger.Debugw("dropping authorized apps unique name index")
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
				logger.Debugw("adding authorized apps realm/name composite index")
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
				logger.Debugw("db migrations: dropping user purge index=")
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
				logger.Debugw("db migrations: dropping user disabled column")
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
				logger.Debugw("db migrations: migrating sms configs")

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
						sms.TwilioAuthToken = string(val)
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
				logger.Debugw("db migrations: adding issuer id columns to verification codes")
				err := tx.AutoMigrate(&VerificationCode{}, &UserStats{}, &AuthorizedAppStats{}).Error
				return err

			},
			Rollback: func(tx *gorm.DB) error {
				if err := tx.Model(&VerificationCode{}).DropColumn("issuing_user_id").Error; err != nil {
					return err
				}
				if err := tx.Model(&VerificationCode{}).DropColumn("issuing_app_id").Error; err != nil {
					return err
				}
				if err := tx.DropTable(&UserStats{}).Error; err != nil {
					return err
				}
				return nil
			},
		},
		{
			ID: "00018-IncreaseAPIKeySize",
			Migrate: func(tx *gorm.DB) error {
				logger.Debugw("db migrations: increasing API key size")
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
				logger.Debugw("db migrations: migrating authapp")
				return tx.AutoMigrate(AuthorizedApp{}).Error
			},
			Rollback: func(tx *gorm.DB) error {
				return nil
			},
		},
		{
			ID: "00020-HMACAPIKeys",
			Migrate: func(tx *gorm.DB) error {
				logger.Debugw("db migrations: HMACing existing api keys")

				var apps []AuthorizedApp
				if err := tx.Model(AuthorizedApp{}).Find(&apps).Error; err != nil {
					return err
				}

				for _, app := range apps {
					// If the key has a preview, it's v2
					if app.APIKeyPreview != "" {
						continue
					}

					apiKeyPreview := app.APIKey[:6]
					newAPIKey, err := db.hmacAPIKey(app.APIKey)
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
				logger.Debugw("db migrations: adding uuid extension")
				return tx.Exec("CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\"").Error
			},
			Rollback: func(tx *gorm.DB) error {
				return nil
			},
		},
		{
			ID: "00022-AddUUIDToVerificationCodes",
			Migrate: func(tx *gorm.DB) error {
				logger.Debugw("db migrations: migrating verification code uuid")

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
				logger.Debugw("db migrations: making verification code uuid not null")

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
				logger.Debugw("db migrations: adding test types to realm")

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
				logger.Debugw("db migrations: setting test types to not-null")

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
				logger.Debugw("db migrations: enabling citext extension")
				return tx.Exec("CREATE EXTENSION citext").Error
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Exec("DROP EXTENSION citext").Error
			},
		},
		{
			ID: "00027-AlterColumns_citext",
			Migrate: func(tx *gorm.DB) error {
				logger.Debugw("db migrations: setting columns to case insensitive")
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
				logger.Debugw("db migrations: adding long_code and SMS deeplink settings")
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

				logger.Debugw("db migrations: add verification code purge index")
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
					stmt := fmt.Sprintf("ALTER TABLE realms DROP COLUMN %s", col)
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
				logger.Debugw("db migrations: increasing verification code sizes")
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
				sqls := []string{
					"ALTER TABLE verification_codes ALTER COLUMN code TYPE varchar(20)",
					"ALTER TABLE verification_codes ALTER COLUMN long_code TYPE varchar(20)",
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
			ID: "00030-HMACCodes",
			Migrate: func(tx *gorm.DB) error {
				logger.Debugw("db migrations: HMACing existing tokens")

				var codes []VerificationCode
				if err := tx.Model(VerificationCode{}).Find(&codes).Error; err != nil {
					return err
				}

				for _, code := range codes {
					changed := false

					// Sanity
					if len(code.Code) < 20 {
						h, err := db.hmacVerificationCode(code.Code)
						if err != nil {
							return err
						}
						code.Code = h
						changed = true
					}

					// Sanity
					if len(code.LongCode) < 20 {
						h, err := db.hmacVerificationCode(code.LongCode)
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
				logger.Debugw("db migrations: changing stats columns")

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
	})
}

// MigrateTo migrates the database to a specific target migration ID.
func (db *Database) MigrateTo(ctx context.Context, target string, rollback bool) error {
	logger := logging.FromContext(ctx)
	m := db.getMigrations(ctx)
	logger.Debugw("database migrations started")

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
	logger.Debugw("database migrations completed")
	return nil
}

// RunMigrations will apply sequential, transactional migrations to the database
func (db *Database) RunMigrations(ctx context.Context) error {
	logger := logging.FromContext(ctx)
	m := db.getMigrations(ctx)
	logger.Debugw("database migrations started")
	if err := m.Migrate(); err != nil {
		logger.Errorf("migrations failed: %v", err)
		return err
	}
	logger.Debugw("database migrations completed")
	return nil
}
