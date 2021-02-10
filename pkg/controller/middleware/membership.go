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

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/gorilla/mux"
)

// LoadCurrentMembership attempts to load the current membership. If there is no
// current membership in the session, it does nothing. If a membership exists,
// but fails to load from the database/cache, it returns an error. Use
// RequireMembership to enforce membership.
//
// This must come after RequireAuth so that the user is loaded onto the context.
func LoadCurrentMembership(h *render.Renderer) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			session := controller.SessionFromContext(ctx)
			if session == nil {
				controller.MissingSession(w, r, h)
				return
			}

			currentUser := controller.UserFromContext(ctx)
			if currentUser == nil {
				controller.MissingUser(w, r, h)
				return
			}

			// Extract the current realm ID from the session.
			realmID := controller.RealmIDFromSession(session)
			if realmID == 0 {
				// No realm in session, continue serving
				next.ServeHTTP(w, r)
				return
			}

			// Find the correct membership. The RequireAuth middleware loads the user
			// and all their memberships, so just iterate over that list instead of
			// doing a database lookup. Most users will be a member of a single realm,
			// so N is very, very small.
			var membership *database.Membership
			memberships := controller.MembershipsFromContext(ctx)
			for _, v := range memberships {
				if v.RealmID == realmID {
					membership = v
					break
				}
			}
			if membership == nil {
				// There was a realm in the session, but it does not match a membership
				// of the user. Clear and move along.
				controller.ClearSessionRealm(session)
				next.ServeHTTP(w, r)
				return
			}

			// Save the membership on the context.
			ctx = controller.WithMembership(ctx, membership)
			ctx = controller.WithRealm(ctx, membership.Realm)
			r = r.Clone(ctx)

			next.ServeHTTP(w, r)
		})
	}
}

// RequireMembership requires a membership (realm selection) to exist in the
// session.
//
// This must come after LoadCurrentMembership so the membership is on the
// context
func RequireMembership(h *render.Renderer) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			session := controller.SessionFromContext(ctx)
			if session == nil {
				controller.MissingSession(w, r, h)
				return
			}

			membership := controller.MembershipFromContext(ctx)
			if membership == nil {
				controller.MissingMembership(w, r, h)
				return
			}

			if passwordRedirectRequired(ctx, membership.User, membership.Realm) {
				controller.RedirectToChangePassword(w, r, h)
				return
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
