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

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

const initState = "00000-Init"

func (db *Database) getMigrations(ctx context.Context) *gormigrate.Gormigrate {
	logger := logging.FromContext(ctx)
	options := gormigrate.DefaultOptions

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
				if err := tx.Migrator().AutoMigrate(&Realm{}); err != nil {
					return err
				}

				return tx.Migrator().AutoMigrate(&User{})
			},
			Rollback: func(tx *gorm.DB) error {
				if err := tx.Migrator().DropTable("users"); err != nil {
					return err
				}
				return tx.Migrator().DropTable("realms")
			},
		},
		{
			ID: "00002-CreateVerificationCodes",
			Migrate: func(tx *gorm.DB) error {
				return tx.Migrator().AutoMigrate(&VerificationCode{})
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable("verification_codes")
			},
		},
		{
			ID: "00003-CreateAuthorizedApps",
			Migrate: func(tx *gorm.DB) error {
				return tx.Migrator().AutoMigrate(&AuthorizedApp{})
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable("authorized_apps")
			},
		},
		{
			ID: "00004-CreateTokens",
			Migrate: func(tx *gorm.DB) error {
				return tx.Migrator().AutoMigrate(&Token{})
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable("tokens")
			},
		},
		{
			ID: "00005-CreateCleanups",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.Migrator().AutoMigrate(&CleanupStatus{}); err != nil {
					return err
				}
				// Seed database w/ cleanup record.
				if err := tx.Create(&CleanupStatus{Type: "cleanup", Generation: 1, NotBefore: time.Now()}).Error; err != nil {
					return err
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable("cleanup_statuses")
			},
		},
		{
			ID: "00006-AddIndexes",
			Migrate: func(tx *gorm.DB) error {
				return exec(tx,
					`CREATE INDEX IF NOT EXISTS users_purge_index ON users(updated_at)`,
					`CREATE INDEX IF NOT EXISTS ver_code_purge_index ON verification_codes(expires_at)`,
					`CREATE INDEX IF NOT EXISTS token_purge_index ON tokens(expires_at)`,
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return exec(tx,
					`DROP INDEX IF EXISTS users_purge_index`,
					`DROP INDEX IF EXISTS ver_code_purge_index`,
					`DROP INDEX IF EXISTS token_purge_index`,
				)
			},
		},
		{
			ID: "00007-AddSymptomOnset",
			Migrate: func(tx *gorm.DB) error {
				// AutoMigrate will add missing fields.
				if err := tx.Migrator().AutoMigrate(&VerificationCode{}); err != nil {
					return err
				}

				// If not upgrading from an old version, this column will have never
				// been created.
				if err := tx.Exec(`ALTER TABLE verification_codes DROP COLUMN IF EXISTS test_date`).Error; err != nil {
					return err
				}

				if err := tx.Migrator().AutoMigrate(&Token{}); err != nil {
					return err
				}

				// If not upgrading from an old version, this column will have never
				// been created.
				if err := tx.Exec(`ALTER TABLE tokens DROP COLUMN IF EXISTS test_date`).Error; err != nil {
					return err
				}

				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE verification_codes DROP COLUMN IF EXISTS symptom_date`,
					`ALTER TABLE tokens DROP COLUMN IF EXISTS symptom_date`,
				)
			},
		},
		{
			ID: "00008-AddKeyTypes",
			Migrate: func(tx *gorm.DB) error {
				return tx.Migrator().AutoMigrate(&AuthorizedApp{})
			},
			Rollback: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE authorized_apps DROP COLUMN IF EXISTS admin_key`,
				)
			},
		},
		{
			ID: "00009-AddIssuerColumns",
			Migrate: func(tx *gorm.DB) error {
				return tx.Migrator().AutoMigrate(&VerificationCode{})
			},
			Rollback: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE verification_codes DROP COLUMN IF EXISTS issuing_user`,
					`ALTER TABLE verification_codes DROP COLUMN IF EXISTS issuing_app`,
				)
			},
		},
		{
			ID: "00010-AddSMSConfig",
			Migrate: func(tx *gorm.DB) error {
				return tx.Migrator().AutoMigrate(&SMSConfig{})
			},
			Rollback: func(tx *gorm.DB) error {
				return exec(tx,
					`DROP TABLE IF EXISTS sms_configs`,
				)
			},
		},
		{
			ID: "00011-AddRealms",
			Migrate: func(tx *gorm.DB) error {
				// Add the realms table.
				if err := tx.Migrator().AutoMigrate(&Realm{}); err != nil {
					return err
				}
				// Create the default realm with all of the default settings.
				defaultRealm := NewRealmWithDefaults("Default")
				if err := tx.FirstOrCreate(defaultRealm).Error; err != nil {
					return err
				}

				// Add realm relations to the rest of the tables.
				if err := tx.Migrator().AutoMigrate(&User{}); err != nil {
					return err
				}
				var users []*User
				if err := tx.Find(&users).Error; err != nil {
					return err
				}

				for _, u := range users {
					u.AddRealm(defaultRealm)
					if u.Admin {
						u.AddRealmAdmin(defaultRealm)
					}

					if err := tx.Save(u).Error; err != nil {
						return err
					}
				}

				if err := tx.Migrator().AutoMigrate(&AuthorizedApp{}); err != nil {
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

				if err := tx.Migrator().AutoMigrate(&VerificationCode{}); err != nil {
					return err
				}
				if err := tx.Exec("UPDATE verification_codes SET realm_id = ?", defaultRealm.ID).Error; err != nil {
					return err
				}

				if err := tx.Migrator().AutoMigrate(&Token{}); err != nil {
					return err
				}
				if err := tx.Exec("UPDATE tokens SET realm_id = ?", defaultRealm.ID).Error; err != nil {
					return err
				}

				if err := tx.Migrator().AutoMigrate(&SMSConfig{}); err != nil {
					return err
				}
				if err := tx.Exec("UPDATE sms_configs SET realm_id = ?", defaultRealm.ID).Error; err != nil {
					return err
				}

				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				return exec(tx,
					"ALTER TABLE sms_configs DROP COLUMN realm_id",
					"ALTER TABLE tokens DROP COLUMN realm_id",
					"ALTER TABLE verification_codes DROP COLUMN realm_id",
					"ALTER TABLE authorized_apps DROP COLUMN realm_id",
					"DROP TABLE admin_realms",
					"DROP TABLE user_realms",
					"DROP TABLE realms",
				)
			},
		},
		{
			ID: "00012-DropAuthorizedAppUniqueNameIndex",
			Migrate: func(tx *gorm.DB) error {
				return exec(tx,
					`DROP INDEX IF EXISTS uix_authorized_apps_name`,
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return nil
			},
		},
		{
			ID: "00013-AddCompositeIndexOnAuthorizedApp",
			Migrate: func(tx *gorm.DB) error {
				return tx.Migrator().AutoMigrate(&AuthorizedApp{})
			},
			Rollback: func(tx *gorm.DB) error {
				return nil
			},
		},
		{
			ID: "00014-DropUserPurgeIndex",
			Migrate: func(tx *gorm.DB) error {
				return exec(tx,
					`DROP INDEX IF EXISTS users_purge_index`,
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return exec(tx,
					`CREATE INDEX IF NOT EXISTS users_purge_index ON users(updated_at)`,
				)
			},
		},
		{
			ID: "00015-DropUserDisabled",
			Migrate: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE users DROP COLUMN IF EXISTS disabled`,
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE users ADD COLUMN disabled bool NOT NULL DEFAULT true`,
				)
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
				return tx.Migrator().AutoMigrate(&VerificationCode{}, &UserStats{}, &AuthorizedAppStats{})
			},
			Rollback: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE verification_codes DROP COLUMN IF EXISTS issuing_user_id`,
					`ALTER TABLE verification_codes DROP COLUMN IF EXISTS issuing_app_id`,
					`DROP TABLE IF EXISTS user_stats`,
				)
			},
		},
		{
			ID: "00018-IncreaseAPIKeySize",
			Migrate: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE authorized_apps ALTER COLUMN api_key TYPE varchar(512)`,
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return nil
			},
		},
		{
			ID: "00019-AddAPIKeyPreviewAuthApp",
			Migrate: func(tx *gorm.DB) error {
				return tx.Migrator().AutoMigrate(AuthorizedApp{})
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
				return exec(tx,
					`CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`,
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return nil
			},
		},
		{
			ID: "00022-AddUUIDToVerificationCodes",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.Migrator().AutoMigrate(VerificationCode{}); err != nil {
					return err
				}

				return exec(tx,
					`ALTER TABLE verification_codes ALTER COLUMN uuid SET DEFAULT uuid_generate_v4()`,
					`UPDATE verification_codes SET uuid = uuid_generate_v4() WHERE uuid IS NULL`,
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE verification_codes ALTER COLUMN uuid DROP DEFAULT`,
				)
			},
		},
		{
			ID: "00023-MakeUUIDVerificationCodesNotNull",
			Migrate: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE verification_codes ALTER COLUMN uuid SET NOT NULL`,
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE verification_codes ALTER COLUMN uuid DROP NOT NULL`,
				)
			},
		},
		{
			ID: "00024-AddTestTypesToRealms",
			Migrate: func(tx *gorm.DB) error {
				return exec(tx,
					fmt.Sprintf("ALTER TABLE realms ADD COLUMN IF NOT EXISTS allowed_test_types INTEGER DEFAULT %d", TestTypeConfirmed|TestTypeLikely|TestTypeNegative),
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE realms DROP COLUMN IF EXISTS allowed_test_types`,
				)
			},
		},
		{
			ID: "00025-SetTestTypesNotNull",
			Migrate: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE realms ALTER COLUMN allowed_test_types SET NOT NULL`,
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE realms ALTER COLUMN allowed_test_types DROP NOT NULL`,
				)
			},
		},
		{
			ID: "00026-EnableExtension_citext",
			Migrate: func(tx *gorm.DB) error {
				return exec(tx,
					`CREATE EXTENSION citext`,
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return nil
			},
		},
		{
			ID: "00027-AlterColumns_citext",
			Migrate: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE authorized_apps ALTER COLUMN name TYPE CITEXT`,
					`ALTER TABLE realms ALTER COLUMN name TYPE CITEXT`,
					`ALTER TABLE users ALTER COLUMN email TYPE CITEXT`,
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE authorized_apps ALTER COLUMN name TYPE TEXT`,
					`ALTER TABLE realms ALTER COLUMN name TYPE TEXT`,
					`ALTER TABLE users ALTER COLUMN email TYPE TEXT`,
				)
			},
		},
		{
			ID: "00028-AddSMSDeeplinkFields",
			Migrate: func(tx *gorm.DB) error {
				// long_code cannot be auto migrated because of unique index.
				// manually create long_code and long_expires_at and backfill with existing data.
				if err := exec(tx,
					`ALTER TABLE verification_codes ADD COLUMN IF NOT EXISTS long_code VARCHAR(20)`,
					`UPDATE verification_codes SET long_code = code`,
					`CREATE UNIQUE INDEX IF NOT EXISTS uix_verification_codes_long_code ON verification_codes(long_code)`,
					`ALTER TABLE verification_codes ALTER COLUMN long_code SET NOT NULL`,
					`ALTER TABLE verification_codes ADD COLUMN IF NOT EXISTS long_expires_at TIMESTAMPTZ`,
					`UPDATE verification_codes SET long_expires_at = expires_at`,
				); err != nil {
					return nil
				}

				if err := tx.Migrator().AutoMigrate(&Realm{}, &VerificationCode{}); err != nil {
					return err
				}

				return exec(tx,
					`CREATE INDEX IF NOT EXISTS ver_code_long_purge_index ON verification_codes(long_expires_at)`,
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE realms DROP COLUMN IF EXISTS long_code_length`,
					`ALTER TABLE realms DROP COLUMN IF EXISTS long_code_duration`,
					`ALTER TABLE realms DROP COLUMN IF EXISTS region_code`,
					`ALTER TABLE realms DROP COLUMN IF EXISTS code_length`,
					`ALTER TABLE realms DROP COLUMN IF EXISTS code_duration`,
					`ALTER TABLE realms DROP COLUMN IF EXISTS sms_text_template`,
				)
			},
		},
		{
			ID: "00029-IncreaseVerificationCodeSizes",
			Migrate: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE verification_codes ALTER COLUMN code TYPE varchar(512)`,
					`ALTER TABLE verification_codes ALTER COLUMN long_code TYPE varchar(512)`,
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return nil
			},
		},
		{
			ID: "00030-HMACCodes",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.Migrator().AutoMigrate(&Realm{}); err != nil {
					return err
				}

				var codes []VerificationCode
				if err := tx.Model(VerificationCode{}).Find(&codes).Error; err != nil {
					return err
				}

				for _, code := range codes {
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

				return exec(tx,
					// AuthorizedApps
					`CREATE UNIQUE INDEX IF NOT EXISTS idx_authorized_app_stats_date_authorized_app_id ON authorized_app_stats (date, authorized_app_id)`,
					`DROP INDEX IF EXISTS idx_date_app_realm`,
					`DROP INDEX IF EXISTS idx_authorized_app_stats_deleted_at`,
					`CREATE INDEX IF NOT EXISTS idx_authorized_app_stats_date ON authorized_app_stats (date)`,
					`ALTER TABLE authorized_app_stats DROP COLUMN IF EXISTS id`,
					`ALTER TABLE authorized_app_stats DROP COLUMN IF EXISTS created_at`,
					`ALTER TABLE authorized_app_stats DROP COLUMN IF EXISTS updated_at`,
					`ALTER TABLE authorized_app_stats DROP COLUMN IF EXISTS deleted_at`,
					`ALTER TABLE authorized_app_stats DROP COLUMN IF EXISTS realm_id`,
					`ALTER TABLE authorized_app_stats ALTER COLUMN date TYPE date`,
					`ALTER TABLE authorized_app_stats ALTER COLUMN date SET NOT NULL`,
					`ALTER TABLE authorized_app_stats ALTER COLUMN authorized_app_id SET NOT NULL`,
					`ALTER TABLE authorized_app_stats ALTER COLUMN codes_issued TYPE INTEGER`,
					`ALTER TABLE authorized_app_stats ALTER COLUMN codes_issued SET DEFAULT 0`,
					`ALTER TABLE authorized_app_stats ALTER COLUMN codes_issued SET NOT NULL`,

					// Users
					`CREATE UNIQUE INDEX IF NOT EXISTS idx_user_stats_date_realm_id_user_id ON user_stats (date, realm_id, user_id)`,
					`DROP INDEX IF EXISTS idx_date_user_realm`,
					`DROP INDEX IF EXISTS idx_user_stats_deleted_at`,
					`CREATE INDEX IF NOT EXISTS idx_user_stats_date ON user_stats (date)`,
					`ALTER TABLE user_stats DROP COLUMN IF EXISTS id`,
					`ALTER TABLE user_stats DROP COLUMN IF EXISTS created_at`,
					`ALTER TABLE user_stats DROP COLUMN IF EXISTS updated_at`,
					`ALTER TABLE user_stats DROP COLUMN IF EXISTS deleted_at`,
					`ALTER TABLE user_stats ALTER COLUMN date TYPE date`,
					`ALTER TABLE user_stats ALTER COLUMN date SET NOT NULL`,
					`ALTER TABLE user_stats ALTER COLUMN realm_id SET NOT NULL`,
					`ALTER TABLE user_stats ALTER COLUMN user_id SET NOT NULL`,
					`ALTER TABLE user_stats ALTER COLUMN codes_issued TYPE INTEGER`,
					`ALTER TABLE user_stats ALTER COLUMN codes_issued SET DEFAULT 0`,
					`ALTER TABLE user_stats ALTER COLUMN codes_issued SET NOT NULL`,
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return nil
			},
		},
		{
			ID: "00032-RegionCodeSize",
			Migrate: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE realms ALTER COLUMN region_code TYPE varchar(10)`,
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return nil
			},
		},
		{
			ID: "00033-PerlRealmSigningKeys",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.Migrator().AutoMigrate(&Realm{}); err != nil {
					return err
				}
				if err := tx.Migrator().AutoMigrate(&SigningKey{}); err != nil {
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
				return tx.Migrator().AutoMigrate(&Realm{})
			},
			Rollback: func(tx *gorm.DB) error {
				return nil
			},
		},
		{
			ID: "00035-AddMFARequiredToRealms",
			Migrate: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE realms ADD COLUMN IF NOT EXISTS mfa_mode INTEGER DEFAULT 0`,
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE realms DROP COLUMN IF EXISTS mfa_mode`,
				)
			},
		},
		{
			ID: "00036-AddRealmStats",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.Migrator().AutoMigrate(&RealmStats{}); err != nil {
					return err
				}

				return exec(tx,
					`CREATE UNIQUE INDEX IF NOT EXISTS idx_realm_stats_stats_date_realm_id ON realm_stats (date, realm_id)`,
					`CREATE INDEX IF NOT EXISTS idx_realm_stats_date ON realm_stats (date)`,
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return exec(tx,
					`DROP TABLE IF EXISTS realm_stats`,
				)
			},
		},
		{
			ID: "00037-AddRealmRequireDate",
			Migrate: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE realms ADD COLUMN IF NOT EXISTS require_date bool DEFAULT false`,
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE realms DROP COLUMN IF EXISTS require_date`,
				)
			},
		},
		{
			ID: "00038-AddRealmRequireDateNotNull",
			Migrate: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE realms ALTER COLUMN require_date SET NOT NULL`,
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE realms ALTER COLUMN require_date SET NULL`,
				)
			},
		},
		{
			ID: "00039-RealmStatsToDate",
			Migrate: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE realm_stats ALTER COLUMN date TYPE date`,
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE realm_stats ALTER COLUMN date TYPE timestamp with time zone`,
				)
			},
		},
		{
			ID: "00040-BackfillRealmStats",
			Migrate: func(tx *gorm.DB) error {
				return exec(tx,
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
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return nil
			},
		},
		{
			ID: "00041-AddRealmAbusePrevention",
			Migrate: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE realms ADD COLUMN IF NOT EXISTS abuse_prevention_enabled bool NOT NULL DEFAULT false`,
					`ALTER TABLE realms ADD COLUMN IF NOT EXISTS abuse_prevention_limit integer NOT NULL DEFAULT 100`,
					`ALTER TABLE realms ADD COLUMN IF NOT EXISTS abuse_prevention_limit_factor numeric(8, 5) NOT NULL DEFAULT 1.0`,
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE realms DROP COLUMN IF EXISTS abuse_prevention_enabled`,
					`ALTER TABLE realms DROP COLUMN IF EXISTS abuse_prevention_limit`,
					`ALTER TABLE realms DROP COLUMN IF EXISTS abuse_prevention_limit_factor`,
				)
			},
		},
		{
			ID: "00042-ChangeRealmAbusePreventionLimitDefault",
			Migrate: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE realms ALTER COLUMN abuse_prevention_limit SET DEFAULT 10`,
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE realms ALTER COLUMN abuse_prevention_limit SET DEFAULT 100`,
				)
			},
		},
		{
			ID: "00043-CreateModelerStatus",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.Migrator().AutoMigrate(&ModelerStatus{}); err != nil {
					return err
				}
				if err := tx.Create(&ModelerStatus{}).Error; err != nil {
					return err
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				return exec(tx,
					`DROP TABLE IF EXISTS modeler_statuses`,
				)
			},
		},
		{
			ID: "00044-AddEmailVerifiedRequiredToRealms",
			Migrate: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE realms ADD COLUMN IF NOT EXISTS email_verified_mode INTEGER DEFAULT 0`,
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE realms DROP COLUMN IF EXISTS email_verified_mode`,
				)
			},
		},
		{
			ID: "00045-BootstrapSystemAdmin",
			Migrate: func(tx *gorm.DB) error {
				// Only create the default system admin if there are no users. This
				// ensures people who are already running a system don't get a random
				// admin user.
				var user User
				if err := db.db.Model(&User{}).First(&user).Error; err == nil {
					return nil
				} else {
					if !IsNotFound(err) {
						return err
					}
				}

				user = User{
					Name:  "System admin",
					Email: "super@example.com",
					Admin: true,
				}

				return tx.Save(&user).Error
			},
			Rollback: func(tx *gorm.DB) error {
				return nil
			},
		},
		{
			ID: "00046-AddWelcomeMessageToRealm",
			Migrate: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE realms ADD COLUMN IF NOT EXISTS welcome_message text`,
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE realms DROP COLUMN IF EXISTS welcome_message`,
				)
			},
		},
		{
			ID: "00047-AddPasswordLastChangedToUsers",
			Migrate: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE users ADD COLUMN IF NOT EXISTS last_password_change DATE DEFAULT CURRENT_DATE`,
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE users DROP COLUMN IF EXISTS last_password_change`,
				)
			},
		},
		{
			ID: "00048-AddPasswordRotateToRealm",
			Migrate: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE realms ADD COLUMN IF NOT EXISTS password_rotation_period_days integer NOT NULL DEFAULT 0`,
					`ALTER TABLE realms ADD COLUMN IF NOT EXISTS password_rotation_warning_days integer NOT NULL DEFAULT 0`,
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE realms DROP COLUMN IF EXISTS password_rotation_period_days`,
					`ALTER TABLE realms DROP COLUMN IF EXISTS password_rotation_warning_days `,
				)
			},
		},
		{
			ID: "00049-MakeRegionCodeUnique",
			Migrate: func(tx *gorm.DB) error {
				if err := exec(tx,
					// Make region code case insensitive and unique.
					"ALTER TABLE realms ALTER COLUMN region_code TYPE CITEXT",
					"ALTER TABLE realms ALTER COLUMN region_code DROP DEFAULT",
					"ALTER TABLE realms ALTER COLUMN region_code DROP NOT NULL",
				); err != nil {
					return err
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

				return exec(tx,
					// There's already a runtime constraint and validation on names, this
					// is just an extra layer of protection at the database layer.
					`ALTER TABLE realms ALTER COLUMN name SET NOT NULL`,

					// Alter the unique index on realm names to be a column constraint.
					`DROP INDEX IF EXISTS uix_realms_name`,
					`ALTER TABLE realms ADD CONSTRAINT uix_realms_name UNIQUE (name)`,

					// Now finally add a unique constraint on region codes.
					`ALTER TABLE realms ADD CONSTRAINT uix_realms_region_code UNIQUE (region_code)`,
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return nil
			},
		},
		{
			ID: "00050-CreateMobileApps",
			Migrate: func(tx *gorm.DB) error {
				return tx.Migrator().AutoMigrate(&MobileApp{})
			},
			Rollback: func(tx *gorm.DB) error {
				return exec(tx,
					`DROP TABLE IF EXISTS mobile_apps`,
				)
			},
		},
		{
			ID: "00051-CreateSystemSMSConfig",
			Migrate: func(tx *gorm.DB) error {
				return exec(tx,
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
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE sms_configs DROP COLUMN IF EXISTS is_system`,
					`DROP INDEX IF EXISTS uix_sms_configs_is_system_true`,
					`ALTER TABLE sms_configs DROP CONSTRAINT IF EXISTS nn_sms_configs_realm_id`,
					`ALTER TABLE sms_configs DROP CONSTRAINT IF EXISTS uix_sms_configs_realm_id`,

					`ALTER TABLE realms DROP COLUMN IF EXISTS can_use_system_sms_config`,
					`ALTER TABLE realms DROP COLUMN IF EXISTS use_system_sms_config`,
				)
			},
		},
		{
			ID: "00052-CreateRealmAllowedCIDRs",
			Migrate: func(tx *gorm.DB) error {
				return tx.Migrator().AutoMigrate(&Realm{})
			},
			Rollback: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE realms DROP COLUMN IF EXISTS allowed_cidrs_adminapi`,
					`ALTER TABLE realms DROP COLUMN IF EXISTS allowed_cidrs_apiserver`,
					`ALTER TABLE realms DROP COLUMN IF EXISTS allowed_cidrs_server`,
				)
			},
		},
		{
			ID: "00053-AddRealmSMSCountry",
			Migrate: func(tx *gorm.DB) error {
				return tx.Migrator().AutoMigrate(&Realm{})
			},
			Rollback: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE realms DROP COLUMN IF EXISTS sms_country`,
				)
			},
		},
		{
			ID: "00054-ChangeMobileAppNameUniqueness",
			Migrate: func(tx *gorm.DB) error {
				return exec(tx,
					`DROP INDEX IF EXISTS realm_app_name`,
					`ALTER TABLE mobile_apps ADD CONSTRAINT uix_name_realm_id_os UNIQUE (name, realm_id, os)`,
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return exec(tx,
					`CREATE UNIQUE INDEX realm_app_name ON mobile_apps (name, realm_id)`,
					`ALTER TABLE mobile_apps DROP CONSTRAINT uix_name_realm_id_os UNIQUE (name, realm_id, os)`,
				)
			},
		},
		{
			ID: "00055-AddAuditEntries",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.Migrator().AutoMigrate(&AuditEntry{}); err != nil {
					return err
				}

				return exec(tx,
					`CREATE INDEX IF NOT EXISTS idx_audit_entries_realm_id ON audit_entries (realm_id)`,
					`CREATE INDEX IF NOT EXISTS idx_audit_entries_actor_id ON audit_entries (actor_id)`,
					`CREATE INDEX IF NOT EXISTS idx_audit_entries_target_id ON audit_entries (target_id)`,
					`CREATE INDEX IF NOT EXISTS idx_audit_entries_created_at ON audit_entries (created_at)`,
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return exec(tx,
					`DROP TABLE IF EXISTS audit_entries`,
				)
			},
		},
		{
			ID: "00056-AuthorzedAppsAPIKeyTypeBasis",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.Migrator().AutoMigrate(&AuditEntry{}); err != nil {
					return err
				}

				return exec(tx,
					`ALTER TABLE authorized_apps ALTER COLUMN api_key_type DROP DEFAULT`,
					`ALTER TABLE authorized_apps ALTER COLUMN api_key_type SET NOT NULL`,
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return nil
			},
		},
		{
			ID: "00057-AddMFARequiredGracePeriod",
			Migrate: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE realms ADD COLUMN IF NOT EXISTS mfa_required_grace_period BIGINT DEFAULT 0`,
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE realms DROP COLUMN IF EXISTS mfa_required_grace_period`,
				)
			},
		},
		{
			ID: "00058-AddAppStoreLink",
			Migrate: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE mobile_apps ADD COLUMN IF NOT EXISTS url TEXT`,
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return exec(tx,
					`ALTER TABLE realms DROP COLUMN IF EXISTS url`,
				)
			},
		},
		{
			ID: "00059-AddVerCodeIndexes",
			Migrate: func(tx *gorm.DB) error {
				return exec(tx,
					`CREATE INDEX IF NOT EXISTS idx_vercode_recent ON verification_codes(realm_id, issuing_user_id)`,
					`CREATE INDEX IF NOT EXISTS idx_vercode_uuid ON verification_codes(uuid)`,
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return exec(tx,
					`DROP INDEX IF EXISTS idx_vercode_recent`,
					`DROP INDEX IF EXISTS idx_vercode_uuid`,
				)
			},
		},
	})
}

// MigrateTo migrates the database to a specific target migration ID.
func (db *Database) MigrateTo(ctx context.Context, target string, rollback bool) error {
	logger := logging.FromContext(ctx).Named("migrate")
	ctx = logging.WithLogger(ctx, logger)

	m := db.getMigrations(ctx)

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
		logger.Errorw("failed to migrate", "error", err)
		return nil
	}
	return nil
}

// RunMigrations will apply sequential, transactional migrations to the database
func (db *Database) RunMigrations(ctx context.Context) error {
	logger := logging.FromContext(ctx).Named("migrate")
	ctx = logging.WithLogger(ctx, logger)

	m := db.getMigrations(ctx)
	if err := m.Migrate(); err != nil {
		logger.Errorw("failed to migrate", "error", err)
		return err
	}
	return nil
}

// exec is just like tx.Exec, except it runs all the provided SQLs.
func exec(tx *gorm.DB, sqls ...string) error {
	for _, sql := range sqls {
		if err := tx.Exec(sql).Error; err != nil {
			return fmt.Errorf("failed to run `%s`: %w", sql, err)
		}
	}
	return nil
}
