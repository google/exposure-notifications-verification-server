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
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/project"

	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-server/pkg/timeutils"

	"github.com/jinzhu/gorm"
)

const (
	oneDay = 24 * time.Hour

	// MinCodeLength defines the minimum number of digits in a code.
	MinCodeLength = 6
)

type CodeType int

const (
	_ CodeType = iota
	CodeTypeShort
	CodeTypeLong
)

var (
	// ValidTestTypes is a map containing the valid test types.
	ValidTestTypes = map[string]struct{}{
		"confirmed":   {},
		"likely":      {},
		"negative":    {},
		"user-report": {},
	}

	ErrInvalidTestType     = errors.New("invalid test type, must be confirmed, likely, negative, or self_report")
	ErrCodeAlreadyExpired  = errors.New("code already expired")
	ErrCodeAlreadyClaimed  = errors.New("code already claimed")
	ErrCodeTooShort        = errors.New("verification code is too short")
	ErrAlreadyReported     = errors.New("phone number not eligible for user report, try again later")
	ErrRequiresPhoneNumber = errors.New("phone number is required for user report requests")
)

var _ Auditable = (*VerificationCode)(nil)

// VerificationCode represents a verification code in the database.
type VerificationCode struct {
	gorm.Model
	Errorable

	RealmID       uint   // VerificationCodes belong to exactly one realm when issued.
	Code          string `gorm:"type:varchar(512)"`
	LongCode      string `gorm:"type:varchar(512)"`
	UUID          string `gorm:"type:uuid;default:null"`
	Claimed       bool   `gorm:"default:false"`
	TestType      string `gorm:"type:varchar(20)"`
	SymptomDate   *time.Time
	TestDate      *time.Time
	ExpiresAt     time.Time
	LongExpiresAt time.Time

	UserReportID *uint
	// These are used in building a user report
	Nonce         []byte `gorm:"-"`
	PhoneNumber   string `gorm:"-"`
	NonceRequired bool   `gorm:"-"`

	// IssuingUserID is the ID of the user in the database that created this
	// verification code. This is only populated if the code was created via the
	// UI.
	IssuingUserID uint `gorm:"column:issuing_user_id; type:integer;"`

	// IssuingAppID is the ID of the app in the database that created this
	// verification code. This is only populated if the code was created via the
	// API.
	IssuingAppID uint `gorm:"column:issuing_app_id; type:integer;"`

	// IssuingExternalID is an optional ID to an external system that created this
	// verification code. This is only populated if the code was created via the
	// API AND the API caller supplied it in the request. This ID has no meaning
	// in this system. It can be up to 255 characters in length.
	IssuingExternalID string `gorm:"column:issuing_external_id; type:varchar(255);"`
}

// BeforeSave is used by callbacks.
func (v *VerificationCode) BeforeSave(tx *gorm.DB) error {
	if len(v.IssuingExternalID) > 255 {
		v.AddError("issuingExternalID", "cannot exceed 255 characters")
	}

	if msgs := v.ErrorMessages(); len(msgs) > 0 {
		return fmt.Errorf("validation failed: %s", strings.Join(msgs, ", "))
	}
	return nil
}

// FormatSymptomDate returns YYYY-MM-DD formatted test date, or "" if nil.
func (v *VerificationCode) FormatSymptomDate() string {
	if v.SymptomDate == nil {
		return ""
	}
	return v.SymptomDate.Format(project.RFC3339Date)
}

// IsCodeExpired checks to see if the actual code provided is the short or long
// code, and determines if it is expired based on that.
func (db *Database) IsCodeExpired(v *VerificationCode, code string) (bool, CodeType, error) {
	if v == nil {
		return false, 0, fmt.Errorf("provided code is nil")
	}

	// It's possible that this could be called with the already HMACd version.
	possibles, err := db.generateVerificationCodeHMACs(code)
	if err != nil {
		return false, 0, fmt.Errorf("failed to create hmac: %w", err)
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
		return !v.ExpiresAt.After(now), CodeTypeShort, nil
	case inList(v.LongCode, possibles):
		return !v.LongExpiresAt.After(now), CodeTypeLong, nil
	default:
		// This should be unreachable code because the caller looks up the
		// verification code.
		return true, 0, fmt.Errorf("not found")
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
func (v *VerificationCode) Validate(realm *Realm) error {
	now := time.Now()
	v.Code = project.TrimSpace(v.Code)
	if len(v.Code) < MinCodeLength {
		return ErrCodeTooShort
	}
	v.LongCode = project.TrimSpace(v.LongCode)
	if len(v.LongCode) < MinCodeLength {
		return ErrCodeTooShort
	}

	if _, ok := ValidTestTypes[v.TestType]; !ok {
		return ErrInvalidTestType
	}
	if !realm.ValidTestType(v.TestType) {
		return ErrUnsupportedTestType
	}

	if now.After(v.ExpiresAt) || now.After(v.LongExpiresAt) {
		return ErrCodeAlreadyExpired
	}
	return nil
}

// FindVerificationCode find a verification code by the code number (can be
// short code or long code).
func (r *Realm) FindVerificationCode(db *Database, code string) (*VerificationCode, error) {
	hmacedCodes, err := db.generateVerificationCodeHMACs(code)
	if err != nil {
		return nil, fmt.Errorf("failed to create hmac: %w", err)
	}

	var vc VerificationCode
	if err := db.db.
		Where("realm_id = ? AND (code IN (?) OR long_code IN (?))", r.ID, hmacedCodes, hmacedCodes).
		First(&vc).
		Error; err != nil {
		return nil, err
	}
	return &vc, nil
}

// ListRecentCodes shows the last 5 recently issued codes for a given issuing
// user. The code and longCode are removed, this is only intended to show
// metadata.
func (r *Realm) ListRecentCodes(db *Database, user *User) ([]*VerificationCode, error) {
	var codes []*VerificationCode
	if err := db.db.
		Model(&VerificationCode{}).
		Where("realm_id = ? AND issuing_user_id = ?", r.ID, user.ID).
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
func (r *Realm) ExpireCode(db *Database, uuid string, actor Auditable) (*VerificationCode, error) {
	if actor == nil {
		return nil, fmt.Errorf("auditing actor is nil")
	}

	var vc VerificationCode
	if err := db.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Set("gorm:query_option", "FOR UPDATE").
			Where("realm_id = ? AND uuid = ?", r.ID, uuid).
			Find(&vc).Error; err != nil {
			return fmt.Errorf("failed to get existing verification code: %w", err)
		}

		// We don't want to update the expiration time on an already-expired code,
		// but this also isn't an error, so just return now.
		if vc.IsExpired() {
			return nil
		}

		// It's not possible to expire an already-claimed code.
		if vc.Claimed {
			return ErrCodeAlreadyClaimed
		}

		oldExpires := vc.ExpiresAt
		vc.ExpiresAt = time.Now().UTC()
		vc.LongExpiresAt = vc.ExpiresAt
		if err := tx.Save(&vc).Error; err != nil {
			return err
		}

		audit := BuildAuditEntry(actor, "expired verification code", &vc, r.ID)
		audit.Diff = stringDiff(oldExpires.Format(time.RFC3339), vc.ExpiresAt.Format(time.RFC3339))
		if err := tx.Save(audit).Error; err != nil {
			return fmt.Errorf("failed to save audits: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return &vc, nil
}

// SaveVerificationCode created or updates a verification code in the database.
// Max age represents the maximum age of the test date [optional] in the record.
func (r *Realm) SaveVerificationCode(db *Database, vc *VerificationCode) error {
	if err := vc.Validate(r); err != nil {
		return err
	}
	return db.db.Transaction(func(tx *gorm.DB) error {
		// If the report type is self-report, this verification code requests that
		// the phone number not exist in the de-duplicate table.
		var userReport *UserReport
		var err error
		if vc.TestType == verifyapi.ReportTypeSelfReport {
			if len(vc.PhoneNumber) == 0 {
				return ErrRequiresPhoneNumber
			}
			userReport, err = db.FindUserReport(tx, vc.PhoneNumber)
			if err != nil && !IsNotFound(err) {
				return fmt.Errorf("findUserReport: %w", err)
			}
			if userReport != nil {
				return ErrAlreadyReported
			}

			userReport, err = db.NewUserReport(vc.PhoneNumber, vc.Nonce, vc.NonceRequired)
			if err != nil {
				return fmt.Errorf("newUserReport: %w", err)
			}
			if err := tx.Create(userReport).Error; err != nil {
				return ErrAlreadyReported
			}
		}
		if userReport != nil {
			vc.UserReportID = &userReport.ID
			vc.LongExpiresAt = vc.ExpiresAt // Self report expiration codes are all short.
		}

		if vc.Model.ID == 0 {
			return tx.Create(vc).Error
		}
		return tx.Save(vc).Error
	})
}

// DeleteVerificationCode deletes the code by ID, this is a hard delete.
func (r *Realm) DeleteVerificationCode(db *Database, id uint) error {
	return db.db.Unscoped().
		Where("realm_id = ? AND id = ?", r.ID, id).
		Delete(&VerificationCode{}).
		Error
}

// AuditID is the id to use in audit logs.
func (v *VerificationCode) AuditID() string {
	return fmt.Sprintf("verification_code:%d", v.ID)
}

// AuditDisplay is the display name in audit logs.
func (v *VerificationCode) AuditDisplay() string {
	return v.UUID
}

// UpdateStats increments VerificationCode statistics incrementing stats but the number issued.
func (db *Database) UpdateStats(ctx context.Context, codes ...*VerificationCode) {
	issued := len(codes)
	if issued == 0 {
		return
	}
	logger := logging.FromContext(ctx).Named("issueapi.recordStats")
	v := codes[0]
	date := timeutils.UTCMidnight(v.CreatedAt)

	// If the issuer was a user, update the user stats for the day.
	if v.IssuingUserID != 0 {
		sql := `
			INSERT INTO user_stats (date, realm_id, user_id, codes_issued)
				VALUES ($1, $2, $3, $4)
			ON CONFLICT (date, realm_id, user_id) DO UPDATE
				SET codes_issued = user_stats.codes_issued + $4`

		if err := db.db.Exec(sql, date, v.RealmID, v.IssuingUserID, issued).Error; err != nil {
			logger.Warnw("failed to update user stats", "error", err)
		}
	}

	// If the request was an API request, we might have an external issuer ID.
	if len(v.IssuingExternalID) != 0 {
		sql := `
			INSERT INTO external_issuer_stats (date, realm_id, issuer_id, codes_issued)
				VALUES ($1, $2, $3, $4)
			ON CONFLICT (date, realm_id, issuer_id) DO UPDATE
				SET codes_issued = external_issuer_stats.codes_issued + $4
		`

		if err := db.db.Exec(sql, date, v.RealmID, v.IssuingExternalID, issued).Error; err != nil {
			logger.Warnw("failed to update external-issuer stats", "error", err)
		}
	}

	// If the issuer was a app, update the app stats for the day.
	if v.IssuingAppID != 0 {
		sql := `
			INSERT INTO authorized_app_stats (date, authorized_app_id, codes_issued)
				VALUES ($1, $2, $3)
			ON CONFLICT (date, authorized_app_id) DO UPDATE
				SET codes_issued = authorized_app_stats.codes_issued + $3
		`

		if err := db.db.Exec(sql, date, v.IssuingAppID, issued).Error; err != nil {
			logger.Warnw("failed to update authorized app stats", "error", err)
		}
	}

	// Update the per-realm stats.
	if v.RealmID != 0 {
		// Count the number of user initiated reports
		userReports := 0
		for _, vc := range codes {
			if vc.TestType == verifyapi.ReportTypeSelfReport {
				userReports++
			}
		}

		sql := `
			INSERT INTO realm_stats(date, realm_id, codes_issued, user_reports_issued)
				VALUES ($1, $2, $3, $4)
			ON CONFLICT (date, realm_id) DO UPDATE
				SET codes_issued = realm_stats.codes_issued + $3,
				user_reports_issued = realm_stats.user_reports_issued + $4`

		if err := db.db.Exec(sql, date, v.RealmID, issued, userReports).Error; err != nil {
			logger.Warnw("failed to update realm stats", "error", err)
		}
	}
}

// RecycleVerificationCodes sets to null code and long_code values
// so that status can be retained longer, but the codes are recycled into the pool.
func (db *Database) RecycleVerificationCodes(maxAge time.Duration) (int64, error) {
	if maxAge > 0 {
		maxAge = -1 * maxAge
	}
	deleteBefore := time.Now().UTC().Add(maxAge)
	// Null out the codes where this can be done.
	rtn := db.db.Model(&VerificationCode{}).
		Select("code", "long_code").
		Where("expires_at < ? AND long_expires_at < ? AND (code != ? OR long_code != ?)", deleteBefore, deleteBefore, "", "").
		Update(map[string]interface{}{"code": "", "long_code": ""})
	return rtn.RowsAffected, rtn.Error
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
	keys, err := db.GetVerificationCodeDatabaseHMAC()
	if err != nil {
		return "", fmt.Errorf("failed to get keys to generate verification code database HMAC: %w", err)
	}

	return initialHMAC(keys, verCode)
}

// generateVerificationCodeHMACs is a helper for generating all possible HMACs of a
// token.
func (db *Database) generateVerificationCodeHMACs(v string) ([]string, error) {
	keys, err := db.GetVerificationCodeDatabaseHMAC()
	if err != nil {
		return nil, fmt.Errorf("failed to get keys to generate verification code database HMACs: %w", err)
	}

	return allAllowedHMACs(keys, v)
}
