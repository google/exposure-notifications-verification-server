// Copyright 2020 the Exposure Notifications Verification Server authors
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

package admin

import (
	"context"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/email"
)

// HandleEmailUpdate creates or updates the Email config.
func (c *Controller) HandleEmailUpdate() http.Handler {
	type FormData struct {
		SMTPAccount  string `form:"smtp_account"`
		SMTPPassword string `form:"smtp_password"`
		SMTPHost     string `form:"smtp_host"`
		SMTPPort     string `form:"smtp_port"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}
		flash := controller.Flash(session)

		emailConfig, err := c.db.SystemEmailConfig()
		if err != nil {
			if !database.IsNotFound(err) {
				controller.InternalError(w, r, c.h, err)
				return
			}
			emailConfig = &database.EmailConfig{
				SMTPPort: "587",
				IsSystem: true,
			}
		}

		// Requested form, stop processing.
		if r.Method == http.MethodGet {
			c.renderShowEmail(ctx, w, emailConfig)
			return
		}

		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			emailConfig.AddError("", err.Error())
			w.WriteHeader(http.StatusUnprocessableEntity)
			c.renderShowEmail(ctx, w, emailConfig)
			return
		}

		// Update
		emailConfig.ProviderType = email.ProviderTypeSMTP
		emailConfig.SMTPAccount = form.SMTPAccount
		if form.SMTPPassword != project.PasswordSentinel {
			emailConfig.SMTPPassword = form.SMTPPassword
		}
		emailConfig.SMTPHost = form.SMTPHost
		emailConfig.SMTPPort = form.SMTPPort
		if err := c.db.SaveEmailConfig(emailConfig); err != nil {
			flash.Error("Failed to save system email config: %v", err)
			c.renderShowEmail(ctx, w, emailConfig)
			return
		}

		flash.Alert("Successfully updated system email config")
		http.Redirect(w, r, "/admin/email", http.StatusSeeOther)
	})
}

func (c *Controller) renderShowEmail(ctx context.Context, w http.ResponseWriter, emailConfig *database.EmailConfig) {
	m := controller.TemplateMapFromContext(ctx)
	m.Title("Email - System Admin")
	m["emailConfig"] = emailConfig
	c.h.RenderHTML(w, "admin/email/show", m)
}
