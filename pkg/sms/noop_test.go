// Copyright 2021 the Exposure Notifications Verification Server authors
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
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/project"
)

func TestNoop_SendSMS(t *testing.T) {
	t.Parallel()
	ctx := project.TestContext(t)

	c := &Config{ProviderType: ProviderTypeNoop}
	p, err := ProviderFor(ctx, c)
	if err != nil {
		t.Fatal(err)
	}

	if err := p.SendSMS(ctx, "+nobody", "noop"); err != nil {
		t.Fatal(err)
	}
}

func TestNoopFail_SendSMS(t *testing.T) {
	t.Parallel()
	ctx := project.TestContext(t)

	c := &Config{ProviderType: ProviderTypeNoopFail}
	p, err := ProviderFor(ctx, c)
	if err != nil {
		t.Fatal(err)
	}

	if err := p.SendSMS(ctx, "+nobody", "noop"); err == nil {
		t.Fatal("Noop fail should always fail")
	}
}
