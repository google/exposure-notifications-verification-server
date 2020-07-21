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

package sms

import (
	"context"
)

var _ Provider = (*Noop)(nil)

// Noop does nothing.
type Noop struct{}

// NewNoop creates a new SMS sender that does nothing.
func NewNoop(_ context.Context) (Provider, error) {
	return &Noop{}, nil
}

// SendSMS does nothing.
func (p *Noop) SendSMS(_ context.Context, _, _ string) error {
	return nil
}
