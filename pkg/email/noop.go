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

// Package email is logic for sending email invitations
package email

import (
	"context"

	"github.com/google/exposure-notifications-server/pkg/logging"
)

var _ Provider = (*SMTPProvider)(nil)

type NoopProvider struct{}

func NewNoop() (Provider, error) {
	return &NoopProvider{}, nil
}

// SendNewUserInvitation sends a password reset email to the user.
func (s *NoopProvider) SendNewUserInvitation(ctx context.Context, toEmail string) error {
	logger := logging.FromContext(ctx)
	logger.Infow("Noop send invitation", "email", toEmail)
	return nil
}
