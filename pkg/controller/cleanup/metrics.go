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

package cleanup

import (
	enobservability "github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

const metricPrefix = observability.MetricRoot + "/cleanup"

var (
	mClaimAttempts                  = stats.Int64(metricPrefix+"/claim_attempts", "The number of attempts to claim the cleanup", stats.UnitDimensionless)
	mClaimErrors                    = stats.Int64(metricPrefix+"/claim_errors", "The number of errors to claim the cleanup", stats.UnitDimensionless)
	mPurgeVerificationCodesAttempts = stats.Int64(metricPrefix+"/purge_verification_codes_attempts", "The number of attempts to purge verification codes", stats.UnitDimensionless)
	mPurgeVerificationCodesErrors   = stats.Int64(metricPrefix+"/purge_verification_codes_errors", "The number of attempts to purge verification codes", stats.UnitDimensionless)
	mPurgeVerificationCodesPurged   = stats.Int64(metricPrefix+"/purge_verification_codes_purged", "The number of verification codes that were purged", stats.UnitDimensionless)

	// Verification tokens
	mPurgeVerificationTokensAttempts = stats.Int64(metricPrefix+"/purge_verification_tokens_attempts", "The number of attempts to purge verification tokens", stats.UnitDimensionless)
	mPurgeVerificationTokensErrors   = stats.Int64(metricPrefix+"/purge_verification_tokens_errors", "The number of attempts to purge verification tokens", stats.UnitDimensionless)
	mPurgeVerificationTokensPurged   = stats.Int64(metricPrefix+"/purge_verification_tokens_purged", "The number of verification tokens that were purged", stats.UnitDimensionless)

	// Mobile apps
	mPurgeMobileAppsAttempts = stats.Int64(metricPrefix+"/purge_mobile_apps_attempts", "The number of attempts to purge mobile apps", stats.UnitDimensionless)
	mPurgeMobileAppsErrors   = stats.Int64(metricPrefix+"/purge_mobile_apps_errors", "The number of attempts to purge mobile apps", stats.UnitDimensionless)
	mPurgeMobileAppsPurged   = stats.Int64(metricPrefix+"/purge_mobile_apps_purged", "The number of mobile apps that were purged", stats.UnitDimensionless)

	// Audit entries
	mPurgeAuditEntriesAttempts = stats.Int64(metricPrefix+"/purge_audit_entries_attempts", "The number of attempts to purge audit entries", stats.UnitDimensionless)
	mPurgeAuditEntriesErrors   = stats.Int64(metricPrefix+"/purge_audit_entries_errors", "The number of attempts to purge audit entries", stats.UnitDimensionless)
	mPurgeAuditEntriesPurged   = stats.Int64(metricPrefix+"/purge_audit_entries_purged", "The number of audit entries that were purged", stats.UnitDimensionless)
)

func init() {
	enobservability.CollectViews([]*view.View{
		{
			Name:        metricPrefix + "/claim_attempt_count",
			Measure:     mClaimAttempts,
			Description: "The count of the number of attempts to claim the cleanup",
			Aggregation: view.Count(),
		}, {
			Name:        metricPrefix + "/claim_error_count",
			Measure:     mClaimErrors,
			Description: "The count of the number of errors to claim the cleanup",
			Aggregation: view.Count(),
		}, {
			Name:        metricPrefix + "/purge_verification_codes_attempt_count",
			Measure:     mPurgeVerificationCodesAttempts,
			Description: "The count of the number of attempts to purge verification codes",
			Aggregation: view.Count(),
		}, {
			Name:        metricPrefix + "/purge_verification_codes_error_count",
			Measure:     mPurgeVerificationCodesErrors,
			Description: "The count of the number of errors to purge verification codes",
			Aggregation: view.Count(),
		}, {
			Name:        metricPrefix + "/purge_verification_codes_purged_count",
			Measure:     mPurgeVerificationCodesPurged,
			Description: "The count of the number of verification codes purged",
			Aggregation: view.Count(),
		}, {
			Name:        metricPrefix + "/purge_verification_tokens_attempt_count",
			Measure:     mPurgeVerificationTokensAttempts,
			Description: "The count of the number of attempts to purge verification tokens",
			Aggregation: view.Count(),
		}, {
			Name:        metricPrefix + "/purge_verification_tokens_error_count",
			Measure:     mPurgeVerificationTokensErrors,
			Description: "The count of the number of errors to purge verification tokens",
			Aggregation: view.Count(),
		}, {
			Name:        metricPrefix + "/purge_verification_tokens_purged_count",
			Measure:     mPurgeVerificationTokensPurged,
			Description: "The count of the number of verification tokens purged",
			Aggregation: view.Count(),
		}, {
			Name:        metricPrefix + "/purge_mobile_apps_attempt_count",
			Measure:     mPurgeMobileAppsAttempts,
			Description: "The count of the number of attempts to purge mobile apps",
			Aggregation: view.Count(),
		}, {
			Name:        metricPrefix + "/purge_mobile_apps_error_count",
			Measure:     mPurgeMobileAppsErrors,
			Description: "The count of the number of errors to purge mobile apps",
			Aggregation: view.Count(),
		}, {
			Name:        metricPrefix + "/purge_mobile_apps_purged_count",
			Measure:     mPurgeMobileAppsPurged,
			Description: "The count of the number of mobile apps purged",
			Aggregation: view.Count(),
		}, {
			Name:        metricPrefix + "/purge_audit_entries_attempt_count",
			Measure:     mPurgeAuditEntriesAttempts,
			Description: "The count of the number of attempts to purge audit entries",
			Aggregation: view.Count(),
		}, {
			Name:        metricPrefix + "/purge_audit_entries_error_count",
			Measure:     mPurgeAuditEntriesErrors,
			Description: "The count of the number of errors to purge audit entries",
			Aggregation: view.Count(),
		}, {
			Name:        metricPrefix + "/purge_audit_entries_purged_count",
			Measure:     mPurgeAuditEntriesPurged,
			Description: "The count of the number of audit entries purged",
			Aggregation: view.Count(),
		},
	}...)

}
