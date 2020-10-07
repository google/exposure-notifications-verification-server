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
	"crypto/hmac"
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jinzhu/gorm"
)

const (
	oneDay = 24 * time.Hour
	// MinCodeLength defines the minimum number of digits in a code.
	MinCodeLength = 6
)

var (
	// ValidTestTypes is a map containing the valid test types.
	ValidTestTypes = map[string]struct{}{
		"confirmed": {},
		"likely":    {},
		"negative":  {},
	}

	ErrInvalidTestType    = errors.New("invalid test type, must be confirmed, likely, or negative")
	ErrCodeAlreadyExpired = errors.New("code already expired")
	ErrCodeTooShort       = errors.New("verification code must be at least 6 digits")
	ErrTestTooOld         = errors.New("test date is more than 14 day ago")
)

// VerificationCode represents a verification code in the database.
type VerificationCode struct {
	gorm.Model
	Errorable

	RealmID       uint   // VerificationCodes belong to exactly one realm when issued.
	Code          string `gorm:"type:varchar(512);unique_index"`
	LongCode      string `gorm:"type:varchar(512);unique_index"`
	UUID          string `gorm:"type:uuid;unique_index;default:null"`
	Claimed       bool   `gorm:"default:false"`
	TestType      string `gorm:"type:varchar(20)"`
	SymptomDate   *time.Time
	ExpiresAt     time.Time
	LongExpiresAt time.Time
	IssuingUserID uint
	IssuingAppID  uint
}

// TableName sets the VerificationCode table name
func (VerificationCode) TableName() string {
	return "verification_codes"
}

// AfterCreate runs after the verification code has been saved, primarily used
// to update statistics about usage. If the executions fail, an error is logged
// but the transaction continues. This is called automatically by gorm.
func (v *VerificationCode) AfterCreate(scope *gorm.Scope) {
	date := v.CreatedAt.Truncate(24 * time.Hour)

	// If the issuer was a user, update the user stats for the day.
	if v.IssuingUserID != 0 {
		sql := `
			INSERT INTO user_stats (date, realm_id, user_id, codes_issued)
				VALUES ($1, $2, $3, 1)
			ON CONFLICT (date, realm_id, user_id) DO UPDATE
				SET codes_issued = user_stats.codes_issued + 1
		`

		if err := scope.DB().Exec(sql, date, v.RealmID, v.IssuingUserID).Error; err != nil {
			scope.Log(fmt.Sprintf("failed to update stats: %v", err))
		}
	}

	// If the issuer was a app, update the app stats for the day.
	if v.IssuingAppID != 0 {
		sql := `
			INSERT INTO authorized_app_stats (date, authorized_app_id, codes_issued)
				VALUES ($1, $2, 1)
			ON CONFLICT (date, authorized_app_id) DO UPDATE
				SET codes_issued = authorized_app_stats.codes_issued + 1
		`

		if err := scope.DB().Exec(sql, date, v.IssuingAppID).Error; err != nil {
			scope.Log(fmt.Sprintf("failed to update stats: %v", err))
		}
	}

	// Update the per-realm stats.
	if v.RealmID != 0 {
		sql := `
			INSERT INTO realm_stats(date, realm_id, codes_issued)
				VALUES ($1, $2, 1)
			ON CONFLICT (date, realm_id) DO UPDATE
				SET codes_issued = realm_stats.codes_issued + 1
		`

		if err := scope.DB().Exec(sql, date, v.RealmID).Error; err != nil {
			scope.Log(fmt.Sprintf("failed to update stats: %v", err))
		}
	}
}

// TODO(mikehelmick) - Add method to soft delete expired codes
// TODO(mikehelmick) - Add method to purge verification codes that are > XX hours old
//   Keeping expired codes prevents a code from being regenerated during that period of time.

// FormatSymptomDate returns YYYY-MM-DD formatted test date, or "" if nil.
func (v *VerificationCode) FormatSymptomDate() string {
	if v.SymptomDate == nil {
		return ""
	}
	return v.SymptomDate.Format("2006-01-02")
}

// IsCodeExpired checks to see if the actual code provided is the short or long
// code, and determines if it is expired based on that.
func (db *Database) IsCodeExpired(v *VerificationCode, code string) (bool, error) {
	if v == nil {
		return false, fmt.Errorf("provided code is nil")
	}

	// It's possible that this could be called with the already HMACd version.
	possibles, err := db.generateVerificationCodeHMACs(code)
	if err != nil {
		return false, fmt.Errorf("failed to create hmac: %w", err)
	}
	possibles = append(possibles, code)

	inList := func(needle string, haystack []string) bool {
		for _, hay := range haystack {
			if hay == needle {
				return true
			}
		}
		return false
	}

	now := time.Now().UTC()
	switch {
	case inList(v.Code, possibles):
		return !v.ExpiresAt.After(now), nil
	case inList(v.LongCode, possibles):
		return !v.LongExpiresAt.After(now), nil
	default:
		return true, fmt.Errorf("not found")
	}
}

// IsExpired returns true if a verification code has expired.
func (v *VerificationCode) IsExpired() bool {
	now := time.Now().UTC()
	return v.ExpiresAt.Before(now) && v.LongExpiresAt.Before(now)
}

func (v *VerificationCode) HasLongExpiration() bool {
	return v.LongExpiresAt.After(v.ExpiresAt)
}

// Validate validates a verification code before save.
func (v *VerificationCode) Validate(maxAge time.Duration) error {
	now := time.Now()
	v.Code = strings.TrimSpace(v.Code)
	if len(v.Code) < MinCodeLength {
		return ErrCodeTooShort
	}
	v.LongCode = strings.TrimSpace(v.LongCode)
	if len(v.LongCode) < MinCodeLength {
		return ErrCodeTooShort
	}
	if _, ok := ValidTestTypes[v.TestType]; !ok {
		return ErrInvalidTestType
	}
	if v.SymptomDate != nil {
		minSymptomDate := now.Add(-1 * maxAge).Truncate(oneDay)
		if minSymptomDate.After(*v.SymptomDate) {
			return ErrTestTooOld
		}
	}
	if now.After(v.ExpiresAt) || now.After(v.LongExpiresAt) {
		return ErrCodeAlreadyExpired
	}
	return nil
}

// FindVerificationCode find a verification code by the code number (can be short
// code or long code).
func (db *Database) FindVerificationCode(code string) (*VerificationCode, error) {
	hmacedCodes, err := db.generateVerificationCodeHMACs(code)
	if err != nil {
		return nil, fmt.Errorf("failed to create hmac: %w", err)
	}

	var vc VerificationCode
	if err := db.db.
		Where("code IN (?) OR long_code IN (?)", hmacedCodes, hmacedCodes).
		First(&vc).
		Error; err != nil {
		return nil, err
	}
	return &vc, nil
}

// FindVerificationCodeByUUID find a verification codes by UUID.
func (db *Database) FindVerificationCodeByUUID(uuid string) (*VerificationCode, error) {
	var vc VerificationCode
	if err := db.db.Where("uuid = ?", uuid).Find(&vc).Error; err != nil {
		return nil, err
	}
	return &vc, nil
}

// ListRecentCodes shows the last 5 recently issued codes for a given issuing user.
// The code and longCode are removed, this is only intended to show metadata.
func (db *Database) ListRecentCodes(realm *Realm, user *User) ([]*VerificationCode, error) {
	var codes []*VerificationCode
	if err := db.db.
		Model(&VerificationCode{}).
		Where("realm_id = ? AND issuing_user_id = ?", realm.ID, user.ID).
		Order("created_at DESC").
		Limit(5).
		Find(&codes).
		Error; err != nil {
		return nil, err
	}

	// We're only showing meta details, not the encrypted codes.
	for _, t := range codes {
		if t.Code != "" {
			t.Code = "short"
		}
		if t.LongCode != "" {
			t.LongCode = "long"
		}
	}

	return codes, nil
}

// ExpireCode saves a verification code as expired.
func (db *Database) ExpireCode(uuid string) (*VerificationCode, error) {
	var vc VerificationCode
	err := db.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Set("gorm:query_option", "FOR UPDATE").
			Where("uuid = ?", uuid).
			Find(&vc).Error; err != nil {
			return err
		}
		if vc.IsExpired() {
			return errors.New("code already expired")
		}
		if vc.Claimed {
			return errors.New("code already caimed")
		}

		vc.ExpiresAt = time.Now()
		vc.LongExpiresAt = vc.ExpiresAt
		return tx.Save(&vc).Error
	})
	if err != nil {
		return nil, err
	}
	return &vc, nil
}

// SaveVerificationCode created or updates a verification code in the database.
// Max age represents the maximum age of the test date [optional] in the record.
func (db *Database) SaveVerificationCode(vc *VerificationCode, maxAge time.Duration) error {
	if err := vc.Validate(maxAge); err != nil {
		return err
	}
	if vc.Model.ID == 0 {
		return db.db.Create(vc).Error
	}
	return db.db.Save(vc).Error
}

// DeleteVerificationCode deletes the code if it exists. This is a hard delete.
func (db *Database) DeleteVerificationCode(code string) error {
	hmacedCodes, err := db.generateVerificationCodeHMACs(code)
	if err != nil {
		return fmt.Errorf("failed to create hmac: %w", err)
	}

	return db.db.Unscoped().
		Where("code IN (?) OR long_code IN (?)", hmacedCodes, hmacedCodes).
		Delete(&VerificationCode{}).
		Error
}

// PurgeVerificationCodes will delete verifications that have expired since at least the
// provided maxAge ago.
// This is a hard delete, not a soft delete.
func (db *Database) PurgeVerificationCodes(maxAge time.Duration) (int64, error) {
	if maxAge > 0 {
		maxAge = -1 * maxAge
	}
	deleteBefore := time.Now().UTC().Add(maxAge)
	// Delete codes that expired before the delete before time.
	rtn := db.db.Unscoped().Where("expires_at < ? AND long_expires_at < ?", deleteBefore, deleteBefore).Delete(&VerificationCode{})
	return rtn.RowsAffected, rtn.Error
}

// GenerateVerificationCodeHMAC generates the HMAC of the code using the latest
// key.
func (db *Database) GenerateVerificationCodeHMAC(verCode string) (string, error) {
	keys := db.config.VerificationCodeDatabaseHMAC
	if len(keys) < 1 {
		return "", fmt.Errorf("expected at least 1 hmac key")
	}
	sig := hmac.New(sha512.New, keys[0])
	if _, err := sig.Write([]byte(verCode)); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(sig.Sum(nil)), nil
}

// generateVerificationCodeHMACs is a helper for generating all possible HMACs of a
// token.
func (db *Database) generateVerificationCodeHMACs(v string) ([]string, error) {
	sigs := make([]string, 0, len(db.config.VerificationCodeDatabaseHMAC))
	for _, key := range db.config.VerificationCodeDatabaseHMAC {
		sig := hmac.New(sha512.New, key)
		if _, err := sig.Write([]byte(v)); err != nil {
			return nil, err
		}
		sigs = append(sigs, base64.RawURLEncoding.EncodeToString(sig.Sum(nil)))
	}
	return sigs, nil
}
