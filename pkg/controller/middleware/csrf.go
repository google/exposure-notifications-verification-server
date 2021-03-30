// Copyright 2021 Google LLC
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

package middleware

import (
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"html/template"
	"net/http"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"github.com/gorilla/mux"
)

const (
	// CSRFHeaderField is the name of the header where the CSRF token resides.
	CSRFHeaderField = "X-CSRF-Token"

	// CSRFFormField is the form field name.
	CSRFFormField         = "csrf_token"
	CSRFFormFieldTemplate = `<input type="hidden" name="%s" value="%s" />`

	// CSRFMetaTagName is the meta tag name (used by Javascript).
	CSRFMetaTagName     = "csrf-token"
	CSRFMetaTagTemplate = `<meta name="%s" content="%s" />`

	// TokenLength is the length of the token (in bytes).
	TokenLength = 64
)

const (
	ErrMissingExistingToken = Error("missing existing csrf token in session")
	ErrMissingIncomingToken = Error("missing csrf token in request")
	ErrInvalidToken         = Error("invalid csrf token")
)

type Error string

func (e Error) Error() string { return string(e) }

// HandleCSRF first extracts an existing CSRF token from the session (if one
// exists). Then, it generates a unique, per-request CSRF token and stores it in
// the session. This must come after RequireSession to ensure the session has
// been populated.
func HandleCSRF(h *render.Renderer) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			logger := logging.FromContext(ctx).Named("middleware.HandleCSRF")

			session := controller.SessionFromContext(ctx)
			if session == nil {
				controller.MissingSession(w, r, h)
				return
			}

			// Get the existing CSRF token from the session, if one exists.
			existingToken := controller.CSRFTokenFromSession(session)

			// If the existing token is invalid or missing, generate and store a new
			// token on the session.
			if l := len(existingToken); l != TokenLength {
				newToken, err := project.RandomBytes(TokenLength)
				if err != nil {
					controller.InternalError(w, r, h, err)
					return
				}
				controller.StoreSessionCSRFToken(session, newToken)

				// Set the existing token to this new token. The only way we got into
				// this block was if the existing token was missing or invalid. That
				// means it's not safe to pass to the template or request helpers or
				// else we risk future requests failing. The new token will fail
				// validation, if applicable, below.
				existingToken = newToken
			}

			// Save the csrf functions on the template map for use in HTML templates.
			masked, err := mask(existingToken)
			if err != nil {
				controller.InternalError(w, r, h, err)
				return
			}
			m := controller.TemplateMapFromContext(ctx)
			m["csrfHeaderField"] = CSRFHeaderField
			m["csrfMetaTagName"] = CSRFMetaTagName
			m["csrfToken"] = template.HTML(masked)
			m["csrfField"] = template.HTML(fmt.Sprintf(CSRFFormFieldTemplate, CSRFFormField, masked))
			m["csrfMeta"] = template.HTML(fmt.Sprintf(CSRFMetaTagTemplate, CSRFMetaTagName, masked))
			ctx = controller.WithTemplateMap(ctx, m)
			r = r.Clone(ctx)

			// Only mutating methods need CSRF verification.
			if canSkipCSRFCheck(r) {
				next.ServeHTTP(w, r)
				return
			}

			// Grab the incoming token from the request.
			incomingToken, err := tokenFromRequest(r)
			if err != nil {
				// Warn, but don't return an error here. The token comparison below will
				// correctly fail and we can return the correct response there. This is
				// almost certainly user error with the provided token in the request.
				logger.Warnw("invalid csrf token from request", "error", err)
			}

			if subtle.ConstantTimeCompare(existingToken, incomingToken) != 1 {
				controller.Unauthorized(w, r, h)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// tokenFromRequest extracts the provided token from the request, then the form,
// then the multi-part form. The token must be a valid base64 URL-encoded string
// generated from the csrf middleware. If the token is missing, it returns
// ErrMissingIncomingToken. If the token is present but cannot be decoded, it
// returns ErrInvalidToken.
func tokenFromRequest(r *http.Request) ([]byte, error) {
	str := r.Header.Get(CSRFHeaderField)

	if str == "" {
		str = r.PostFormValue(CSRFFormField)
	}

	if str == "" && r.MultipartForm != nil {
		vals, ok := r.MultipartForm.Value[CSRFFormField]
		if ok && len(vals) > 0 {
			str = vals[0]
		}
	}

	if str == "" {
		return nil, ErrMissingIncomingToken
	}

	b, err := base64.RawURLEncoding.DecodeString(str)
	if err != nil {
		return nil, ErrInvalidToken
	}

	raw, err := unmask(b)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

// canSkipCSRFCheck returns true if the request does not need CSRF validation,
// false otherwise. Only mutating methods need CSRF verification.
func canSkipCSRFCheck(r *http.Request) bool {
	return r.Method == http.MethodGet || r.Method == http.MethodHead ||
		r.Method == http.MethodOptions || r.Method == http.MethodTrace
}

// mask xors the raw token with random bytes and prepends those random bytes to
// the result.
func mask(raw []byte) (string, error) {
	b, err := project.RandomBytes(len(raw))
	if err != nil {
		return "", fmt.Errorf("maskToken: %w", err)
	}

	b = append(b, xor(b, raw)...)
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// unmask splits the raw token into its otp and token parts and XORs the values
// again to get back the original token. It returns an error if the slice is too
// small or of an uneven length.
func unmask(raw []byte) ([]byte, error) {
	if len(raw) < 2 || len(raw)%2 != 0 {
		return nil, ErrInvalidToken
	}

	half := len(raw) / 2
	return xor(raw[half:], raw[:half]), nil
}

// xor takes the XOR of x of y. len(y) must be >= len(x).
func xor(x, y []byte) []byte {
	result := make([]byte, len(x))
	for i := 0; i < len(x); i++ {
		result[i] = x[i] ^ y[i]
	}
	return result
}
