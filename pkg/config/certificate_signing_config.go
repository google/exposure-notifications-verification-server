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
	"time"

	"github.com/google/exposure-notifications-server/pkg/keys"
)

// CertificateSigningConfig represents the settings for system-wide certificate
// signing. These should be used if you are managing certificate keys externally.
type CertificateSigningConfig struct {
	// Keys determines the key manager configuration for this certificate signing
	// configuration.
	Keys keys.Config `env:",prefix=CERTIFICATE_"`

	// AllowedClockSkew impacts the "not before" or NBF time for certificates
	AllowedClockSkew time.Duration `env:"ALLOWED_CLOCK_SKEW,default=5s"`

	PublicKeyCacheDuration  time.Duration `env:"PUBLIC_KEY_CACHE_DURATION, default=15m"`
	SignerCacheDuration     time.Duration `env:"CERTIFICATE_SIGNER_CACHE_DURATION, default=1m"`
	CertificateSigningKey   string        `env:"CERTIFICATE_SIGNING_KEY, required"`
	CertificateSigningKeyID string        `env:"CERTIFICATE_SIGNING_KEY_ID, default=v1"`
	CertificateIssuer       string        `env:"CERTIFICATE_ISSUER, default=diagnosis-verification-example"`
	CertificateAudience     string        `env:"CERTIFICATE_AUDIENCE, default=exposure-notifications-server"`
	CertificateDuration     time.Duration `env:"CERTIFICATE_DURATION, default=15m"`
}
