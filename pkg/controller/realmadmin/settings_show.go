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

package realmadmin

import (
	"context"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

const defaultSMSTemplateLabel = "Default SMS template"

type TemplateData struct {
	Label string
	Value string
	Index int
}

func (c *Controller) renderSettings(
	ctx context.Context, w http.ResponseWriter, r *http.Request, realm *database.Realm,
	smsConfig *database.SMSConfig, emailConfig *database.EmailConfig, keyServerStats *database.KeyServerStats,
	quotaLimit, quotaRemaining uint64) {
	if smsConfig == nil {
		var err error
		smsConfig, err = realm.SMSConfig(c.db)
		if err != nil {
			if !database.IsNotFound(err) {
				controller.InternalError(w, r, c.h, err)
				return
			}
			smsConfig = new(database.SMSConfig)
		}
	}

	if emailConfig == nil {
		var err error
		emailConfig, err = realm.EmailConfig(c.db)
		if err != nil {
			if !database.IsNotFound(err) {
				controller.InternalError(w, r, c.h, err)
				return
			}
			emailConfig = &database.EmailConfig{SMTPPort: "587"}
		}
	}

	if keyServerStats == nil {
		keyServerStats = &database.KeyServerStats{}
	}

	// Look up the sms from numbers.
	smsFromNumbers, err := c.db.SMSFromNumbers()
	if err != nil {
		controller.InternalError(w, r, c.h, err)
		return
	}

	// Don't pass through the system config to the template - we don't want to
	// risk accidentally rendering its ID or values since the realm should never
	// see these values. However, we have to go lookup the actual SMS config
	// values if present so that if the user unchecks the form, they don't see
	// blank values if they were previously using their own SMS configs.
	if smsConfig.IsSystem {
		var tmpRealm database.Realm
		tmpRealm.ID = realm.ID
		tmpRealm.UseSystemSMSConfig = false

		var err error
		smsConfig, err = tmpRealm.SMSConfig(c.db)
		if err != nil {
			if !database.IsNotFound(err) {
				controller.InternalError(w, r, c.h, err)
				return
			}
			smsConfig = new(database.SMSConfig)
		}
	}

	if emailConfig.IsSystem {
		var tmpRealm database.Realm
		tmpRealm.ID = realm.ID
		tmpRealm.UseSystemEmailConfig = false

		var err error
		emailConfig, err = tmpRealm.EmailConfig(c.db)
		if err != nil {
			if !database.IsNotFound(err) {
				controller.InternalError(w, r, c.h, err)
				return
			}
			emailConfig = new(database.EmailConfig)
		}
	}

	templates := map[int]TemplateData{
		0: {
			Label: defaultSMSTemplateLabel,
			Value: realm.SMSTextTemplate,
		}}
	if realm.SMSTextAlternateTemplates != nil {
		i := 0
		for k, v := range realm.SMSTextAlternateTemplates {
			i++
			templates[i] = TemplateData{
				Label: k,
				Value: *v,
			}
		}
	}

	m := controller.TemplateMapFromContext(ctx)
	m.Title("Realm settings")
	m["realm"] = realm
	m["smsConfig"] = smsConfig
	m["smsFromNumbers"] = smsFromNumbers
	m["smsTemplates"] = templates
	m["emailConfig"] = emailConfig
	m["statsConfig"] = keyServerStats
	m["countries"] = database.Countries
	m["testTypes"] = map[string]database.TestType{
		"confirmed": database.TestTypeConfirmed,
		"likely":    database.TestTypeConfirmed | database.TestTypeLikely,
		"negative":  database.TestTypeConfirmed | database.TestTypeLikely | database.TestTypeNegative,
	}
	// Valid settings for pwd rotation.
	m["mfaGracePeriod"] = mfaGracePeriod
	m["passwordRotateDays"] = passwordRotationPeriodDays
	m["passwordWarnDays"] = passwordRotationWarningDays
	// Valid settings for code parameters.
	m["shortCodeLengths"] = shortCodeLengths
	m["shortCodeMinutes"] = shortCodeMinutes
	m["longCodeLengths"] = longCodeLengths
	m["longCodeHours"] = longCodeHours
	m["enxRedirectDomain"] = c.config.GetENXRedirectDomain()

	m["maxSMSTemplate"] = database.SMSTemplateMaxLength

	m["quotaLimit"] = quotaLimit
	m["quotaRemaining"] = quotaRemaining

	c.h.RenderHTML(w, "realmadmin/edit", m)
}
