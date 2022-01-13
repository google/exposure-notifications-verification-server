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
	"strings"

	"github.com/google/exposure-notifications-verification-server/internal/project"
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
	ProviderType sms.ProviderType `gorm:"type:text"`

	// Twilio configuration options.
	TwilioAccountSid string `gorm:"type:text"`
	// E.164 format telephone number or
	// Twilio messaging service identifier see: https://support.twilio.com/hc/en-us/articles/223134387-What-is-a-Message-SID-
	TwilioFromNumber string `gorm:"type:text"`

	// E.164 format telephone number or
	// Twilio messaging service identifier see: https://support.twilio.com/hc/en-us/articles/223134387-What-is-a-Message-SID-
	// If present, used for user report only.
	TwilioUserReportFromNumber string `gorm:"type:text"`

	// TwilioAuthToken is encrypted/decrypted automatically by callbacks. The
	// cache fields exist as optimizations.
	TwilioAuthToken                string `gorm:"text" json:"-"` // ignored by zap's JSON formatter
	TwilioAuthTokenPlaintextCache  string `gorm:"-"`
	TwilioAuthTokenCiphertextCache string `gorm:"-"`

	// IsSystem determines if this is a system-level SMS configuration. There can
	// only be one system-level SMS configuration.
	IsSystem bool `gorm:"type:bool; not null; default:false;"`
}

func (s *SMSConfig) BeforeSave(tx *gorm.DB) error {
	// Twilio config is all or nothing
	if (s.TwilioAccountSid == "") != (s.TwilioAuthToken == "") {
		s.AddError("twilioAccountSid", "all must be specified or all must be blank")
		s.AddError("twilioAuthToken", "all must be specified or all must be blank")
	}

	if s.TwilioAccountSid != "" {
		phones := map[string]string{
			"twilioFromNumber":           s.TwilioFromNumber,
			"twilioUserReportFromNumber": s.TwilioUserReportFromNumber,
		}
		for k, fromNumber := range phones {
			if fromNumber != "" {
				switch {
				case strings.HasPrefix(fromNumber, sms.TwilioMessagingServiceSidPrefix):
					if len(fromNumber) != 34 {
						s.AddError(k, `a valid twilio messaging service sid should be 34 characters`)
					}
				case strings.HasPrefix(fromNumber, "+"):
					if !project.AllDigits(fromNumber[1:]) {
						s.AddError(k, `an E.164 format phone number should begin with "+" followed by digits`)
					}
				case len(fromNumber) <= 6:
					if !project.AllDigits(fromNumber) && len(fromNumber) <= 6 {
						s.AddError(k, `a short code should contain only digits`)
					}
				default:
					s.AddError(k, `an E.164 format phone number should begin with "+" followed by digits`)
				}
			}
		}
	}

	if s.IsSystem {
		// Do not persist from numbers for system configs
		s.TwilioFromNumber = ""
	}

	return s.ErrorOrNil()
}

// SystemSMSConfig returns the system SMS config, if one exists
func (db *Database) SystemSMSConfig() (*SMSConfig, error) {
	var smsConfig SMSConfig
	if err := db.db.
		Model(&SMSConfig{}).
		Where("is_system IS TRUE").
		First(&smsConfig).
		Error; err != nil {
		return nil, err
	}
	return &smsConfig, nil
}

// SaveSMSConfig creates or updates an SMS configuration record.
func (db *Database) SaveSMSConfig(s *SMSConfig) error {
	if s.ProviderType == sms.ProviderTypeTwilio &&
		s.TwilioAccountSid == "" && s.TwilioAuthToken == "" && s.TwilioFromNumber == "" {
		if db.db.NewRecord(s) {
			// The fields are all blank, do not create the record.
			return nil
		}

		if s.IsSystem {
			// We're about to delete the system SMS config, revoke everyone's
			// permissions to use it. You would think there'd be a way to do this with
			// gorm natively. You'd even find an example in the documentation that led
			// you to believe that gorm does this.
			//
			// Narrator: gorm does not do this.
			if err := db.db.
				Exec(`UPDATE realms SET can_use_system_sms_config = FALSE, use_system_sms_config = FALSE`).
				Error; err != nil {
				return err
			}
		}

		// The fields were blank, delete the SMS config.
		return db.db.Unscoped().Delete(s).Error
	}

	if db.db.NewRecord(s) {
		return db.db.Create(s).Error
	}
	return db.db.Save(s).Error
}
