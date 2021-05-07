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
	// EnableUserReport allows for realms to enable user initiated diagnosis reporting,
	// via API + SMS.
	EnableUserReport bool `env:"ENABLE_USER_REPORT, default=false"`

	// EnableUserReportWeb will host a web page on the enx-redirect service
	// under a realm's domain IFF that realm has enabled user report and admin user report
	EnableUserReportWeb bool `env:"ENABLE_USER_REPORT_WEB, default=false"`

	// EnableAPIKeyLastUsedAt controls whether the UI will show the last_used_at
	// value for an API key.
	EnableAPIKeyLastUsedAt bool `env:"ENABLE_API_KEY_LAST_USED_AT, default=true"`
}

// AddToTemplate takes TemplateMap and writes the status of all known
// feature flags for use in HTML templates.
func (f *FeatureConfig) AddToTemplate(m controller.TemplateMap) controller.TemplateMap {
	m["features"] = f
	return m
}
