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
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/exposure-notifications-server/pkg/base64util"
	"github.com/google/exposure-notifications-verification-server/pkg/sms"
	"github.com/jinzhu/gorm"
)

// SMSConfig represents and SMS configuration.
type SMSConfig struct {
	db *Database `gorm:"-"`

	gorm.Model

	// SMS Config belongs to exactly one realm.
	RealmID uint `gorm:"unique_index"`
	Realm   Realm

	// ProviderType is the SMS provider type - it's used to determine the
	// underlying configuration.
	ProviderType sms.ProviderType `gorm:"type:varchar(100)"`

	// Twilio configuration options.
	TwilioAccountSid string `gorm:"type:varchar(250)"`
	TwilioAuthToken  string `gorm:"type:varchar(250)"`
	TwilioFromNumber string `gorm:"type:varchar(16)"`

	// twilioAuthToken and twilioAuthTokenEncrypted are cached values of the auth
	// token. It's used to compare whether the value changed to avoid unnecessary
	// calls the KMS.
	twilioAuthToken          string `gorm:"-"`
	twilioAuthTokenEncrypted string `gorm:"-"`
}

// BeforeSave runs before records are saved/created. It's used to mutate values
// (such as the auth tokens) before storing them in the database. Do not call
// this function, gorm calls it automatically.
func (r *SMSConfig) BeforeSave(tx *gorm.DB) error {
	ctx, done := context.WithTimeout(context.Background(), 5*time.Second)
	defer done()

	if plaintext := r.TwilioAuthToken; plaintext != "" {
		// Don't encrypt again if the value hasn't changed.
		if plaintext == r.twilioAuthToken {
			r.TwilioAuthToken = r.twilioAuthTokenEncrypted
			return nil
		}

		keyID := r.db.config.EncryptionKey
		b, err := r.db.keyManager.Encrypt(ctx, keyID, []byte(plaintext), nil)
		if err != nil {
			return fmt.Errorf("failed to encrypt twilio auth token: %w", err)
		}
		r.TwilioAuthToken = base64.RawStdEncoding.EncodeToString(b)
	}
	return nil
}

// AfterFind runs after a record is found. It's used to mutate values (such as
// auth tokens) before storing them on the struct. Do not call this function,
// gorm calls it automatically.
func (r *SMSConfig) AfterFind() error {
	ctx, done := context.WithTimeout(context.Background(), 5*time.Second)
	defer done()

	if ciphertextStr := r.TwilioAuthToken; ciphertextStr != "" {
		ciphertext, err := base64util.DecodeString(ciphertextStr)
		if err != nil {
			return fmt.Errorf("failed to decode twilio auth token: %w", err)
		}

		keyID := r.db.config.EncryptionKey
		plaintext, err := r.db.keyManager.Decrypt(ctx, keyID, []byte(ciphertext), nil)
		if err != nil {
			return fmt.Errorf("failed to decrypt twilio auth token: %w", err)
		}
		r.TwilioAuthToken = string(plaintext)
		r.twilioAuthToken = string(plaintext)
		r.twilioAuthTokenEncrypted = ciphertextStr
	}

	return nil
}

// SMSProvider gets the SMS provider for the given realm. The values are
// cached.
func (r *SMSConfig) SMSProvider() (sms.Provider, error) {
	key := fmt.Sprintf("GetSMSProvider/%v", r.ID)
	val, err := r.db.cacher.WriteThruLookup(key, func() (interface{}, error) {
		ctx := context.Background()
		provider, err := sms.ProviderFor(ctx, &sms.Config{
			ProviderType:     r.ProviderType,
			TwilioAccountSid: r.TwilioAccountSid,
			TwilioAuthToken:  r.twilioAuthToken,
			TwilioFromNumber: r.TwilioFromNumber,
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
	s.db = db

	if s.Model.ID == 0 {
		return db.db.Create(s).Error
	}
	return db.db.Save(s).Error
}

// DeleteSMSConfig removes an SMS configuration record.
func (db *Database) DeleteSMSConfig(s *SMSConfig) error {
	s.db = db

	return db.db.Unscoped().Delete(s).Error
}
