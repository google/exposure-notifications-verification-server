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
	"context"
	"strings"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/ratelimit"

	"github.com/google/exposure-notifications-server/pkg/observability"

	firebase "firebase.google.com/go"
	"github.com/sethvargo/go-envconfig"
)

var _ IssueAPIConfig = (*ServerConfig)(nil)

// ServerConfig represents the environment based config for the server.
type ServerConfig struct {
	Firebase      FirebaseConfig
	Database      database.Config
	Observability observability.Config

	Port string `env:"PORT,default=8080"`

	// Login Config
	SessionDuration   time.Duration `env:"SESSION_DURATION,default=24h"`
	RevokeCheckPeriod time.Duration `env:"REVOKE_CHECK_DURATION,default=5m"`

	// CookieKeys is a slice of bytes. The odd values are hash keys to HMAC the
	// cookies. The even values are block keys to encrypt the cookie. Both keys
	// should be 64 bytes. The value's should be specified as base64 encoded.
	CookieKeys Base64ByteSlice `env:"COOKIE_KEYS,required"`

	// CookieDomain is the domain for which cookie should be valid.
	CookieDomain string `env:"COOKIE_DOMAIN"`

	// CSRFAuthKey is the authentication key. It must be 32-bytes and can be
	// generated with tools/gen-secret. The value's should be base64 encoded.
	CSRFAuthKey envconfig.Base64Bytes `env:"CSRF_AUTH_KEY,required"`

	// Application Config
	ServerName          string        `env:"SERVER_NAME,default=Diagnosis Verification Server"`
	CollisionRetryCount uint          `env:"COLLISION_RETRY_COUNT,default=6"`
	AllowedSymptomAge   time.Duration `env:"ALLOWED_PAST_SYMPTOM_DAYS,default=336h"` // 336h is 14 days.

	AssetsPath string `env:"ASSETS_PATH,default=./cmd/server/assets"`

	// If Dev mode is true, cookies aren't required to be sent over secure channels.
	// This includes CSRF protection base cookie. You want this false in production (the default).
	DevMode bool `env:"DEV_MODE"`

	// Rate limiting configuration
	RateLimit ratelimit.Config
}

// NewServerConfig initializes and validates a ServerConfig struct.
func NewServerConfig(ctx context.Context) (*ServerConfig, error) {
	var config ServerConfig
	if err := ProcessWith(ctx, &config, envconfig.OsLookuper()); err != nil {
		return nil, err
	}
	// For the, when inserting into the javascript, gets escaped and becomes unusable.
	config.Firebase.DatabaseURL = strings.ReplaceAll(config.Firebase.DatabaseURL, "https://", "")
	return &config, nil
}

func (c *ServerConfig) Validate() error {
	fields := []struct {
		Var  time.Duration
		Name string
	}{
		{c.SessionDuration, "SESSION_DURATION"},
		{c.RevokeCheckPeriod, "REVOKE_CHECK_DURATION"},
		{c.AllowedSymptomAge, "ALLOWED_PAST_SYMPTOM_DAYS"},
	}

	for _, f := range fields {
		if err := checkPositiveDuration(f.Var, f.Name); err != nil {
			return err
		}
	}

	return nil
}

func (c *ServerConfig) GetCollisionRetryCount() uint {
	return c.CollisionRetryCount
}

func (c *ServerConfig) GetAllowedSymptomAge() time.Duration {
	return c.AllowedSymptomAge
}

func (c *ServerConfig) ObservabilityExporterConfig() *observability.Config {
	return &c.Observability
}

// FirebaseConfig represents configuration specific to firebase auth.
type FirebaseConfig struct {
	APIKey          string `env:"FIREBASE_API_KEY,required"`
	AuthDomain      string `env:"FIREBASE_AUTH_DOMAIN,required"`
	DatabaseURL     string `env:"FIREBASE_DATABASE_URL,required"`
	ProjectID       string `env:"FIREBASE_PROJECT_ID,required"`
	StorageBucket   string `env:"FIREBASE_STORAGE_BUCKET,required"`
	MessageSenderID string `env:"FIREBASE_MESSAGE_SENDER_ID,required"`
	AppID           string `env:"FIREBASE_APP_ID,required"`
	MeasurementID   string `env:"FIREBASE_MEASUREMENT_ID,required"`

	TermsOfServiceURL string `env:"FIREBASE_TERMS_OF_SERVICE_URL,required"`
	PrivacyPolicyURL  string `env:"FIREBASE_PRIVACY_POLICY_URL,required"`
}

// FirebaseConfig returns the firebase SDK config based on the local env config.
func (c *ServerConfig) FirebaseConfig() *firebase.Config {
	return &firebase.Config{
		DatabaseURL:   c.Firebase.DatabaseURL,
		ProjectID:     c.Firebase.ProjectID,
		StorageBucket: c.Firebase.StorageBucket,
	}
}

// Base64ByteSlice is a slice of base64-encoded strings that we want to convert
// to bytes.
type Base64ByteSlice []envconfig.Base64Bytes

// AsBytes returns the value as a slice of bytes instead of its main type.
func (c Base64ByteSlice) AsBytes() [][]byte {
	s := make([][]byte, len(c))
	for i, v := range c {
		s[i] = []byte(v)
	}
	return s
}
