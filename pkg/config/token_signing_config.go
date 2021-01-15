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

package config

import (
	"fmt"

	"github.com/google/exposure-notifications-server/pkg/keys"
)

// TokenSigningConfig represents the settings for system-wide certificate
// signing. These should be used if you are managing certificate keys externally.
type TokenSigningConfig struct {
	// Keys determines the key manager configuration for this token signing
	// configuration.
	Keys keys.Config `env:",prefix=TOKEN_"`

	TokenSigningKeys   []string `env:"TOKEN_SIGNING_KEY, required"`
	TokenSigningKeyIDs []string `env:"TOKEN_SIGNING_KEY_ID, default=v1"`
	TokenIssuer        string   `env:"TOKEN_ISSUER, default=diagnosis-verification-example"`
}

func (t *TokenSigningConfig) ActiveKey() string {
	// Validation prevents this slice from being empty.
	return t.TokenSigningKeys[0]
}

func (t *TokenSigningConfig) ActiveKeyID() string {
	// Validation prevents this slice from being empty.
	return t.TokenSigningKeyIDs[0]
}

func (t *TokenSigningConfig) Validate() error {
	if len(t.TokenSigningKeys) == 0 {
		return fmt.Errorf("TOKEN_SIGNING_KEY must have at least one entry")
	}
	if len(t.TokenSigningKeyIDs) == 0 {
		return fmt.Errorf("TOKEN_SIGNING_KEY_ID must have at least one entry")
	}

	if len(t.TokenSigningKeys) != len(t.TokenSigningKeyIDs) {
		return fmt.Errorf("TOKEN_SIGNING_KEY and TOKEN_SIGNING_KEY_ID must be lists of the same length")
	}
	return nil
}
