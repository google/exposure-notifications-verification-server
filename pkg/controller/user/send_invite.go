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
	"errors"
	"fmt"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
)

func (c *Controller) sendInvitation(ctx context.Context, toEmail string) error {
	if err := c.sendInvitationFromRealmEmailer(ctx, toEmail); err == nil {
		return nil
	}

	// Fallback to Firebase

	if err := c.firebaseInternal.SendNewUserInvitation(ctx, toEmail); err != nil {
		c.logger.Warnw("failed sending invitation", "error", err)
		return fmt.Errorf("failed sending invitation: %w", err)
	}
	return nil
}

func (c *Controller) sendInvitationFromRealmEmailer(ctx context.Context, toEmail string) error {
	// Send email with realm email config
	realm := controller.RealmFromContext(ctx)
	if realm == nil {
		return errors.New("no realm found")
	}

	emailer, err := realm.EmailProvider(c.db)
	if err != nil {
		c.logger.Warnw("failed to get emailer for realm:", "error", err)
		return fmt.Errorf("failed to get emailer for realm: %w", err)
	}

	message, err := controller.ComposeInviteEmail(ctx, c.h, c.client, toEmail, emailer.From(), realm.Name)
	if err != nil {
		c.logger.Warnw("failed composing invitation", "error", err)
		return fmt.Errorf("failed composing invitation: %w", err)
	}

	if err := emailer.SendEmail(ctx, toEmail, message); err != nil {
		c.logger.Warnw("failed sending invitation", "error", err)
		return fmt.Errorf("failed sending invitation: %w", err)
	}

	return nil
}
