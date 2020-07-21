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
	"errors"
	"fmt"

	"github.com/google/exposure-notifications-verification-server/pkg/sms"
	"github.com/jinzhu/gorm"
)

// SMSConfig represents and SMS configuration.
type SMSConfig struct {
	gorm.Model

	// ProviderType is the SMS provider type - it's used to determine the
	// underlying configuration.
	ProviderType sms.ProviderType `gorm:"type:varchar(100)"`

	// Twilio configuration options.
	TwilioAccountSid string `gorm:"type:varchar(250)"`
	TwilioAuthToken  string `gorm:"type:varchar(250)"`
	TwilioFromNumber string `gorm:"type:varchar(16)"`
}

// GetSMSProvider gets the SMS provider for the given realm. The values are
// cached.
func (db *Database) GetSMSProvider(realm string) (sms.Provider, error) {
	key := fmt.Sprintf("GetSMSProvider/%s", realm)
	val, err := db.cacher.WriteThruLookup(key, func() (interface{}, error) {
		smsConfig, err := db.FindSMSConfig(realm)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, nil
			}
			return nil, err
		}

		ctx := context.Background()
		provider, err := sms.ProviderFor(ctx, &sms.Config{
			ProviderType:     smsConfig.ProviderType,
			TwilioAccountSid: smsConfig.TwilioAccountSid,
			TwilioAuthToken:  smsConfig.TwilioAuthToken,
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

// FindSMSConfig gets the SMS configuration for the given realm.
func (db *Database) FindSMSConfig(realm string) (*SMSConfig, error) {
	var smsConfig SMSConfig

	// TODO(sethvargo): support realms and lookup SMS configuration by realm.
	if err := db.db.First(&smsConfig).Error; err != nil {
		return nil, err
	}
	return &smsConfig, nil
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
	return db.db.Delete(s).Error
}
