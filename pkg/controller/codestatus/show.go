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
	"net/http"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/issueapi"
)

func (c *Controller) HandleShow(issueCtrl *issueapi.Controller) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}
		flash := controller.Flash(session)

		uuids := r.URL.Query()["uuid"]
		var uuid string
		if len(uuids) == 1 {
			uuid = uuids[0]
		}
		m := controller.TemplateMapFromContext(ctx)
		m["UUID"] = uuid

		code, _, apiErr := issueCtrl.CheckCodeStatus(r, uuid)
		if apiErr != nil {
			flash.Error("Failed to process form: %v", apiErr.Error)
			c.h.RenderHTML(w, "codestatus/show", m)
			return
		}

		var status string
		if code.Claimed {
			status = "claimed by user"
		} else {
			status = "not yet claimed"
		}
		m["Status"] = status
		m["Expires"] = code.ExpiresAt.UTC().Format(time.RFC1123)
		c.h.RenderHTML(w, "codestatus/show", m)
	})
}
