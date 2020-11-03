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
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/pkg/timeutils"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/jinzhu/gorm"
)

const (
	tokenBytes     = 96
	intervalLength = 10 * time.Minute
)

var (
	ErrVerificationCodeNotFound = errors.New("verification code not found")
	ErrVerificationCodeExpired  = errors.New("verification code expired")
	ErrVerificationCodeUsed     = errors.New("verification code used")
	ErrTokenExpired             = errors.New("verification token expired")
	ErrTokenUsed                = errors.New("verification token used")
	ErrTokenMetadataMismatch    = errors.New("verification token test metadata mismatch")
	ErrUnsupportedTestType      = errors.New("verification code has unsupported test type")
)

// Token represents an issued "long term" from a validated verification code.
type Token struct {
	gorm.Model
	// Tokens belong to one realm.
	RealmID     uint
	TokenID     string `gorm:"type:varchar(200); unique_index"`
	TestType    string `gorm:"type:varchar(20)"`
	SymptomDate *time.Time
	TestDate    *time.Time
	Used        bool `gorm:"default:false"`
	ExpiresAt   time.Time
}

// Subject represents the data that is used in the 'sub' field of the token JWT.
type Subject struct {
	TestType    string
	SymptomDate *time.Time
	TestDate    *time.Time
}

func (s *Subject) String() string {
	parts := make([]string, 3)

	parts[0] = s.TestType
	if s.SymptomDate != nil {
		parts[1] = s.SymptomDate.Format("2006-01-02")
	}
	if s.TestDate != nil {
		parts[2] = s.TestDate.Format("2006-01-02")
	}

	return strings.Join(parts, ".")
}

func (s *Subject) SymptomInterval() uint32 {
	if s.SymptomDate == nil {
		return 0
	}
	return uint32(s.SymptomDate.UTC().Truncate(oneDay).Unix() / int64(intervalLength.Seconds()))
}

func ParseSubject(sub string) (*Subject, error) {
	parts := strings.Split(sub, ".")
	if length := len(parts); length < 2 || length > 3 {
		return nil, fmt.Errorf("subject must contain 2 or 3 parts, got: %v", length)
	}
	var symptomDate *time.Time
	if parts[1] != "" {
		parsedDate, err := time.Parse("2006-01-02", parts[1])
		if err != nil {
			return nil, fmt.Errorf("subject contains invalid symptom date: %w", err)
		}
		symptomDate = &parsedDate
	}

	var testDate *time.Time
	if len(parts) == 3 && parts[2] != "" {
		parsedDate, err := time.Parse("2006-01-02", parts[2])
		if err != nil {
			return nil, fmt.Errorf("subject contains invalid test date: %w", err)
		}
		testDate = &parsedDate
	}

	return &Subject{
		TestType:    parts[0],
		SymptomDate: symptomDate,
		TestDate:    testDate,
	}, nil
}

// FormatSymptomDate returns YYYY-MM-DD formatted symptom date, or "" if nil.
func (t *Token) FormatSymptomDate() string {
	if t.SymptomDate == nil {
		return ""
	}
	return t.SymptomDate.Format("2006-01-02")
}

// FormatTestDate returns YYYY-MM-DD formatted test date, or "" if nil.
func (t *Token) FormatTestDate() string {
	if t.TestDate == nil {
		return ""
	}
	return t.TestDate.Format("2006-01-02")
}

func (t *Token) Subject() *Subject {
	return &Subject{
		TestType:    t.TestType,
		SymptomDate: t.SymptomDate,
		TestDate:    t.TestDate,
	}
}

// ClaimToken looks up the token by ID, verifies that it is not expired and that
// the specified subject matches the parameters that were configured when issued.
func (db *Database) ClaimToken(realmID uint, tokenID string, subject *Subject) error {
	return db.db.Transaction(func(tx *gorm.DB) error {
		var tok Token
		if err := tx.
			Set("gorm:query_option", "FOR UPDATE").
			Where("token_id = ?", tokenID).
			Where("realm_id = ?", realmID).
			First(&tok).
			Error; err != nil {
			return err
		}

		if !tok.ExpiresAt.After(time.Now().UTC()) {
			db.logger.Debugw("tried to claim expired token", "ID", tok.ID)
			return ErrTokenExpired
		}

		if tok.Used {
			db.logger.Debugw("tried to claim used token", "ID", tok.ID)
			return ErrTokenUsed
		}

		// The subject is made up of testtype.symptomDate
		if tok.TestType != subject.TestType {
			db.logger.Debugw("database testType changed after token issued", "ID", tok.ID)
			return ErrTokenMetadataMismatch
		}
		if (tok.SymptomDate == nil && subject.SymptomDate != nil) ||
			(tok.SymptomDate != nil && subject.SymptomDate == nil) ||
			(tok.SymptomDate != nil && !tok.SymptomDate.Equal(*subject.SymptomDate)) {
			db.logger.Debugw("database symptomDate changed after token issued", "ID", tok.ID)
			return ErrTokenMetadataMismatch
		}
		if (tok.TestDate == nil && subject.TestDate != nil) ||
			(tok.TestDate != nil && subject.TestDate == nil) ||
			(tok.TestDate != nil && !tok.TestDate.Equal(*subject.TestDate)) {
			db.logger.Debugw("database testDate changed after token issued", "ID", tok.ID)
			return ErrTokenMetadataMismatch
		}

		tok.Used = true
		return tx.Save(&tok).Error
	})
}

// VerifyCodeAndIssueToken takes a previously issued verification code and exchanges
// it for a long term token. The verification code must not have expired and must
// not have been previously used. Both acctions are done in a single database
// transaction.
// The verCode can be the "short code" or the "long code" which impacts expiry time.
//
// The long term token can be used later to sign keys when they are submitted.
func (db *Database) VerifyCodeAndIssueToken(realmID uint, verCode string, acceptTypes api.AcceptTypes, expireAfter time.Duration) (*Token, error) {
	hmacedCodes, err := db.generateVerificationCodeHMACs(verCode)
	if err != nil {
		return nil, fmt.Errorf("failed to create hmac: %w", err)
	}

	var tok *Token
	err = db.db.Transaction(func(tx *gorm.DB) error {
		// Load the verification code - do quick expiry and claim checks.
		// Also lock the row for update.
		var vc VerificationCode
		if err := tx.
			Set("gorm:query_option", "FOR UPDATE").
			Where("realm_id = ?", realmID).
			Where("(code IN (?) OR long_code IN (?))", hmacedCodes, hmacedCodes).
			First(&vc).
			Error; err != nil {
			if gorm.IsRecordNotFoundError(err) {
				return ErrVerificationCodeNotFound
			}
			return err
		}

		// Validation
		expired, codeType, err := db.IsCodeExpired(&vc, verCode)
		if err != nil {
			db.logger.Errorw("failed to check code expiration", "ID", vc.ID, "error", err)
			return ErrVerificationCodeExpired
		}
		if expired {
			db.logger.Debugw("checked expired code", "ID", vc.ID, "codeType", codeType)
			return ErrVerificationCodeExpired
		}
		if vc.Claimed {
			db.logger.Debugw("checked expired code already used", "ID", vc.ID, "codeType", codeType)
			return ErrVerificationCodeUsed
		}

		if _, ok := acceptTypes[vc.TestType]; !ok {
			db.logger.Debugw("checked not of accepted testType", "ID", vc.ID)
			return ErrUnsupportedTestType
		}

		// Mark as claimed
		vc.Claimed = true
		if err := tx.Save(&vc).Error; err != nil {
			return fmt.Errorf("failed to claim token: %w", err)
		}

		// Update statistics
		now := timeutils.Midnight(vc.CreatedAt)
		sql := `
			INSERT INTO realm_stats(date, realm_id, codes_claimed)
				VALUES ($1, $2, 1)
			ON CONFLICT (date, realm_id) DO UPDATE
				SET codes_claimed = realm_stats.codes_claimed + 1
		`
		if err := tx.Exec(sql, now, vc.RealmID).Error; err != nil {
			return fmt.Errorf("failed to update stats: %w", err)
		}

		buffer := make([]byte, tokenBytes)
		if _, err := rand.Read(buffer); err != nil {
			return fmt.Errorf("failed to create token: %w", err)
		}
		tokenID := base64.RawStdEncoding.EncodeToString(buffer)

		// Issue the token. Take the generated value and create a new long term token.
		tok = &Token{
			TokenID:     tokenID,
			TestType:    vc.TestType,
			SymptomDate: vc.SymptomDate,
			TestDate:    vc.TestDate,
			Used:        false,
			ExpiresAt:   time.Now().UTC().Add(expireAfter),
			RealmID:     realmID,
		}

		return tx.Create(tok).Error
	})

	return tok, err
}

func (db *Database) FindTokenByID(tokenID string) (*Token, error) {
	var token Token
	if err := db.db.
		Where("token_id = ?", tokenID).
		First(&token).
		Error; err != nil {
		return nil, err
	}
	return &token, nil
}

// PurgeTokens will delete tokens that have expired since at least the
// provided maxAge ago.
// This is a hard delete, not a soft delete.
func (db *Database) PurgeTokens(maxAge time.Duration) (int64, error) {
	if maxAge > 0 {
		maxAge = -1 * maxAge
	}
	deleteBefore := time.Now().UTC().Add(maxAge)
	// Delete codes that expired before the delete before time.
	rtn := db.db.Unscoped().Where("expires_at < ?", deleteBefore).Delete(&Token{})
	return rtn.RowsAffected, rtn.Error
}
