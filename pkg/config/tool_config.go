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

	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/gcpkms"
	"github.com/sethvargo/go-envconfig"
)

// ToolConfig represents the environment based config for command line tools.
type ToolConfig struct {
	Database database.Config
	GCPKMS   gcpkms.Config

	CertificateSigningKeyRing string `env:"CERTIFICATE_SIGNING_KEYRING,required"`
}

// NewToolConfig initializes and validates a ToolConfig struct.
func NewToolConfig(ctx context.Context) (*ToolConfig, error) {
	var config ToolConfig
	if err := ProcessWith(ctx, &config, envconfig.OsLookuper()); err != nil {
		return nil, err
	}
	return &config, nil
}
