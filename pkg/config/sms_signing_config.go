// Copyright 2021 the Exposure Notifications Verification Server authors
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
	"github.com/google/exposure-notifications-server/pkg/keys"
)

// SMSSigningConfig represents the settings for SMS-signing.
type SMSSigningConfig struct {
	// Keys determines the key manager configuration for this SMS signing
	// configuration.
	Keys keys.Config `env:", prefix=SMS_"`

	// FailClosed indicates whether authenticated SMS signature errors should fail
	// open (continue on error) or fail closed (halt and return error). In both
	// cases, a metric is logged and can be tracked for monitoring.
	//
	// This configuration only applies if authenticated SMS is enabled at the
	// system level AND a realm has configured authenticated SMS.
	//
	// The default behavior is to continue on error.
	FailClosed bool `env:"SMS_FAIL_CLOSED, default=false"`
}
