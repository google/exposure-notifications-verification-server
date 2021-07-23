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

// Package controller defines common utilities used by web and API controllers.
package controller

import (
	"encoding/gob"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

var (
	apiErrorBadRequest   = api.Errorf("bad request")
	apiErrorUnauthorized = api.Errorf("unauthorized")
	apiErrorMissingRealm = api.Errorf("missing realm")

	errMissingAuthorizedApp = fmt.Errorf("authorized app missing in request context")
	errMissingLocale        = fmt.Errorf("locale missing in request context")
	errMissingSession       = fmt.Errorf("session missing in request context")
	errMissingUser          = fmt.Errorf("user missing in request context")
)

func init() {
	gob.Register(sessionKey(""))
}

// Back goes back to the referrer. If the referrer is missing, or if the
// referrer base URL does not match the request base URL, the redirect is to the
// homepage.
func Back(w http.ResponseWriter, r *http.Request, h *render.Renderer) {
	logger := logging.FromContext(r.Context()).Named("controller.Back")

	ref := r.Header.Get("Referer")
	if ref == "" {
		logger.Warnw("referrer is empty")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	refURL, err := url.Parse(ref)
	if err != nil {
		logger.Warnw("failed to parse referrer URL", "error", err)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	if got, want := refURL.Host, r.Host; got != want {
		logger.Warnw("referrer host mismatch", "got", got, "want", want)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, ref, http.StatusSeeOther)
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
		h.RenderHTML500(w, err)
	case prefixInList(accept, ContentTypeJSON):
		h.RenderJSON500(w, err)
	default:
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

// NotFound returns an error indicating the URL was not found.
func NotFound(w http.ResponseWriter, r *http.Request, h *render.Renderer) {
	accept := strings.Split(r.Header.Get("Accept"), ",")
	accept = append(accept, strings.Split(r.Header.Get("Content-Type"), ",")...)

	switch {
	case prefixInList(accept, ContentTypeHTML):
		m := TemplateMapFromContext(r.Context())
		m.Title(http.StatusText(http.StatusNotFound))
		h.RenderHTMLStatus(w, http.StatusNotFound, "404", m)
	case prefixInList(accept, ContentTypeJSON):
		h.RenderJSON(w, http.StatusNotFound, http.StatusText(http.StatusNotFound))
	default:
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	}
}

// RedirectToLogout redirects the user to the logout page to terminate the session.
func RedirectToLogout(w http.ResponseWriter, r *http.Request, h *render.Renderer) {
	accept := strings.Split(r.Header.Get("Accept"), ",")
	accept = append(accept, strings.Split(r.Header.Get("Content-Type"), ",")...)

	if prefixInList(accept, ContentTypeHTML) {
		http.Redirect(w, r, "/signout", http.StatusSeeOther)
		return
	}

	Unauthorized(w, r, h)
	return
}

// Unauthorized returns an error indicating the request was unauthorized. The
// system always returns 401 (even with authentication is provided but
// authorization fails).
func Unauthorized(w http.ResponseWriter, r *http.Request, h *render.Renderer) {
	accept := strings.Split(r.Header.Get("Accept"), ",")
	accept = append(accept, strings.Split(r.Header.Get("Content-Type"), ",")...)

	switch {
	case prefixInList(accept, ContentTypeHTML):
		m := TemplateMapFromContext(r.Context())
		m.Title(http.StatusText(http.StatusUnauthorized))
		h.RenderHTMLStatus(w, http.StatusUnauthorized, "401", m)
	case prefixInList(accept, ContentTypeJSON):
		h.RenderJSON(w, http.StatusUnauthorized, apiErrorUnauthorized)
	default:
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
	}
}

// BadRequest indicates the client sent an invalid request.
func BadRequest(w http.ResponseWriter, r *http.Request, h *render.Renderer) {
	accept := strings.Split(r.Header.Get("Accept"), ",")
	accept = append(accept, strings.Split(r.Header.Get("Content-Type"), ",")...)

	switch {
	case prefixInList(accept, ContentTypeHTML):
		h.RenderHTMLStatus(w, http.StatusBadRequest, "400", nil)
	case prefixInList(accept, ContentTypeJSON):
		h.RenderJSON(w, http.StatusBadRequest, apiErrorBadRequest)
	default:
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
	}
}

// MissingMembership returns an error indicating that the request requires a
// realm selection, but one was not present.
func MissingMembership(w http.ResponseWriter, r *http.Request, h *render.Renderer) {
	accept := strings.Split(r.Header.Get("Accept"), ",")
	accept = append(accept, strings.Split(r.Header.Get("Content-Type"), ",")...)

	switch {
	case prefixInList(accept, ContentTypeHTML):
		flash := Flash(SessionFromContext(r.Context()))
		flash.Error("Please select a realm to continue.")
		http.Redirect(w, r, "/login/select-realm", http.StatusSeeOther)
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
	return
}

// MissingLocale returns an internal error when the locale does not exist.
func MissingLocale(w http.ResponseWriter, r *http.Request, h *render.Renderer) {
	InternalError(w, r, h, errMissingLocale)
	return
}

// MissingSession returns an internal error when the session does not exist.
func MissingSession(w http.ResponseWriter, r *http.Request, h *render.Renderer) {
	InternalError(w, r, h, errMissingSession)
	return
}

// MissingUser returns an internal error when the user does not exist.
func MissingUser(w http.ResponseWriter, r *http.Request, h *render.Renderer) {
	InternalError(w, r, h, errMissingUser)
	return
}

// RedirectToMFA redirects to the MFA registration.
func RedirectToMFA(w http.ResponseWriter, r *http.Request, h *render.Renderer) {
	http.Redirect(w, r, "/login/register-phone", http.StatusSeeOther)
	return
}

// RedirectToChangePassword redirects to the password reset page.
func RedirectToChangePassword(w http.ResponseWriter, r *http.Request, h *render.Renderer) {
	http.Redirect(w, r, "/login/change-password", http.StatusSeeOther)
	return
}

func prefixInList(list []string, prefix string) bool {
	for _, v := range list {
		if strings.HasPrefix(v, prefix) {
			return true
		}
	}
	return false
}
