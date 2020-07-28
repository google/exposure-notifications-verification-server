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
	"strconv"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/flash"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"
	httpcontext "github.com/gorilla/context"
	"go.uber.org/zap"
)

var (
	ErrNoRealmSelected = errors.New("no realm selected")
)

type RequireRealmHandler struct {
	logger *zap.SugaredLogger
}

// RequireRealm requires that a user is logged in and that a realm
// is selectted. This middleware must be run after RequireAuth.
func RequireRealm(ctx context.Context) *RequireRealmHandler {
	return &RequireRealmHandler{logging.FromContext(ctx)}
}

func (h *RequireRealmHandler) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := func() error {
			cookie, err := r.Cookie("realm")
			if err != nil {
				return ErrNoRealmSelected
			}

			realmID, err := strconv.ParseInt(cookie.Value, 10, 64)
			if err != nil {
				return ErrNoRealmSelected
			}

			// Get user from the session
			userRaw, ok := httpcontext.GetOk(r, "user")
			if !ok {
				return fmt.Errorf("missing user in session")
			}

			user, ok := userRaw.(*database.User)
			if !ok {
				return fmt.Errorf("user is not a database.User")
			}

			// Make sure the user can see this realm.
			realm := user.GetRealm(uint(realmID))
			if realm == nil {
				return fmt.Errorf("not authorized to use realm")
			}

			r = r.WithContext(controller.WithRealm(r.Context(), realm))
			httpcontext.Set(r, "realm", realm)
			return nil
		}(); err != nil {
			h.logger.Errorw("RequireRealm", "error", err)

			if errors.Is(err, ErrNoRealmSelected) {
				flash.FromContext(w, r).Error("Select a realm")
				http.Redirect(w, r, "/home/realm", http.StatusSeeOther)
			} else {
				flash.FromContext(w, r).Error("Internal error, you have been logged out.")
				http.Redirect(w, r, "/signout", http.StatusFound)
			}
		} else {
			next.ServeHTTP(w, r)
		}
	})
}
