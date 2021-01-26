// Copyright 2021 Google LLC
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

import "github.com/google/exposure-notifications-verification-server/pkg/controller"

// FeatureConfig represents features that are introduced as off by default allowing
// for server operators to control their release.
type FeatureConfig struct {
	// EnableAuthenticatedSMS allows for realms to managed SMS specific signing keys,
	// and enable/disable this feature. There is no launch timeline for GA.
	// This should not be used without prior coordination with Apple/Google.
	EnableAuthenticatedSMS bool `env:"ENABLE_AUTHENTICATED_SMS, default=false"`
}

// AddToTemplate takes TemplateMap and writes the status of all known
// feature flags for use in HTML templates.
func (f *FeatureConfig) AddToTemplate(m controller.TemplateMap) controller.TemplateMap {
	m["features"] = f
	return m
}
