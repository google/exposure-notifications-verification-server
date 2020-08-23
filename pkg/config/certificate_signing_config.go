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

import "time"

// CertificateSigningConfig represents the settinsg for system wide certificate signing.
// these should be used if you are managing certifiate keys externally.
type CertificateSigningConfig struct {
	PublicKeyCacheDuration  time.Duration `env:"PUBLIC_KEY_CACHE_DURATION,default=15m"`
	SignerCacheDuration     time.Duration `env:"CERTIFICATE_SIGNER_CACHE_DURATION,default=1m"`
	CertificateSigningKey   string        `env:"CERTIFICATE_SIGNING_KEY,required"`
	CertificateSigningKeyID string        `env:"CERTIFICATE_SIGNING_KEY_ID,default=v1"`
	CertificateIssuer       string        `env:"CERTIFICATE_ISSUER,default=diagnosis-verification-example"`
	CertificateAudience     string        `env:"CERTIFICATE_AUDIENCE,default=exposure-notifications-server"`
	CertificateDuration     time.Duration `env:"CERTIFICATE_DURATION,default=15m"`
}
