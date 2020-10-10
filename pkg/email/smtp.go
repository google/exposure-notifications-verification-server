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
	"bytes"
	"context"
	"fmt"
	"html/template"
	"net/smtp"

	"firebase.google.com/go/auth"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
)

var _ Provider = (*SMTPProvider)(nil)

// SMTPProvider sends messages via an external SMTP server.
type SMTPProvider struct {
	FirebaseAuth *auth.Client

	InviteTemplate *template.Template

	User     string
	Password string
	SMTPHost string
	SMTPPort string
}

// NewSMTP creates a new Smtp email sender with the given auth.
func NewSMTP(ctx context.Context, user, password, host, port, assetsRoot string, auth *auth.Client) (Provider, error) {
	t, err := template.ParseFiles(assetsRoot + "/email/default_invitation.html")
	if err != nil {
		return nil, fmt.Errorf("failed parsing invite template %v", err)
	}

	return &SMTPProvider{
		FirebaseAuth:   auth,
		InviteTemplate: t,
		User:           user,
		Password:       password,
		SMTPHost:       host,
		SMTPPort:       port,
	}, nil
}

// SendNewUserInvitation sends a password reset email to the user.
func (s *SMTPProvider) SendNewUserInvitation(ctx context.Context, toEmail string) error {
	inviteLink, err := s.FirebaseAuth.PasswordResetLink(ctx, toEmail)
	if err != nil {
		return err
	}

	realmName := ""
	if realm := controller.RealmFromContext(ctx); realm != nil {
		realmName = realm.Name
	}

	// Compose message
	var message bytes.Buffer
	s.InviteTemplate.Execute(&message, struct {
		ToEmail    string
		FromEmail  string
		InviteLink string
		RealmName  string
	}{
		ToEmail:    toEmail,
		FromEmail:  s.User,
		InviteLink: inviteLink,
		RealmName:  realmName,
	})

	// Authentication.
	auth := smtp.PlainAuth("", s.User, s.Password, s.SMTPHost)

	// Sending email.
	err = smtp.SendMail(s.SMTPHost+":"+s.SMTPPort, auth, s.User, []string{toEmail}, message.Bytes())
	if err != nil {
		return err
	}
	return nil
}
