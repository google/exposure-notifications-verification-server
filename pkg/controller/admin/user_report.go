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

package admin

import (
	"context"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/nyaruka/phonenumbers"
)

func (c *Controller) HandleUserReportIndex() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		c.renderUserReport(ctx, w)
	})
}

func (c *Controller) HandleUserReportPurge() http.Handler {
	type FormData struct {
		PhoneNumber string `form:"phone_number[full]"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}
		flash := controller.Flash(session)

		currentUser := controller.UserFromContext(ctx)
		if currentUser == nil {
			controller.MissingUser(w, r, c.h)
			return
		}

		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			flash.Error("Failed to process form: %v", err)
			w.WriteHeader(http.StatusUnprocessableEntity)
			c.renderUserReport(ctx, w)
			return
		}

		// In this form, we're using the input library to get the fully internationalized
		// phone number, so set the default region to unknown. If an invalid number, incomplete
		// number is entered in the selected region, the input won't validate in the JS and then
		// will send an empty string through to here which will fail.
		parsedPhone, err := project.CanonicalPhoneNumber(form.PhoneNumber, phonenumbers.UNKNOWN_REGION)
		if err != nil {
			flash.Error("Failed to decode phone number: %v", err)
			w.WriteHeader(http.StatusUnprocessableEntity)
			c.renderUserReport(ctx, w)
			return
		}
		form.PhoneNumber = parsedPhone
		if err := c.db.DeleteUserReport(form.PhoneNumber, currentUser); err != nil {
			if !database.IsNotFound(err) {
				flash.Error("Failed to purge phone number")
				c.renderUserReport(ctx, w)
				return
			}
		}

		flash.Alert("Successfully purged phone number from user report database.")
		c.renderUserReport(ctx, w)
	})
}

func (c *Controller) renderUserReport(ctx context.Context, w http.ResponseWriter) {
	m := controller.TemplateMapFromContext(ctx)
	m.Title("User report - System Admin")
	c.h.RenderHTML(w, "admin/user-report/index", m)
}
