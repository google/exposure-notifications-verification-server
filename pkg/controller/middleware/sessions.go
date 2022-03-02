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

package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/internal/auth"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"github.com/hashicorp/go-multierror"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"go.uber.org/zap"
)

const (
	// sessionName is the name of the session.
	sessionName     = "verification-server-session"
	authSessionName = "verification-user"
)

// RequireNamedSession retrieves or creates a new session with a specific name, other
// than the default session name.
func RequireNamedSession(store sessions.Store, name string, authProvider auth.Provider, h *render.Renderer) func(http.Handler) http.Handler {
	return buildHandler(store, name, authProvider, h)
}

// RequireSession retrieves or creates a new session and stores it on the
// request's context for future retrieval. It also ensures the flash data is
// populated in the template map. Any handler that wants to utilize sessions
// should use this middleware.
func RequireSession(store sessions.Store, authProvider auth.Provider, h *render.Renderer) func(http.Handler) http.Handler {
	return buildHandler(store, sessionName, authProvider, h)
}

func buildHandler(store sessions.Store, name string, authProvider auth.Provider, h *render.Renderer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			logger := logging.FromContext(ctx).Named("middleware.RequireSession")

			// Get or create a session from the store.
			session, err := store.Get(r, name)
			if err != nil {
				logger.Errorw("failed to get session", "error", err)

				// We couldn't get a session (invalid cookie, can't talk to redis,
				// whatever). According to the spec, this can return an error but can never
				// return an empty session. We intentionally discard the error to ensure we
				// have a session.
				session, _ = store.New(r, name)
			}
			var authSession *sessions.Session
			if authProvider != nil {
				authSession, err = store.Get(r, authSessionName)
				if err != nil {
					logger.Errorw("failed to get auth session", "error", err)
					authSession, _ = store.New(r, name)
				}
				if err = authProvider.JoinAuthCookie(session, authSession); err != nil {
					logger.Debugw("failed to join sessions", "error", err)
				}
			}

			// Save the flash in the template map.
			m := controller.TemplateMapFromContext(ctx)
			m["flash"] = controller.Flash(session)
			ctx = controller.WithTemplateMap(ctx, m)

			// Save the session on the context.
			ctx = controller.WithSession(ctx, session)
			r = r.Clone(ctx)

			// Ensure the session is saved at least once. This is passed to the
			// before-first-byte writer AND called after the middleware executes to
			// ensure the session is always sent.
			var once sync.Once
			saveSession := func() error {
				var merr *multierror.Error
				once.Do(func() {
					if authSession != nil {
						if err := authProvider.SplitAuthCookie(session, authSession); err != nil {
							merr = multierror.Append(merr, err)
						}
						// even if the split fails, save the auth session (force logout).
						if err := authSession.Save(r, w); err != nil {
							merr = multierror.Append(merr, err)
						}
					}
					session := controller.SessionFromContext(ctx)
					if session != nil {
						controller.StoreSessionLastActivity(session, time.Now())
						if err := session.Save(r, w); err != nil {
							merr = multierror.Append(merr, err)
						}
					}
				})
				return merr.ErrorOrNil()
			}

			bfbw := &beforeFirstByteWriter{
				w:      w,
				before: saveSession,
				logger: logger,
			}

			next.ServeHTTP(bfbw, r)

			// Ensure the session is saved - this will happen if no bytes were
			// written (perhaps due to a redirect or empty body).
			if err := saveSession(); err != nil {
				controller.InternalError(w, r, h, err)
				return
			}
		})
	}
}

// CheckSessionIdleNoAuth is an explicit check for session idleness. This check is also performed along with authentication
// and is intended to be used when no other auth check is performed.
func CheckSessionIdleNoAuth(h *render.Renderer, sessionIdleTTL time.Duration) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			if session := controller.SessionFromContext(ctx); session != nil {
				// Check session idle timeout.
				if t := controller.LastActivityFromSession(session); !t.IsZero() {
					// If it's been more than the TTL since we've seen this session,
					// expire it by creating a new empty session.
					if time.Since(t) > sessionIdleTTL {
						controller.RedirectToLogout(w, r, h)
						return
					}
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// beforeFirstByteWriter is a custom http.ResponseWriter with a hook to run
// before the first byte is written. This is useful if you want to store a
// cookie or some other information that must be sent before any body bytes.
type beforeFirstByteWriter struct {
	w http.ResponseWriter

	before func() error
	logger *zap.SugaredLogger
}

func (w *beforeFirstByteWriter) Header() http.Header {
	return w.w.Header()
}

func (w *beforeFirstByteWriter) WriteHeader(c int) {
	if err := w.before(); err != nil {
		w.logger.Errorw("failed to invoke before() in beforeFirstByteWriter", "error", err)
	}
	w.w.WriteHeader(c)
}

func (w *beforeFirstByteWriter) Write(b []byte) (int, error) {
	if err := w.before(); err != nil {
		return 0, err
	}
	return w.w.Write(b)
}
