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

// Package codestatus defines a web controller for the code status page of the verification
// server. This view allows users to view the status of previously-issued OTP codes.
package codestatus

import (
	"context"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
)

func (c *Controller) HandleShow() http.Handler {
	type FormData struct {
		UUID string `form:"uuid"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}
		flash := controller.Flash(session)

		realm := controller.RealmFromContext(ctx)
		if realm == nil {
			controller.MissingRealm(w, r, c.h)
			return
		}

		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			flash.Error("Failed to process form: %v", err)
			c.renderShow(ctx, w, "", "", "")
			return
		}

		code, _, apiErr := c.CheckCodeStatus(r, form.UUID)
		if apiErr != nil {
			flash.Error("Failed to process form: %v", apiErr.Error)
			c.renderShow(ctx, w, form.UUID, "", "")
			return
		}

		var status string
		if code.Claimed {
			status = "claimed by user"
		} else {
			status = "not yet claimed"
		}
		var exp string
		if code.IsExpired() {
			exp = "expired"
		} else {
			// TODO(whaught): This might be nicer as a formatted duration until now
			exp = code.ExpiresAt.UTC().Format(time.RFC1123)
		}
		c.renderShow(ctx, w, form.UUID, status, exp)
	})
}

func (c *Controller) renderShow(ctx context.Context, w http.ResponseWriter, uuid, status, expires string) {
	m := controller.TemplateMapFromContext(ctx)
	m["uuid"] = uuid
	m["status"] = status
	m["expires"] = expires
	c.h.RenderHTML(w, "code/show", m)
}
