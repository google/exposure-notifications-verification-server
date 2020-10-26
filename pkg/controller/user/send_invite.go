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
	"fmt"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/email"
)

func (c *Controller) sendInvitation(ctx context.Context, toEmail string) error {
	compose := func(emailer email.Provider, realm *database.Realm, toEmail string) ([]byte, error) {
		return controller.ComposeInviteEmail(ctx, c.h, c.client, toEmail, emailer.From(), realm.Name)
	}
	sent, err := controller.SendRealmEmail(ctx, c.db, compose, toEmail)
	if err != nil {
		c.logger.Warnw("failed sending invitation", "error", err)
		return fmt.Errorf("failed sending invitation: %w", err)
	}
	if !sent {
		// fallback to Firebase if not SMTP found
		if err := c.firebaseInternal.SendNewUserInvitation(ctx, toEmail); err != nil {
			c.logger.Warnw("failed sending invitation", "error", err)
			return fmt.Errorf("failed sending invitation: %w", err)
		}
	}

	return nil
}
