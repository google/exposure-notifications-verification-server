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

package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/flash"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"github.com/gorilla/sessions"
	"go.uber.org/zap"
)

var (
	ErrMissingRealm = errors.New("missing realm")
)

type RequireRealmHandler struct {
	h        *render.Renderer
	logger   *zap.SugaredLogger
	sessions sessions.Store
}

// RequireRealm requires that a user is logged in and that a realm
// is selectted. This middleware must be run after RequireAuth.
func RequireRealm(ctx context.Context, h *render.Renderer, sessions sessions.Store) *RequireRealmHandler {
	logger := logging.FromContext(ctx)

	return &RequireRealmHandler{
		h:        h,
		logger:   logger,
		sessions: sessions,
	}
}

func (h *RequireRealmHandler) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		if err := func() error {
			// Get user from the context
			user := controller.UserFromContext(ctx)
			if user == nil {
				return fmt.Errorf("user is not a database.User")
			}

			// Get the session
			session, err := h.sessions.Get(r, "session")
			if err != nil {
				return fmt.Errorf("failed to get session: %w", err)
			}

			// Verify realm is set on session
			if session.Values == nil || session.Values["realm"] == nil {
				return ErrMissingRealm
			}

			// Get active realm ID
			realmID, ok := session.Values["realm"].(uint)
			if !ok {
				return fmt.Errorf("realm is not a uint")
			}

			// Make sure the user can see this realm.
			realm := user.GetRealm(realmID)
			if realm == nil {
				return fmt.Errorf("not authorized to use realm")
			}

			r = r.WithContext(controller.WithRealm(ctx, realm))
			return nil
		}(); err != nil {
			h.logger.Errorw("RequireRealm", "error", err)

			if errors.Is(err, ErrMissingRealm) {
				flash.FromContext(w, r).Error("Select a realm to continue")
				http.Redirect(w, r, "/realm", http.StatusSeeOther)
				return
			}

			flash.FromContext(w, r).Error("Internal error, you have been logged out.")
			http.Redirect(w, r, "/signout", http.StatusFound)
			return
		}

		next.ServeHTTP(w, r)
	})
}
