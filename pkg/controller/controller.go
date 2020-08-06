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

// Package controller defines common utilities used by web and API controllers.
package controller

import (
	"encoding/gob"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

var (
	apiErrorUnauthorized = api.Errorf("unauthorized")
	apiErrorMissingRealm = api.Errorf("missing realm")

	errMissingAuthorizedApp = fmt.Errorf("authorized app missing in request context")
	errMissingSession       = fmt.Errorf("session missing in request context")
	errMissingUser          = fmt.Errorf("user missing in request context")
)

func init() {
	gob.Register(*new(sessionKey))
}

// Back goes back to the referrer.
func Back(w http.ResponseWriter, r *http.Request, h *render.Renderer) {
	http.Redirect(w, r, r.Header.Get("Referer"), http.StatusSeeOther)
}

// InternalError handles an internal error, returning the right response to the
// client.
func InternalError(w http.ResponseWriter, r *http.Request, h *render.Renderer, err error) {
	logger := logging.FromContext(r.Context())
	logger.Errorw("internal error", "error", err)

	accept := strings.Split(r.Header.Get("Accept"), ",")
	accept = append(accept, strings.Split(r.Header.Get("Content-Type"), ",")...)

	switch {
	case prefixInList(accept, ContentTypeHTML):
		h.HTML500(w, err)
	case prefixInList(accept, ContentTypeJSON):
		h.JSON500(w, err)
	default:
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

// Unauthorized returns an error indicating the request was unauthorized.
func Unauthorized(w http.ResponseWriter, r *http.Request, h *render.Renderer) {
	accept := strings.Split(r.Header.Get("Accept"), ",")
	accept = append(accept, strings.Split(r.Header.Get("Content-Type"), ",")...)

	switch {
	case prefixInList(accept, ContentTypeHTML):
		flash := Flash(SessionFromContext(r.Context()))
		flash.Error("You are not authorized to perform that action!")
		http.Redirect(w, r, "/", http.StatusSeeOther)
	case prefixInList(accept, ContentTypeJSON):
		h.RenderJSON(w, http.StatusUnauthorized, apiErrorUnauthorized)
	default:
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
	}
}

// MissingRealm returns an error indicating that the request requires a realm
// selection, but one was not present.
func MissingRealm(w http.ResponseWriter, r *http.Request, h *render.Renderer) {
	accept := strings.Split(r.Header.Get("Accept"), ",")
	accept = append(accept, strings.Split(r.Header.Get("Content-Type"), ",")...)

	switch {
	case prefixInList(accept, ContentTypeHTML):
		flash := Flash(SessionFromContext(r.Context()))
		flash.Error("Please select a realm to continue.")
		http.Redirect(w, r, "/realm", http.StatusSeeOther)
	case prefixInList(accept, ContentTypeJSON):
		h.RenderJSON(w, http.StatusBadRequest, apiErrorMissingRealm)
	default:
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
	}
}

// MissingAuthorizedApp returns an internal error when the authorized app does
// not exist.
func MissingAuthorizedApp(w http.ResponseWriter, r *http.Request, h *render.Renderer) {
	InternalError(w, r, h, errMissingAuthorizedApp)
}

// MissingSession returns an internal error when the session does not exist.
func MissingSession(w http.ResponseWriter, r *http.Request, h *render.Renderer) {
	InternalError(w, r, h, errMissingSession)
}

// MissingUser returns an internal error when the user does not exist.
func MissingUser(w http.ResponseWriter, r *http.Request, h *render.Renderer) {
	InternalError(w, r, h, errMissingUser)
}

func prefixInList(list []string, prefix string) bool {
	for _, v := range list {
		if strings.HasPrefix(v, prefix) {
			return true
		}
	}
	return false
}
