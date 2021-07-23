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

	// TokenSigningKey is the parent token signing key (not the actual signing
	// version).
	TokenSigningKey string `env:"TOKEN_SIGNING_KEY, required"`

	// TokenIssuer is the `iss` field on the JWT.
	TokenIssuer string `env:"TOKEN_ISSUER, default=diagnosis-verification-example"`
}

// Validate validates the configuration.
func (t *TokenSigningConfig) Validate() error {
	if t.TokenSigningKey == "" {
		return fmt.Errorf("TOKEN_SIGNING_KEY is required")
	}
	if strings.Contains(t.TokenSigningKey, ",") {
		return fmt.Errorf("TOKEN_SIGNING_KEY can only contain one element")
	}
	if strings.Contains(t.TokenSigningKey, "/cryptoKeyVersions") {
		return fmt.Errorf("TOKEN_SIGNING_KEY must be the path to a parent crypto key (not the version)")
	}

	return nil
}
