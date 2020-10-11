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

package email

import (
	"context"
	"fmt"

	"firebase.google.com/go/auth"
	"github.com/google/exposure-notifications-server/pkg/secrets"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

// ProviderType represents a type of email provider.
type ProviderType string

const (
	// ProviderTypeNoop is a no-op provider
	ProviderTypeNoop ProviderType = "NOOP"

	// ProviderTypeFirebase falls back to firebase's default email template.
	// it uses password-reset rather than a true invitation.
	ProviderTypeFirebase ProviderType = "FIREBASE"

	// ProviderTypeSMTP composes emails and sends them via an external SMTP server.
	ProviderTypeSMTP ProviderType = "SIMPLE_SMTP"
)

// Config represents the env var based configuration for email SMTP server connection.
type Config struct {
	ProviderType ProviderType

	User     string `env:"EMAIL_USER"`
	Password string `env:"EMAIL_PASSWORD"`
	SMTPHost string `env:"EMAIL_SMTP_HOST"`

	// SMTPPort defines the email port to connect to.
	// Note: legacy email port 25 is blocked on GCP and many other systems.
	SMTPPort string `env:"EMAIL_SMTP_PORT, default=587"`

	// Secrets is the secret configuration. This is used to resolve values that
	// are actually pointers to secrets before returning them to the caller. The
	// table implementation is the source of truth for which values are secrets
	// and which are plaintext.
	Secrets secrets.Config
}

// Provider is an interface for email-sending mechanisms.
type Provider interface {
	// SendNewUserInvitation sends an invite to join the server.
	SendNewUserInvitation(ctx context.Context, email string) error
}

// HasSMTPCreds returns true if required fields for connecting to SMTP are set.
func (c *Config) HasSMTPCreds() bool {
	return c.User != "" && c.Password != "" && c.SMTPHost != "" && c.SMTPPort != ""
}

// ProviderFor creates an email provider given a Config.
func ProviderFor(ctx context.Context, c *Config, h *render.Renderer, auth *auth.Client) (Provider, error) {
	switch typ := c.ProviderType; typ {
	case ProviderTypeNoop:
		return NewNoop(), nil
	case ProviderTypeFirebase:
		return NewFirebase(ctx)
	case ProviderTypeSMTP:
		return NewSMTP(ctx, c.User, c.Password, c.SMTPHost, c.SMTPPort, h, auth), nil
	default:
		return nil, fmt.Errorf("unknown email provider type: %v", typ)
	}
}
