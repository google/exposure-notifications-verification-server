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

// Package testsuite provides test environment to run E2E and integration tests..
package testsuite

import (
	"context"
	"testing"
)

const (
	realmNamePrefix = "test-realm"
	realmRegionCode = "test"
	adminKeyName    = "integration-admin-key"
	deviceKeyName   = "integration-device-key"
)

type TestSuite interface {
	NewAdminAPIClient(ctx context.Context, tb testing.TB) (*AdminClient, error)
	NewAPIClient(ctx context.Context, tb testing.TB) (*APIClient, error)
}

// NewTestSuite returns environment specific test suite.
func NewTestSuite(tb testing.TB, ctx context.Context, isE2E bool) TestSuite {
	if isE2E {
		return NewE2ESuite(tb, ctx)
	}
	return NewIntegrationSuite(tb, ctx)
}
