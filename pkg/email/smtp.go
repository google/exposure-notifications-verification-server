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

// Package email is logic for sending email invitations
package email

import (
	"context"
	"net/smtp"
	"time"

	"github.com/google/exposure-notifications-server/pkg/logging"
)

var _ Provider = (*SMTPProvider)(nil)

// SMTPProvider sends messages via an external SMTP server.
type SMTPProvider struct {
	User     string
	Password string
	SMTPHost string
	SMTPPort string
}

// NewSMTP creates a new Smtp email sender with the given auth.
func NewSMTP(ctx context.Context, user, password, host, port string) Provider {
	return &SMTPProvider{
		User:     user,
		Password: password,
		SMTPHost: host,
		SMTPPort: port,
	}
}

// SendEmail sends an email to the user.
func (s *SMTPProvider) SendEmail(ctx context.Context, toEmail string, message []byte) error {
	ctx, done := context.WithTimeout(context.Background(), 60*time.Second)

	// Authentication.
	auth := smtp.PlainAuth("", s.User, s.Password, s.SMTPHost)
	go func() {
		defer done()
		s.sendMail(ctx, auth, toEmail, message)
	}()

	return nil
}

func (s *SMTPProvider) sendMail(ctx context.Context, auth smtp.Auth, toEmail string, message []byte) {
	logger := logging.FromContext(ctx)

	// Sending email.
	err := smtp.SendMail(s.SMTPHost+":"+s.SMTPPort, auth, s.User, []string{toEmail}, message)
	if err != nil {
		logger.Warnw("failed to send invitation email", "error", err)
	}
}

// From returns who shown as the sender of the email.
func (s *SMTPProvider) From() string {
	return s.User
}
