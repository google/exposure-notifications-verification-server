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

package user

import (
	"context"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"go.opencensus.io/stats"
)

func (c *Controller) HandleResetPassword() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.Show(w, r, true /*resetPassword*/)
	})
}

func (c *Controller) resetPasswordUserAssertion(ctx context.Context, user *database.User) error {
	created, err := c.ensureFirebaseUserExists(ctx, user)
	if created {
		stats.Record(ctx, controller.MFirebaseRecreates.M(1))
	}
	return err
}

func (c *Controller) ensureFirebaseUserExists(ctx context.Context, user *database.User) (bool, error) {
	session := controller.SessionFromContext(ctx)
	flash := controller.Flash(session)

	// Ensure the firebase user is created
	created, err := user.CreateFirebaseUser(ctx, c.client)
	if err != nil {
		flash.Alert("Failed to create user auth: %v", err)
		return created, err
	}

	if created {
		if c.emailer == nil {
			// fallback to firebase
			c.config.Firebase
		}

		realm := controller.RealmFromContext(ctx)
		currentUser := controller.UserFromContext(ctx)

		message, err := controller.ComposeInviteEmail(c.h, c.client, user.Email, currentUser.Email, realm.Name)
		if err != nil {
			c.logger.Warnw("failed composing invitation", "error", err)
			flash.Error("Could not send new user invitation.")
			return true, err
		}

		err := c.emailer.SendEmail(ctx, user.Email, message)
		if err != nil {
			c.logger.Warnw("failed sending invitation", "error", err)
			flash.Error("Could not send new user invitation.")
			return true, err
		}
	}
	return created, nil
}
