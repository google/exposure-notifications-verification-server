// Copyright 2021 the Exposure Notifications Verification Server authors
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
	"time"

	"github.com/google/exposure-notifications-server/pkg/base64util"
	"github.com/jinzhu/gorm"
)

const (
	// NonceLength is the required length of an associated user-report request.
	// Changing this could break outstanding codes in the system.
	// If the value were to be lowered, uses should change to >= instead of exact match,
	// including updating associated documentation.
	NonceLength = 256
)

var _ Auditable = (*UserReport)(nil)

// UserReport is used to de-duplicate phone numbers for user-initiated reporting.
type UserReport struct {
	Errorable

	// ID is an auto-increment primary key
	ID uint

	// PhoneHash is the base64 encoded HMAC of the phone number used to create a user report
	PhoneHash string `json:"-"` // unique
	// Nonce is the random data that must be presented when verifying a verification code attached to this user report
	Nonce string
	// NonceRequired indicates if this is request requires a nonce, some do not if issued by a PHA web site for example.
	NonceRequired bool

	// CodeClaimed is set to true when the associated code is claimed. This is needed
	// since the verification code itself will be cleaned up before this record.
	CodeClaimed bool

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (ur *UserReport) AuditID() string {
	return fmt.Sprintf("user_report:%d", ur.ID)
}

func (ur *UserReport) AuditDisplay() string {
	phoneHash := ur.PhoneHash
	if len(phoneHash) > 9 {
		phoneHash = phoneHash[0:8]
	}
	return fmt.Sprintf("%s (claimed: %t)", phoneHash, ur.CodeClaimed)
}

// NewUserReport creates a new UserReport by calculating the current HMAC of the
// provided phone number and encoding the nonce. It does NOT save it to the database.
func (db *Database) NewUserReport(phone string, nonce []byte, nonceRequired bool) (*UserReport, error) {
	hmac, err := db.GeneratePhoneNumberHMAC(phone)
	if err != nil {
		return nil, err
	}

	nonceB64 := base64.StdEncoding.EncodeToString(nonce)

	return &UserReport{
		PhoneHash:     hmac,
		Nonce:         nonceB64,
		NonceRequired: nonceRequired,
	}, nil
}

// BeforeSave validates the structure of the UserReport.
func (ur *UserReport) BeforeSave(tx *gorm.DB) error {
	if ur.NonceRequired {
		decoded, err := base64util.DecodeString(ur.Nonce)
		if err != nil {
			ur.AddError("nonce", "is not using a valid base64 encoding")
		} else {
			if l := len(decoded); l != NonceLength {
				ur.AddError("nonce", fmt.Sprintf("is not the correct length, want: %v got: %v", NonceLength, l))
			}
		}
	} else {
		if ur.Nonce != "" {
			ur.AddError("nonce", "is not required for this request and should not be sent")
		}
	}

	return ur.ErrorOrNil()
}

// FindUserReportInTx finds a user report by phone number using any of the currently valid
// HMAC keys.
func (db *Database) FindUserReportInTx(tx *gorm.DB, phoneNumber string) (*UserReport, error) {
	hmacedCodes, err := db.generatePhoneNumberHMACs(phoneNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to create hmac: %w", err)
	}

	var ur UserReport
	if err := tx.
		Where("phone_hash IN (?)", hmacedCodes).
		First(&ur).
		Error; err != nil {
		return nil, fmt.Errorf("failed to find user report: %w", err)
	}
	return &ur, nil
}

// FindUserReport finds a user report by phone number using any of the currently valid
// HMAC keys.
func (db *Database) FindUserReport(phoneNumber string) (*UserReport, error) {
	return db.FindUserReportInTx(db.db, phoneNumber)
}

// DeleteUserReport removes a specific phone number from the user report
// de-duplication table.
func (db *Database) DeleteUserReport(phoneNumber string, actor Auditable) error {
	hmacedCodes, err := db.generatePhoneNumberHMACs(phoneNumber)
	if err != nil {
		return fmt.Errorf("failed to create hmac: %w", err)
	}
	if actor == nil {
		return ErrMissingActor
	}

	return db.db.Transaction(func(tx *gorm.DB) error {
		var ur UserReport
		if err := tx.
			Set("gorm:query_option", "FOR UPDATE").
			Model(&UserReport{}).
			Where("phone_hash IN (?)", hmacedCodes).
			First(&ur).
			Error; err != nil {
			if IsNotFound(err) {
				// Nothing to do - return success.
				return nil
			}
			return fmt.Errorf("failed to find user report: %w", err)
		}

		vc := &VerificationCode{}
		if err := tx.
			Set("gorm:query_option", "FOR UPDATE").
			Model(&VerificationCode{}).
			Where("user_report_id = ?", ur.ID).
			First(vc).
			Error; err != nil {
			if IsNotFound(err) {
				vc = nil
			} else {
				return fmt.Errorf("failed to find related verification codes: %w", err)
			}
		}
		if vc != nil && !vc.IsExpired() {
			// The associated code needs to be expired so that
			// the code can't be used by anyone else.
			exp := time.Now().UTC()
			vc.ExpiresAt = exp
			vc.LongExpiresAt = exp
			if err := tx.Save(vc).Error; err != nil {
				return fmt.Errorf("failed to expire code: %w", err)
			}
		}

		if !IsNullActor(actor) {
			audit := BuildAuditEntry(actor, "purged user report phone", &ur, 0)
			if err := tx.Save(audit).Error; err != nil {
				return fmt.Errorf("failed to save audits: %w", err)
			}
		}

		return tx.Delete(&ur).Error
	})
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

// PurgeClaimedUserReports removes expired user reports.
func (db *Database) PurgeClaimedUserReports(maxAge time.Duration) (int64, error) {
	if maxAge > 0 {
		maxAge = -1 * maxAge
	}
	deleteBefore := time.Now().UTC().Add(maxAge)
	rtn := db.db.Unscoped().
		Where("created_at < ?", deleteBefore).
		Delete(&UserReport{})
	return rtn.RowsAffected, rtn.Error
}

// GeneratePhoneNumberHMAC generates the HMAC of the phone number using the latest key.
func (db *Database) GeneratePhoneNumberHMAC(phoneNumber string) (string, error) {
	keys, err := db.GetPhoneNumberDatabaseHMAC()
	if err != nil {
		return "", fmt.Errorf("failed to get keys to generate phone number database HMAC: %w", err)
	}

	s, err := initialHMAC(keys, phoneNumber)
	if err != nil {
		return "", fmt.Errorf("failed to generate phone number HMAC: %w", err)
	}
	return s, nil
}

// generatePhoneNumberHMACs is a helper for generating all possible HMACs of a phone number.
func (db *Database) generatePhoneNumberHMACs(phoneNumber string) ([]string, error) {
	keys, err := db.GetPhoneNumberDatabaseHMAC()
	if err != nil {
		return nil, fmt.Errorf("failed to get keys to generate phone number database HMACs: %w", err)
	}

	return allAllowedHMACs(keys, phoneNumber)
}
