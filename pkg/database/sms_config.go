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
	"fmt"

	"github.com/google/exposure-notifications-verification-server/pkg/sms"
	"github.com/jinzhu/gorm"
)

// SMSConfig represents and SMS configuration.
type SMSConfig struct {
	gorm.Model
	Errorable

	// SMS Config belongs to exactly one realm.
	RealmID uint

	// ProviderType is the SMS provider type - it's used to determine the
	// underlying configuration.
	ProviderType sms.ProviderType `gorm:"type:varchar(100)"`

	// Twilio configuration options.
	TwilioAccountSid string `gorm:"type:varchar(250)"`
	TwilioFromNumber string `gorm:"type:varchar(16)"`

	// TwilioAuthToken is encrypted/decrypted automatically by callbacks. The
	// cache fields exist as optimizations.
	TwilioAuthToken                string `gorm:"type:varchar(250)"`
	TwilioAuthTokenPlaintextCache  string `gorm:"-"`
	TwilioAuthTokenCiphertextCache string `gorm:"-"`

	// IsSystem determines if this is a system-level SMS configuration. There can
	// only be one system-level SMS configuration.
	IsSystem bool
}

func (s *SMSConfig) BeforeSave(tx *gorm.DB) error {
	// Twilio config is all or nothing
	if (s.TwilioAccountSid != "" || s.TwilioAuthToken != "" || s.TwilioFromNumber != "") &&
		(s.TwilioAccountSid == "" || s.TwilioAuthToken == "" || s.TwilioFromNumber == "") {
		s.AddError("twilioAccountSid", "all must be specified or all must be blank")
		s.AddError("twilioAuthToken", "all must be specified or all must be blank")
		s.AddError("twilioFromNumber", "all must be specified or all must be blank")
	}

	if len(s.Errors()) > 0 {
		return fmt.Errorf("validation failed")
	}
	return nil
}

// SaveSMSConfig creates or updates an SMS configuration record.
func (db *Database) SaveSMSConfig(s *SMSConfig) error {
	if s.TwilioAccountSid == "" && s.TwilioAuthToken == "" && s.TwilioFromNumber == "" {
		if db.db.NewRecord(s) {
			// The fields are all blank, do not create the record.
			return nil
		}

		// The fields were blank, delete the SMS config.
		return db.db.Unscoped().Delete(s).Error
	}

	if db.db.NewRecord(s) {
		return db.db.Create(s).Error
	}
	return db.db.Save(s).Error
}

// DeleteSMSConfig removes an SMS configuration record.
func (db *Database) DeleteSMSConfig(s *SMSConfig) error {
	return db.db.Unscoped().Delete(s).Error
}
