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
)

// ProviderType represents a type of email provider.
type ProviderType string

const (
	ProviderTypeNoop     ProviderType = "NOOP"
	ProviderTypeFirebase ProviderType = "FIREBASE"
	ProviderTypeSMTP     ProviderType = "SIMPLE_SMTP"
)

// Config represents the env var based configuration for email SMTP server connection.
type Config struct {
	ProviderType ProviderType

	User     string `env:"EMAIL_USER" json:",omitempty"`
	Password string `env:"EMAIL_PASSWORD" json:",omitempty"`
	SMTPHost string `env:"EMAIL_SMTP_HOST" json:",omitempty"`
	SMTPPort string `env:"EMAIL_SMTP_PORT" json:",omitempty"`

	// Secrets is the secret configuration. This is used to resolve values that
	// are actually pointers to secrets before returning them to the caller. The
	// table implementation is the source of truth for which values are secrets
	// and which are plaintext.
	Secrets secrets.Config
}

type Provider interface {
	// SendNewUserInvitation sends an invite to join the server.
	SendNewUserInvitation(ctx context.Context, email string) error
}

func ProviderFor(ctx context.Context, c *Config, auth *auth.Client) (Provider, error) {
	switch typ := c.ProviderType; typ {
	case ProviderTypeFirebase:
		return NewFirebase(ctx)
	case ProviderTypeSMTP:
		return NewSMTP(ctx, c.User, c.Password, c.SMTPHost, c.SMTPPort, auth)
	default:
		return nil, fmt.Errorf("unknown email provider type: %v", typ)
	}
}
