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
	"time"

	"github.com/jinzhu/gorm"
)

const (
	tokenBytes = 96
)

var (
	ErrVerificationCodeExpired = errors.New("verification code expired")
	ErrVerificationCodeUsed    = errors.New("verification code used")
	ErrTokenExpired            = errors.New("verification token expired")
	ErrTokenUsed               = errors.New("verification token used")
)

// Token represents an issued "long term" from a validated verification code.
type Token struct {
	gorm.Model
	TokenID   string `gorm:"type:varchar(200); unique_index"`
	TestType  string `gorm:"type:varchar(20)"`
	TestDate  *time.Time
	Used      bool `gorm:"default:false"`
	ExpiresAt time.Time
}

// FormatTestDate returns YYYY-MM-DD formatted test date, or "" if nil.
func (t *Token) FormatTestDate() string {
	if t.TestDate == nil {
		return ""
	}
	return t.TestDate.Format("2006-01-02")
}

func (db *Database) ClaimToken(tokenID string) error {
	return db.db.Transaction(func(tx *gorm.DB) error {
		var tok Token
		if err := db.db.Set("gorm:query_option", "FOR UPDATE").Where("token_id = ?", tokenID).First(&tok).Error; err != nil {
			return err
		}

		if !tok.ExpiresAt.After(time.Now().UTC()) {
			return ErrTokenExpired
		}

		if tok.Used {
			return ErrTokenUsed
		}

		tok.Used = true
		return db.db.Save(&tok).Error
	})
}

// VerifyCodeAndIssueToken takes a previously issed verification code and exchanges
// it for a long term token. The verification code must not have expired and must
// not have been previously used. Both acctions are done in a single database
// transaction.
//
// The long term token can be used later to sign keys when they are submitted.
func (db *Database) VerifyCodeAndIssueToken(verCode string, expireAfter time.Duration) (*Token, error) {
	buffer := make([]byte, tokenBytes)
	_, err := rand.Read(buffer)
	if err != nil {
		return nil, fmt.Errorf("rand.Read: %v", err)
	}
	tokenID := base64.RawStdEncoding.EncodeToString(buffer)

	var tok *Token
	err = db.db.Transaction(func(tx *gorm.DB) error {
		// Load the verification code - do quick expiry and claim checks.
		// Also lock the row for update.
		var vc VerificationCode
		if err := db.db.Set("gorm:query_option", "FOR UPDATE").Where("code = ?", verCode).First(&vc).Error; err != nil {
			return err
		}
		if vc.IsExpired() {
			return ErrVerificationCodeExpired
		}
		if vc.Claimed {
			return ErrVerificationCodeUsed
		}

		// Mark claimed. Transactional update.
		vc.Claimed = true
		res := db.db.Save(vc)
		if res.Error != nil {
			return res.Error
		}

		// Issue the token. Take the generated value and create a new long term token.
		tok = &Token{
			TokenID:   tokenID,
			TestType:  vc.TestType,
			TestDate:  vc.TestDate,
			Used:      false,
			ExpiresAt: time.Now().UTC().Add(expireAfter),
		}

		return db.db.Create(tok).Error
	})
	return tok, err
}

func (db *Database) FindTokenByID(tokenID string) (*Token, error) {
	var token Token
	if err := db.db.Where("token_id = ?", tokenID).First(&token).Error; err != nil {
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
