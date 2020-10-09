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
	"mime/quotedprintable"
	"net/smtp"

	"firebase.google.com/go/auth"
)

var _ Provider = (*SmtpProvider)(nil)

// Smtp sends messages via an external SMTP server.
type SmtpProvider struct {
	FirebaseAuth *auth.Client

	User     string
	Password string
	SmtpHost string
	SmtpPort string
}

// NewSmtp creates a new Smtp email sender with the given auth.
func NewSmtp(ctx context.Context, user, password, host, port string, auth *auth.Client) (Provider, error) {
	return &SmtpProvider{
		FirebaseAuth: auth,
		User:         user,
		Password:     password,
		SmtpHost:     host,
		SmtpPort:     port,
	}, nil
}

// SendNewUserInvitation sends a password reset email to the user.
func (s *SmtpProvider) SendNewUserInvitation(ctx context.Context, toEmail string) error {
	// Header
	header := make(map[string]string)
	header["From"] = s.User
	header["To"] = toEmail
	header["Subject"] = "COVID-19 Verification Server Invitation"

	header["MIME-Version"] = "1.0"
	header["Content-Type"] = fmt.Sprintf(`%s; charset="utf-8"`, "text/html")
	header["Content-Disposition"] = "inline"
	header["Content-Transfer-Encoding"] = "quoted-printable"

	headerMessage := ""
	for key, value := range header {
		headerMessage += fmt.Sprintf("%s: %s\r\n", key, value)
	}

	inviteLink, err := s.FirebaseAuth.PasswordResetLink(ctx, toEmail)
	if err != nil {
		return err
	}

	// Message.
	body := fmt.Sprintf(
		`You've been invited to the COVID-19 Verification Server.
		Use the link below to set up your account. \n\n %s`, inviteLink)
	var bodyMessage bytes.Buffer
	temp := quotedprintable.NewWriter(&bodyMessage)
	temp.Write([]byte(body))
	temp.Close()

	finalMessage := headerMessage + "\r\n" + bodyMessage.String()

	// Authentication.
	auth := smtp.PlainAuth("", s.User, s.Password, s.SmtpHost)

	// Sending email.
	err = smtp.SendMail(s.SmtpHost+":"+s.SmtpPort, auth, s.User, []string{toEmail}, []byte(finalMessage))
	if err != nil {
		return err
	}
	return nil
}
