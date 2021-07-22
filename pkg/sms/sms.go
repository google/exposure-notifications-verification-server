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

// Package sms defines interfaces for sending SMS text messages.
//
// Although exported, this package is non intended for general consumption. It
// is a shared dependency between multiple exposure notifications projects. We
// cannot guarantee that there won't be breaking changes in the future.
package sms

import (
	"context"
	"fmt"
)

// ProviderType represents a type of SMS provider.
type ProviderType string

const (
	ProviderTypeNoop     ProviderType = "NOOP"
	ProviderTypeNoopFail ProviderType = "NOOP_FAIL"
	ProviderTypeTwilio   ProviderType = "TWILIO"
)

// Config represents configuration for an SMS provider.
type Config struct {
	ProviderType ProviderType

	// Twilio options
	TwilioAccountSid string
	TwilioAuthToken  string
	TwilioFromNumber string
}

type Provider interface {
	// SendSMS sends an SMS text message from the given number to the given number
	// with the provided message.
	SendSMS(ctx context.Context, to, message string) error
}

func ProviderFor(ctx context.Context, c *Config) (Provider, error) {
	switch typ := c.ProviderType; typ {
	case ProviderTypeNoop:
		return NewNoop(ctx)
	case ProviderTypeNoopFail:
		return NewNoopFail(ctx)
	case ProviderTypeTwilio:
		return NewTwilio(ctx, c.TwilioAccountSid, c.TwilioAuthToken, c.TwilioFromNumber)
	default:
		return nil, fmt.Errorf("unknown sms provider type: %v", typ)
	}
}
