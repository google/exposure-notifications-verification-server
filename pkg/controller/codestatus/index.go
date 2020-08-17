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

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
)

func (c *Controller) HandleIndex() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		m := controller.TemplateMapFromContext(ctx)
		// TODO(whaught): load a list of recent codes to show

		c.h.RenderHTML(w, "code/status", m)
	})
}
