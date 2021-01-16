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
	"strings"

	"github.com/google/exposure-notifications-server/pkg/keys"
)

// TokenSigningConfig represents the settings for system-wide certificate
// signing. These should be used if you are managing certificate keys externally.
type TokenSigningConfig struct {
	// Keys determines the key manager configuration for this token signing
	// configuration.
	Keys keys.Config `env:", prefix=TOKEN_"`

	// TokenSigningKeys is the parent token signing key (not the actual signing
	// version). It is an array for backwards-compatibility, but in practice it
	// should only have one element.
	//
	// Previously it was a list of all possible signing key versions, but those
	// have moved into the database.
	//
	// TODO(sethvargo): Convert to string in 0.22+.
	TokenSigningKeys []string `env:"TOKEN_SIGNING_KEY, required"`

	// TokenSigningKeyIDs specifies the list of kids, corresponding to the
	// TokenSigningKey
	//
	// TODO(sethvargo): Remove in 0.22+.
	//
	// Deprecated: moved into the database.
	TokenSigningKeyIDs []string `env:"TOKEN_SIGNING_KEY_ID, default=v1"`

	// TokenIssuer is the `iss` field on the JWT.
	TokenIssuer string `env:"TOKEN_ISSUER, default=diagnosis-verification-example"`
}

// ParentKeyName returns the name of the parent key.
func (t *TokenSigningConfig) ParentKeyName() string {
	// Validation prevents this slice from being empty.
	value := t.TokenSigningKeys[0]

	// This is Google-specific, but that's the only platform where we can
	// meaningfully detect this.
	if idx := strings.Index(t.TokenSigningKeys[0], "/cryptoKeyVersions"); idx != -1 {
		value = value[0:idx]
	}

	return value
}

// FindKeyByKid attempts to find the matching signing key for the given kid. The
// boolean indicates whether the search was successful.
//
// TODO(sethvargo): remove in 0.22+.
func (t *TokenSigningConfig) FindKeyByKid(kid string) (string, bool) {
	idx := -1
	for i, v := range t.TokenSigningKeyIDs {
		if v == kid {
			idx = i
			break
		}
	}

	if idx == -1 {
		return "", false
	}

	// This is safe to index because the validation check ensures the lengths are
	// equal.
	return t.TokenSigningKeys[idx], true
}

// Validate validates the configuration.
func (t *TokenSigningConfig) Validate() error {
	if len(t.TokenSigningKeys) == 0 {
		return fmt.Errorf("TOKEN_SIGNING_KEY must have at least one element")
	}

	if len(t.TokenSigningKeyIDs) == 0 {
		return fmt.Errorf("TOKEN_SIGNING_KEY_ID must have at least one entry")
	}

	if len(t.TokenSigningKeys) != len(t.TokenSigningKeyIDs) {
		return fmt.Errorf("TOKEN_SIGNING_KEY and TOKEN_SIGNING_KEY_ID must be lists of the same length")
	}

	return nil
}
