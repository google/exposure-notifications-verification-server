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
	"strings"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/google/exposure-notifications-server/pkg/logging"

	"github.com/gorilla/mux"
)

// LoadCurrentRealm loads the selected realm from the cache to the context
func LoadCurrentRealm(ctx context.Context, cacher cache.Cacher, db *database.Database, h *render.Renderer) mux.MiddlewareFunc {
	logger := logging.FromContext(ctx).Named("middleware.RequireRealm")

	cacheTTL := 5 * time.Minute

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			session := controller.SessionFromContext(ctx)
			if session == nil {
				controller.MissingSession(w, r, h)
				return
			}

			realmID := controller.RealmIDFromSession(session)
			if realmID == 0 {
				logger.Debugw("realm does not exist in session")
				// If no realm on session, continue serving.
				// If realm is non-optional the caller should RequireRealm or RequireAdmin.
				next.ServeHTTP(w, r)
				return
			}

			// Load the realm by using the cache to alleviate pressure on the database
			// layer.
			var realm database.Realm
			cacheKey := fmt.Sprintf("realms:by_id:%d", realmID)
			if err := cacher.Fetch(ctx, cacheKey, &realm, cacheTTL, func() (interface{}, error) {
				return db.FindRealm(realmID)
			}); err != nil {
				if database.IsNotFound(err) {
					logger.Debugw("realm does not exist")
					controller.MissingRealm(w, r, h)
					return
				}

				logger.Errorw("failed to lookup realm", "error", err)
				controller.InternalError(w, r, h, err)
				return
			}

			// Save the realm on the context.
			ctx = controller.WithRealm(ctx, &realm)
			*r = *r.WithContext(ctx)

			next.ServeHTTP(w, r)
		})
	}
}

// RequireRealm requires a realm to exist in the session. It also ensures the
// realm is set as currentRealm in the template map.
//
// Must come after:
//   LoadCurrentRealm to populate the current realm.
//   RequireAuth so that a user is set on the context.
func RequireRealm(ctx context.Context, h *render.Renderer) mux.MiddlewareFunc {
	logger := logging.FromContext(ctx).Named("middleware.RequireRealm")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			currentUser := controller.UserFromContext(ctx)
			if currentUser == nil {
				controller.MissingUser(w, r, h)
				return
			}

			realm := controller.RealmFromContext(ctx)
			if realm == nil {
				controller.MissingRealm(w, r, h)
				return
			}

			if !currentUser.CanViewRealm(realm.ID) {
				logger.Debugw("user cannot view realm")
				// Technically this is unauthorized, but we don't want to leak the
				// existence of a realm by returning a different error.
				controller.MissingRealm(w, r, h)
				return
			}

			if passwordRedirectRequired(ctx, currentUser, realm) {
				controller.RedirectToChangePassword(w, r, h)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireRealmAdmin verifies the user is an admin of the current realm.
//
// Must come after:
//   LoadCurrentRealm to populate the current realm.
//   RequireAuth so that a user is set on the context.
func RequireRealmAdmin(ctx context.Context, h *render.Renderer) mux.MiddlewareFunc {
	logger := logging.FromContext(ctx).Named("middleware.RequireRealmAdmin")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			currentUser := controller.UserFromContext(ctx)
			if currentUser == nil {
				controller.MissingUser(w, r, h)
				return
			}

			realm := controller.RealmFromContext(ctx)
			if realm == nil {
				controller.MissingRealm(w, r, h)
				return
			}

			if !currentUser.CanAdminRealm(realm.ID) {
				logger.Debugw("user cannot manage realm")
				// Technically this is unauthorized, but we don't want to leak the
				// existence of a realm by returning a different error.
				controller.MissingRealm(w, r, h)
				return
			}

			if passwordRedirectRequired(ctx, currentUser, realm) {
				controller.RedirectToChangePassword(w, r, h)
			}

			next.ServeHTTP(w, r)
		})
	}
}

func passwordRedirectRequired(ctx context.Context, user *database.User, realm *database.Realm) bool {
	err := checkRealmPasswordAge(user, realm)
	if err == nil {
		return false
	}
	session := controller.SessionFromContext(ctx)
	flash := controller.Flash(session)

	if err == errPasswordChangeRequired {
		flash.Error(strings.Title(err.Error() + "."))
		return true
	}

	if !controller.PasswordExpireWarnedFromSession(session) {
		controller.StorePasswordExpireWarned(session, true)
		flash.Warning(strings.Title(err.Error() + "."))
	}
	return false
}

var errPasswordChangeRequired = errors.New("password change required")

func checkRealmPasswordAge(user *database.User, realm *database.Realm) error {
	if realm.PasswordRotationPeriodDays <= 0 {
		return nil
	}

	now := time.Now().UTC()
	nextPasswordChange := user.PasswordChanged().Add(
		time.Hour * 24 * time.Duration(realm.PasswordRotationPeriodDays))

	if now.After(nextPasswordChange) {
		return errPasswordChangeRequired
	}

	if time.Until(nextPasswordChange) <
		time.Hour*24*time.Duration(realm.PasswordRotationWarningDays) {
		untilChange := nextPasswordChange.Sub(now).Hours()
		if daysUntilChange := int(untilChange / 24); daysUntilChange > 1 {
			return fmt.Errorf("password change required in %d days", daysUntilChange)
		}
		if hoursUntilChange := int(untilChange); hoursUntilChange > 1 {
			return fmt.Errorf("password change required in %d hours", hoursUntilChange)
		}
		return fmt.Errorf("password change required soon")
	}

	return nil
}
