// Copyright 2022 the Exposure Notifications Verification Server authors
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

// Package email implements periodic email sending.
package emailer

import (
	"crypto/tls"
	"fmt"
	"net/mail"
	"net/smtp"

	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

const (
	emailerAnomaliesLock = "emailerAnomaliesLock"
	emailerSMSErrorsLock = "emailerSMSErrorsLock"
)

type Controller struct {
	config *config.EmailerConfig
	db     *database.Database
	h      *render.Renderer
}

func New(cfg *config.EmailerConfig, db *database.Database, h *render.Renderer) *Controller {
	return &Controller{
		config: cfg,
		db:     db,
		h:      h,
	}
}

// sendMail sends a single message through the Google Workspace SMTP relay.
func (c *Controller) sendMail(to string, msg []byte) error {
	toAddr, err := mail.ParseAddress(to)
	if err != nil {
		return fmt.Errorf("failed to parse to address %q: %w", to, err)
	}

	from := c.config.FromAddress
	fromAddr, err := mail.ParseAddress(from)
	if err != nil {
		return fmt.Errorf("failed to parse from address %q: %w", from, err)
	}

	client, err := smtp.Dial(c.config.SMTPRelayHost + ":" + c.config.SMTPRelayPort)
	if err != nil {
		return fmt.Errorf("failed to dial connection: %w", err)
	}
	defer client.Close()

	if err := client.Hello(c.config.MailDomain); err != nil {
		return fmt.Errorf("failed to HELLO: %w", err)
	}

	if err := client.StartTLS(&tls.Config{
		ServerName: c.config.SMTPRelayHost,
		MinVersion: tls.VersionTLS13,
	}); err != nil {
		return fmt.Errorf("failed to start tls: %w", err)
	}

	if err := client.Mail(fromAddr.Address); err != nil {
		return fmt.Errorf("failed to set FROM: %w", err)
	}

	if err := client.Rcpt(toAddr.Address); err != nil {
		return fmt.Errorf("failed to set TO: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to create data stream: %w", err)
	}

	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("failed to write body: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("failed to close connection: %w", err)
	}

	if err := client.Quit(); err != nil {
		return fmt.Errorf("failed to quit client: %w", err)
	}

	return nil
}
