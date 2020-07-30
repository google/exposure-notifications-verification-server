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

// Package csrf contains utilities for issuing AJAX csrf tokens and
// handling errors on validation.
package csrf

import (
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/flash"
)

// HandleError is an http.HandlerFunc that can be installed inthe gorilla csrf
// protect middleware. It will respond w/ a JSON object containing error: on API
// requests and a signout redirect to other requests.
func (c *Controller) HandleError() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if controller.IsJSONContentType(r) {
			c.h.RenderJSON(w, http.StatusOK, api.Error("Invalid state. Refresh this window."))
			return
		}

		flash := flash.FromContext(w, r)
		flash.Error("CSRF token validation error, you have been signed out.")
		http.Redirect(w, r, "/signout", http.StatusFound)
	})
}
