// Copyright 2022 Google LLC
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
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/pkg/base64util"
	"github.com/jinzhu/gorm"
)

const (
	NonceLength = 256
)

// UserReport is used to de-duplicate phone numbers for user-initiated reporting.
type UserReport struct {
	Errorable

	ID uint

	PhoneHash string `json:"-"` // unique
	Nonce     string

	CodeClaimed bool

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (db *Database) NewUserReport(phone string, nonce []byte) (*UserReport, error) {
	hmac, err := db.GeneratePhoneNumberHMAC(phone)
	if err != nil {
		return nil, err
	}

	nonceB64 := base64.StdEncoding.EncodeToString(nonce)

	return &UserReport{
		PhoneHash: hmac,
		Nonce:     nonceB64,
	}, nil
}

func (ur *UserReport) BeforeSave(tx *gorm.DB) error {
	decoded, err := base64util.DecodeString(ur.Nonce)
	if err != nil {
		ur.AddError("nonce", "is not using a valid base64 encoding")
	}

	if l := len(decoded); l != NonceLength {
		ur.AddError("nonce", fmt.Sprintf("is not the correct length, want: %v got: %v", NonceLength, l))
	}

	if m := ur.ErrorMessages(); len(m) > 0 {
		return fmt.Errorf("validation failed: %s", strings.Join(m, ", "))
	}

	return nil
}

func (db *Database) FindUserReport(tx *gorm.DB, phoneNumber string) (*UserReport, error) {
	hmacedCodes, err := db.generatePhoneNumberHMACs(phoneNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to create hmac: %w", err)
	}

	var ur UserReport
	if err := tx.
		Where("phone_hash IN (?)", hmacedCodes).
		First(&ur).
		Error; err != nil {
		return nil, err
	}
	return &ur, nil
}

// PurgeUnclaimedUserReports deletes record from the database
// if the phone number was used in a user report, but the code was never claimed.
func (db *Database) PurgeUnclaimedUserReports(maxAge time.Duration) (int64, error) {
	if maxAge > 0 {
		maxAge = -1 * maxAge
	}
	deleteBefore := time.Now().UTC().Add(maxAge)
	rtn := db.db.Unscoped().
		Where("created_at < ? AND code_claimed = ?", deleteBefore, false).
		Delete(&UserReport{})
	return rtn.RowsAffected, rtn.Error
}

// PurgeUserReports removes expired user reports.
func (db *Database) PurgeUserReports(maxAge time.Duration) (int64, error) {
	deleteBefore := time.Now().UTC()
	rtn := db.db.Unscoped().
		Where("created_at < ?", deleteBefore).
		Delete(&UserReport{})
	return rtn.RowsAffected, rtn.Error
}

// GeneratePhoneNumberHMAC generates the HMAC of the phone number using the latest key.
func (db *Database) GeneratePhoneNumberHMAC(phoneNumber string) (string, error) {
	return initialHMAC(db.config.PhoneNumberHMAC, phoneNumber)
}

// generatePhoneNumberHMACs is a helper for generating all possible HMACs of a phone number.
func (db *Database) generatePhoneNumberHMACs(phoneNumber string) ([]string, error) {
	return allAllowedHMACs(db.config.PhoneNumberHMAC, phoneNumber)
}
