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
	"github.com/google/exposure-notifications-verification-server/internal/project"
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
		parts[1] = s.SymptomDate.Format(project.RFC3339Date)
	}
	if s.TestDate != nil {
		parts[2] = s.TestDate.Format(project.RFC3339Date)
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
		parsedDate, err := time.Parse(project.RFC3339Date, parts[1])
		if err != nil {
			return nil, fmt.Errorf("subject contains invalid symptom date: %w", err)
		}
		symptomDate = &parsedDate
	}

	var testDate *time.Time
	if len(parts) == 3 && parts[2] != "" {
		parsedDate, err := time.Parse(project.RFC3339Date, parts[2])
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
	return t.SymptomDate.Format(project.RFC3339Date)
}

// FormatTestDate returns YYYY-MM-DD formatted test date, or "" if nil.
func (t *Token) FormatTestDate() string {
	if t.TestDate == nil {
		return ""
	}
	return t.TestDate.Format(project.RFC3339Date)
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
func (db *Database) ClaimToken(t time.Time, authApp *AuthorizedApp, tokenID string, subject *Subject) error {
	t = t.UTC()

	if err := db.db.Transaction(func(tx *gorm.DB) error {
		var tok Token
		if err := tx.
			Set("gorm:query_option", "FOR UPDATE").
			Where("token_id = ?", tokenID).
			Where("realm_id = ?", authApp.RealmID).
			First(&tok).
			Error; err != nil {
			if IsNotFound(err) {
				return ErrTokenExpired
			}
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

		// The subject is made up of testtype.symptomDate.
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

		// Save token.
		tok.Used = true
		if err := tx.Save(&tok).Error; err != nil {
			return err
		}

		return nil
	}); err != nil {
		if !errors.Is(err, ErrTokenUsed) {
			go db.updateStatsTokenInvalid(t, authApp)
		}
		return err
	}

	go db.updateStatsTokenClaimed(t, authApp)
	return nil
}

// IssueTokenRequest is used to request the validation of a verification code
// in order to issue a token
type IssueTokenRequest struct {
	Time        time.Time
	AuthApp     *AuthorizedApp
	VerCode     string
	Nonce       []byte
	AcceptTypes api.AcceptTypes
	ExpireAfter time.Duration
}

// VerifyCodeAndIssueToken takes a previously issued verification code and exchanges
// it for a long term token. The verification code must not have expired and must
// not have been previously used. Both acctions are done in a single database
// transaction.
// The verCode can be the "short code" or the "long code" which impacts expiry time.
//
// The long term token can be used later to sign keys when they are submitted.
func (db *Database) VerifyCodeAndIssueToken(request *IssueTokenRequest) (*Token, error) {
	t := request.Time.UTC()

	hmacedCodes, err := db.generateVerificationCodeHMACs(request.VerCode)
	if err != nil {
		return nil, fmt.Errorf("failed to create hmac: %w", err)
	}

	var tok *Token
	var vc VerificationCode
	if err := db.db.Transaction(func(tx *gorm.DB) error {
		// Load the verification code - do quick expiry and claim checks.
		// Also lock the row for update.
		if err := tx.
			Set("gorm:query_option", "FOR UPDATE").
			Where("realm_id = ?", request.AuthApp.RealmID).
			Where("(code IN (?) OR long_code IN (?))", hmacedCodes, hmacedCodes).
			First(&vc).
			Error; err != nil {
			if IsNotFound(err) {
				return ErrVerificationCodeNotFound
			}
			return err
		}

		// Validation
		expired, codeType, err := db.IsCodeExpired(&vc, request.VerCode)
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

		if _, ok := request.AcceptTypes[vc.TestType]; !ok {
			db.logger.Debugw("checked not of accepted testType", "ID", vc.ID)
			return ErrUnsupportedTestType
		}

		// Check associated UserReport record if necessary.
		if vc.UserReportID != nil {
			var userReport UserReport
			if err := tx.
				Set("gorm:query_option", "FOR UPDATE").
				Where("id = ?", *vc.UserReportID).
				First(&userReport).
				Error; err != nil {
				if IsNotFound(err) {
					return ErrVerificationCodeNotFound
				}
				return err
			}

			providedNonce := base64.StdEncoding.EncodeToString(request.Nonce)
			if userReport.Nonce != providedNonce {
				// If the code was found, but the nonce is required and doesn't match,
				// treat this the same as not found.
				return ErrVerificationCodeNotFound
			}

			// Mark the user report claimed so that it is not garbage collected.
			userReport.CodeClaimed = true
			if err := tx.Save(&userReport).Error; err != nil {
				return fmt.Errorf("failed to validate user report: %w", err)
			}
		}

		// Mark as claimed
		vc.Claimed = true
		if err := tx.Save(&vc).Error; err != nil {
			return fmt.Errorf("failed to claim verification code: %w", err)
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
			ExpiresAt:   time.Now().UTC().Add(request.ExpireAfter),
			RealmID:     request.AuthApp.RealmID,
		}

		return tx.Create(tok).Error
	}); err != nil {
		if !errors.Is(err, ErrVerificationCodeUsed) {
			go db.updateStatsCodeInvalid(t, request.AuthApp)
		}
		return nil, err
	}

	go db.updateStatsCodeClaimed(t, request.AuthApp)
	go db.updateStatsAgeDistrib(t, request.AuthApp, &vc)
	return tok, nil
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

// updateStatsCodeInvalid updates the statistics, increasing the number of codes
// that were invalid.
func (db *Database) updateStatsCodeInvalid(t time.Time, authApp *AuthorizedApp) {
	t = timeutils.UTCMidnight(t)

	realmSQL := `
			INSERT INTO realm_stats(date, realm_id, codes_invalid)
				VALUES ($1, $2, 1)
			ON CONFLICT (date, realm_id) DO UPDATE
				SET codes_invalid = realm_stats.codes_invalid + 1
		`
	if err := db.db.Exec(realmSQL, t, authApp.RealmID).Error; err != nil {
		db.logger.Errorw("failed to update realm stats code invalid", "error", err)
	}

	authAppSQL := `
			INSERT INTO authorized_app_stats(date, authorized_app_id, codes_invalid)
				VALUES ($1, $2, 1)
			ON CONFLICT (date, authorized_app_id) DO UPDATE
				SET codes_invalid = authorized_app_stats.codes_invalid + 1
		`
	if err := db.db.Exec(authAppSQL, t, authApp.ID).Error; err != nil {
		db.logger.Errorw("failed to update authorized app stats code invalid", "error", err)
	}
}

// updateStatsAgeDistrib updates the statistics, increasing the number of codes
// claimed and the distribution of issue-claim time.
func (db *Database) updateStatsAgeDistrib(t time.Time, authApp *AuthorizedApp, vc *VerificationCode) {
	midnight := timeutils.UTCMidnight(t)

	if err := db.db.Transaction(func(tx *gorm.DB) error {
		var existing RealmStat
		isNewRecord := false
		if err := tx.
			Set("gorm:query_option", "FOR UPDATE").
			Table("realm_stats").
			Where("realm_id = ?", authApp.RealmID).
			Where("date = ?", midnight).
			Take(&existing).
			Error; err != nil {
			if IsNotFound(err) {
				isNewRecord = true
			} else {
				return err
			}
		}

		if len(existing.CodeClaimAgeDistribution) == 0 {
			existing.CodeClaimAgeDistribution = make([]int32, len(claimDistributionBuckets))
		}

		// Update claim age distribution
		sinceIssue := t.Sub(vc.CreatedAt)
		for i, bucket := range claimDistributionBuckets {
			if sinceIssue <= bucket {
				existing.CodeClaimAgeDistribution[i]++
				break
			}
		}

		// Update claim age mean
		avg := float64(int64(existing.CodeClaimMeanAge.Duration)*int64(existing.CodesClaimed)+int64(sinceIssue)) / float64(existing.CodesClaimed+1)
		existing.CodeClaimMeanAge = FromDuration(time.Duration(avg))

		// Update total codes claimed
		existing.CodesClaimed++

		sel := tx.Table("realm_stats").
			Where("realm_id = ?", authApp.RealmID).
			Where("date = ?", midnight)
		if isNewRecord {
			existing.RealmID = authApp.RealmID
			existing.Date = midnight
			if err := sel.Create(&existing).Error; err != nil {
				return err
			}
		} else {
			if err := sel.Update(&existing).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		db.logger.Errorw("failed to save realm stats code claimed", "error", err)
	}
}

// updateStatsCodeClaimed updates the statistics, increasing the number of codes
// claimed.
func (db *Database) updateStatsCodeClaimed(t time.Time, authApp *AuthorizedApp) {
	midnight := timeutils.UTCMidnight(t)
	authAppSQL := `
			INSERT INTO authorized_app_stats(date, authorized_app_id, codes_claimed)
				VALUES ($1, $2, 1)
			ON CONFLICT (date, authorized_app_id) DO UPDATE
				SET codes_claimed = authorized_app_stats.codes_claimed + 1
		`
	if err := db.db.Exec(authAppSQL, midnight, authApp.ID).Error; err != nil {
		db.logger.Errorw("failed to update authorized app stats code claimed", "error", err)
	}
}

// updateStatsTokenInvalid updates the statistics, increasing the number of
// tokens that were invalid.
func (db *Database) updateStatsTokenInvalid(t time.Time, authApp *AuthorizedApp) {
	t = timeutils.UTCMidnight(t)

	realmSQL := `
			INSERT INTO realm_stats(date, realm_id, tokens_invalid)
				VALUES ($1, $2, 1)
			ON CONFLICT (date, realm_id) DO UPDATE
				SET tokens_invalid = realm_stats.tokens_invalid + 1
		`
	if err := db.db.Exec(realmSQL, t, authApp.RealmID).Error; err != nil {
		db.logger.Errorw("failed to update realm stats token invalid", "error", err)
	}

	authAppSQL := `
			INSERT INTO authorized_app_stats(date, authorized_app_id, tokens_invalid)
				VALUES ($1, $2, 1)
			ON CONFLICT (date, authorized_app_id) DO UPDATE
				SET tokens_invalid = authorized_app_stats.tokens_invalid + 1
		`
	if err := db.db.Exec(authAppSQL, t, authApp.ID).Error; err != nil {
		db.logger.Errorw("failed to update authorized app stats token invalid", "error", err)
	}
}

// updateStatsTokenClaimed updates the statistics, increasing the number of
// tokens claimed.
func (db *Database) updateStatsTokenClaimed(t time.Time, authApp *AuthorizedApp) {
	t = timeutils.UTCMidnight(t)

	realmSQL := `
			INSERT INTO realm_stats(date, realm_id, tokens_claimed)
					VALUES ($1, $2, 1)
				ON CONFLICT (date, realm_id) DO UPDATE
					SET tokens_claimed = realm_stats.tokens_claimed + 1
			`
	if err := db.db.Exec(realmSQL, t, authApp.RealmID).Error; err != nil {
		db.logger.Errorw("failed to update realm stats token claimed", "error", err)
	}

	authAppSQL := `
			INSERT INTO authorized_app_stats(date, authorized_app_id, tokens_claimed)
				VALUES ($1, $2, 1)
			ON CONFLICT (date, authorized_app_id) DO UPDATE
				SET tokens_claimed = authorized_app_stats.tokens_claimed + 1
		`
	if err := db.db.Exec(authAppSQL, t, authApp.ID).Error; err != nil {
		db.logger.Errorw("failed to update authorized app stats token claimed", "error", err)
	}
}
