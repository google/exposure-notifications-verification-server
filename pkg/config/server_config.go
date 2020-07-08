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
	"fmt"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/pkg/base64util"
	"github.com/google/exposure-notifications-verification-server/pkg/database"

	firebase "firebase.google.com/go"
	"github.com/sethvargo/go-envconfig/pkg/envconfig"
)

// ServerConfig represents the environment based config for the server.
type ServerConfig struct {
	Firebase FirebaseConfig
	Database database.Config

	Port int `env:"PORT,default=8080"`

	// Login Config
	SessionCookieDuration time.Duration `env:"SESSION_DURATION,default=24h"`
	RevokeCheckPeriod     time.Duration `env:"REVOKE_CHECK_DURATION,default=5m"`

	// CSRF Secret Key. Must be 32-bytes. Can be generated with tools/gen-secret
	// Use the syntax of secret:// to pull the secret from secret manager.
	// We assume the secret itself is base64 encoded. Use CSRFKey() to transform to bytes.
	CSRFAuthKey string `env:"CSRF_AUTH_KEY,required"`

	// Application Config
	ServerName          string        `env:"SERVER_NAME,default=Diagnosis Verification Server"`
	CodeDuration        time.Duration `env:"CODE_DURATION,default=1h"`
	CodeDigits          uint          `env:"CODE_DIGITS,default=8"`
	CollisionRetryCount uint          `env:"COLISSION_RETRY_COUNT,default=6"`
	AllowedSymptomAge   time.Duration `env:"ALLOWED_PAST_SYMPTOM_DAYS,default=336h"` // 336h is 14 days.
	RateLimit           uint64        `env:"RATE_LIMIT,default=60"`

	AssetsPath string `env:"ASSETS_PATH,default=./cmd/server/assets"`

	// If Dev mode is true, cookies aren't required to be sent over secure channels.
	// This includes CSRF protection base cookie. You want this false in production (the default).
	DevMode bool `env:"DEV_MODE"`
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

func (c *ServerConfig) CSRFKey() ([]byte, error) {
	key, err := base64util.DecodeString(c.CSRFAuthKey)
	if err != nil {
		return nil, fmt.Errorf("error decoding CSRF_AUTH_KEY: %v", err)
	}
	if l := len(key); l != 32 {
		return nil, fmt.Errorf("CSRF_AUTH_KEY is not 32 bytes, got: %v", l)
	}
	return key, nil
}

func (c *ServerConfig) Validate() error {
	fields := []struct {
		Var  time.Duration
		Name string
	}{
		{c.SessionCookieDuration, "SESSION_DUATION"},
		{c.RevokeCheckPeriod, "REVOKE_CHECK_DURATION"},
		{c.CodeDuration, "CODE_DURATION"},
		{c.AllowedSymptomAge, "ALLOWED_PAST_SYMPTOM_DAYS"},
	}

	for _, f := range fields {
		if err := checkPositiveDuration(f.Var, f.Name); err != nil {
			return err
		}
	}

	return nil
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
}

// FirebaseConfig returns the firebase SDK config based on the local env config.
func (c *ServerConfig) FirebaseConfig() *firebase.Config {
	return &firebase.Config{
		DatabaseURL:   c.Firebase.DatabaseURL,
		ProjectID:     c.Firebase.ProjectID,
		StorageBucket: c.Firebase.StorageBucket,
	}
}
