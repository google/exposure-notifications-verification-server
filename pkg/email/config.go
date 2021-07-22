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

package email

import (
	"context"
	"fmt"

	"github.com/google/exposure-notifications-server/pkg/secrets"
)

// ProviderType represents a type of email provider.
type ProviderType string

const (
	// ProviderTypeNoop is a no-op provider
	ProviderTypeNoop ProviderType = "NOOP"

	// ProviderTypeSMTP composes emails and sends them via an external SMTP server.
	ProviderTypeSMTP ProviderType = "SIMPLE_SMTP"
)

// Config represents the env var based configuration for email SMTP server connection.
//
// Note: This will only work with email providers that accept external connections.
//       The provider must accept TLS, and users should independently consider the security
//       of the email provider / account.
//   Gmail or Google Workspace accounts can be used with an app-password, but will
//   not work with security features such as Advanced Protection enabled.
type Config struct {
	ProviderType ProviderType

	User     string `env:"EMAIL_USER"`
	Password string `env:"EMAIL_PASSWORD" json:"-"` // ignored by zap's JSON formatter
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
	// SendEmail sends an email with the given message.
	SendEmail(ctx context.Context, toEmail string, message []byte) error

	// From returns who shown as the sender of the email.
	From() string
}

// HasSMTPCreds returns true if required fields for connecting to SMTP are set.
func (c *Config) HasSMTPCreds() bool {
	return c.User != "" && c.Password != "" && c.SMTPHost != "" && c.SMTPPort != ""
}

// ProviderFor creates an email provider given a Config.
func ProviderFor(ctx context.Context, c *Config) (Provider, error) {
	switch typ := c.ProviderType; typ {
	case ProviderTypeNoop:
		return NewNoop(), nil
	case ProviderTypeSMTP:
		return NewSMTP(ctx, c.User, c.Password, c.SMTPHost, c.SMTPPort), nil
	default:
		return nil, fmt.Errorf("unknown email provider type: %v", typ)
	}
}
