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
)

func (c *Controller) sendInvitation(ctx context.Context, toEmail string) error {
	sent, err := c.sendInvitationFromRealmEmailer(ctx, toEmail)
	if err != nil {
		c.logger.Warnw("failed sending invitation", "error", err)
	}
	if !sent {
		// fallback to Firebase
		if err := c.firebaseInternal.SendNewUserInvitation(ctx, toEmail); err != nil {
			c.logger.Warnw("failed sending invitation", "error", err)
			return fmt.Errorf("failed sending invitation: %w", err)
		}
	}

	return nil
}

// sendInvitationFromRealmEmailer send email with the realm email config
func (c *Controller) sendInvitationFromRealmEmailer(ctx context.Context, toEmail string) (bool, error) {
	realm := controller.RealmFromContext(ctx)
	if realm == nil {
		return false, nil
	}

	emailer, err := realm.EmailProvider(c.db)
	if err != nil {
		if !database.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed creating email provider: %w", err)
	}

	message, err := controller.ComposeInviteEmail(ctx, c.h, c.client, toEmail, emailer.From(), realm.Name)
	if err != nil {
		return false, fmt.Errorf("failed composing email verification: %w", err)
	}

	if err := emailer.SendEmail(ctx, toEmail, message); err != nil {
		return false, fmt.Errorf("failed sending email verification: %w", err)
	}

	return true, nil
}
