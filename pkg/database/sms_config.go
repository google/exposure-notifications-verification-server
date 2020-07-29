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

	"github.com/google/exposure-notifications-verification-server/pkg/sms"
	"github.com/jinzhu/gorm"
)

// SMSConfig represents and SMS configuration.
type SMSConfig struct {
	gorm.Model

	// SMS Config belongs to exactly one realm.
	RealmID uint `gorm:"unique_index"`

	// ProviderType is the SMS provider type - it's used to determine the
	// underlying configuration.
	ProviderType sms.ProviderType `gorm:"type:varchar(100)"`

	// Twilio configuration options.
	TwilioAccountSid string `gorm:"type:varchar(250)"`
	TwilioAuthToken  string `gorm:"type:varchar(250)"` // secret reference
	TwilioFromNumber string `gorm:"type:varchar(16)"`

	twilioAuthSecret string
}

// GetSMSProvider gets the SMS provider for the given realm. The values are
// cached.
func (smsConfig *SMSConfig) GetSMSProvider(ctx context.Context, db *Database) (sms.Provider, error) {
	// The provider itself is cached, not the SMSConfig entity.
	key := fmt.Sprintf("GetSMSProvider/%v", smsConfig.ID)
	val, err := db.cacher.WriteThruLookup(key, func() (interface{}, error) {
		// Get the actual auth token from the secret manager.
		//
		// TODO(sethvargo): this is a double switch, since ProviderFor is also a
		// switch. Maybe we should push this logic down lower? We only want to
		// resolve secrets for the provider that's configured.
		switch smsConfig.ProviderType {
		case sms.ProviderTypeTwilio:
			authToken, err := db.secretManager.GetSecretValue(ctx, smsConfig.TwilioAuthToken)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve twilio auth token secret: %w", err)
			}
			smsConfig.twilioAuthSecret = authToken
		}

		provider, err := sms.ProviderFor(ctx, &sms.Config{
			ProviderType:     smsConfig.ProviderType,
			TwilioAccountSid: smsConfig.TwilioAccountSid,
			TwilioAuthToken:  smsConfig.twilioAuthSecret,
			TwilioFromNumber: smsConfig.TwilioFromNumber,
		})
		if err != nil {
			return nil, err
		}
		return provider, nil
	})
	if err != nil {
		return nil, err
	}

	if val == nil {
		return nil, nil
	}

	return val.(sms.Provider), nil
}

// SaveSMSConfig creates or updates an SMS configuration record.
func (db *Database) SaveSMSConfig(s *SMSConfig) error {
	if s.Model.ID == 0 {
		return db.db.Create(s).Error
	}
	return db.db.Save(s).Error
}

// DeleteSMSConfig removes an SMS configuration record.
func (db *Database) DeleteSMSConfig(s *SMSConfig) error {
	return db.db.Unscoped().Delete(s).Error
}
