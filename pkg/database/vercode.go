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
	"errors"
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

	ErrInvalidTestType    = errors.New("invalid test tyupe, must be confirmed, likely, or negative")
	ErrCodeAlreadyExpired = errors.New("code already expired")
	ErrCodeTooShort       = errors.New("verification code must be at least 6 digits")
	ErrTestTooOld         = errors.New("test date is more than 14 day ago")
)

// VerificationCode represnts a verification code in the database.
type VerificationCode struct {
	gorm.Model
	Code        string `gorm:"type:varchar(20);unique_index"`
	Claimed     bool   `gorm:"default:false"`
	TestType    string `gorm:"type:varchar(20)"`
	SymptomDate *time.Time
	ExpiresAt   time.Time
	IssuingUser *User
	IssuingApp  *AuthorizedApp
}

// TableName sets the VerificationCode table name
func (VerificationCode) TableName() string {
	return "verification_codes"
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

// IsExpired returns ture if a verification code has expired.
func (v *VerificationCode) IsExpired() bool {
	return !v.ExpiresAt.After(time.Now())
}

// Validate validates a verification code before save.
func (v *VerificationCode) Validate(maxAge time.Duration) error {
	v.Code = strings.TrimSpace(v.Code)
	if len(v.Code) < MinCodeLength {
		return ErrCodeTooShort
	}
	if _, ok := ValidTestTypes[v.TestType]; !ok {
		return ErrInvalidTestType
	}
	if v.SymptomDate != nil {
		minSymptomDate := time.Now().Add(-1 * maxAge).Truncate(oneDay)
		if minSymptomDate.After(*v.SymptomDate) {
			return ErrTestTooOld
		}
	}
	if !v.ExpiresAt.After(time.Now()) {
		return ErrCodeAlreadyExpired
	}
	return nil
}

// FindVerificationCode find a verification code by the code number.
func (db *Database) FindVerificationCode(code string) (*VerificationCode, error) {
	var vc VerificationCode
	if err := db.db.Where("code = ?", code).First(&vc).Error; err != nil {
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

// PurgeVerificationCodes will delete verifications that have expired since at least the
// provided maxAge ago.
// This is a hard delete, not a soft delete.
func (db *Database) PurgeVerificationCodes(maxAge time.Duration) (int64, error) {
	if maxAge > 0 {
		maxAge = -1 * maxAge
	}
	deleteBefore := time.Now().UTC().Add(maxAge)
	// Delete codes that expired before the delete before time.
	rtn := db.db.Unscoped().Where("expires_at < ?", deleteBefore).Delete(&VerificationCode{})
	return rtn.RowsAffected, rtn.Error
}
