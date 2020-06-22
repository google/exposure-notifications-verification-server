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

	"github.com/gorilla/csrf"
)

type csrfController struct{}

// NewCSRFAPI creates a new controller that can return CSRF tokens to JSON APIs.
func NewCSRFAPI() http.Handler {
	return &csrfController{}
}

func (ic *csrfController) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	token := csrf.Token(r)
	w.Header().Add("X-CSRF-Token", token)
	controller.WriteJSON(w, http.StatusOK, api.CSRFResponse{CSRFToken: token})
}
