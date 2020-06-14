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

// Package config defines the environment baased configuration for this server.
package config

import (
	"context"
	"strings"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/database"

	"github.com/sethvargo/go-envconfig/pkg/envconfig"

	firebase "firebase.google.com/go"
)

// New returns the environment config for the server. Only needs to be called once
// per instance, but may be called multiple times.
func New(ctx context.Context) (*Config, error) {
	var config Config
	if err := envconfig.Process(ctx, &config); err != nil {
		return nil, err
	}

	// For the, when inserting into the javascript, gets escaped and becomes unusable.
	config.Firebase.DatabaseURL = strings.ReplaceAll(config.Firebase.DatabaseURL, "https://", "")

	return &config, nil
}

// Config represents the environment based config for the server.
type Config struct {
	Firebase FirebaseConfig
	Database database.Config

	// Login Config
	SessionCookieDuration time.Duration `env:"SESSION_DURATION,default=24h"`
	RevokeCheckPeriod     time.Duration `env:"REVOKE_CHECK_DURATION,default=5m"`

	// Application Config
	ServerName          string        `env:"SERVER_NAME,default=Diagnosis Verification Server"`
	CodeDuration        time.Duration `env:"CODE_DURATION,default=1h"`
	CodeDigits          int           `env:"CODE_DIGITS,default=8"`
	ColissionRetryCount int           `env:"COLISSION_RETRY_COUNT,default=6"`

	KoDataPath string `env:"KO_DATA_PATH,default=./cmd/server/kodata"`
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
func (c *Config) FirebaseConfig() *firebase.Config {
	return &firebase.Config{
		DatabaseURL:   c.Firebase.DatabaseURL,
		ProjectID:     c.Firebase.ProjectID,
		StorageBucket: c.Firebase.StorageBucket,
	}
}
