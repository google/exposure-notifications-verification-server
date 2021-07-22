// Copyright 2020 the Exposure Notifications Verification Server authors
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

	"github.com/google/exposure-notifications-verification-server/pkg/email"
	"github.com/jinzhu/gorm"
)

// EmailConfig represents and email configuration.
type EmailConfig struct {
	gorm.Model
	Errorable

	// email Config belongs to exactly one realm.
	RealmID uint `gorm:"type:integer"`

	// ProviderType is the email provider type - it's used to determine the
	// underlying configuration.
	ProviderType email.ProviderType `gorm:"type:varchar(100)"`

	SMTPAccount string `gorm:"type:varchar(250)"`
	SMTPHost    string `gorm:"type:varchar(250)"`
	SMTPPort    string `gorm:"type:varchar(250)"`

	// SMTPPassword is encrypted/decrypted automatically by callbacks. The
	// cache fields exist as optimizations.
	SMTPPassword                string `gorm:"type:varchar(250)" json:"-"` // ignored by zap's JSON formatter
	SMTPPasswordPlaintextCache  string `gorm:"-"`
	SMTPPasswordCiphertextCache string `gorm:"-"`

	// IsSystem determines if this is a system-level email configuration. There can
	// only be one system-level email configuration.
	IsSystem bool `gorm:"type:bool; not null; default:false;"`
}

func (e *EmailConfig) BeforeSave(tx *gorm.DB) error {
	// Email config is all or nothing
	if (e.SMTPAccount != "" || e.SMTPPassword != "" || e.SMTPHost != "") &&
		(e.SMTPAccount == "" || e.SMTPPassword == "" || e.SMTPHost == "") {
		e.AddError("SMTPAccount", "all must be specified or all must be blank")
		e.AddError("SMTPPassword", "all must be specified or all must be blank")
		e.AddError("SMTPHost", "all must be specified or all must be blank")
	}

	return e.ErrorOrNil()
}

func (e *EmailConfig) Provider() (email.Provider, error) {
	ctx := context.Background()
	provider, err := email.ProviderFor(ctx, &email.Config{
		ProviderType: e.ProviderType,
		User:         e.SMTPAccount,
		Password:     e.SMTPPassword,
		SMTPHost:     e.SMTPHost,
		SMTPPort:     e.SMTPPort,
	})
	if err != nil {
		return nil, err
	}
	return provider, nil
}

// SystemEmailConfig returns the system email config, if one exists
func (db *Database) SystemEmailConfig() (*EmailConfig, error) {
	var emailConfig EmailConfig
	if err := db.db.
		Model(&EmailConfig{}).
		Where("is_system IS TRUE").
		First(&emailConfig).
		Error; err != nil {
		return nil, err
	}
	return &emailConfig, nil
}

// SaveEmailConfig creates or updates an email configuration record.
func (db *Database) SaveEmailConfig(s *EmailConfig) error {
	if s.SMTPAccount == "" && s.SMTPPassword == "" && s.SMTPHost == "" {
		if db.db.NewRecord(s) {
			// The fields are all blank, do not create the record.
			return nil
		}

		if s.IsSystem {
			// We're about to delete the system email config, revoke everyone's
			// permissions to use it. You would think there'd be a way to do this with
			// gorm natively. You'd even find an example in the documentation that led
			// you to believe that gorm does this.
			//
			// Narrator: gorm does not do this.
			if err := db.db.
				Exec(`UPDATE realms SET can_use_system_email_config = FALSE, use_system_email_config = FALSE`).
				Error; err != nil {
				return err
			}
		}

		// The fields were blank, delete the email config.
		return db.db.Unscoped().Delete(s).Error
	}

	return db.db.Save(s).Error
}
