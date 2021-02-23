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

// Package integration runs integration tests. These tests could be internal
// (all the servers are spun up in memory) or it could be via the e2e test which
// communicate across services deployed at distinct URLs.
package integration_test

import (
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/clients"
	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

var testDatabaseInstance *database.TestInstance

func TestMain(m *testing.M) {
	testDatabaseInstance = database.MustTestInstance()
	defer testDatabaseInstance.MustClose()
	m.Run()
}

func TestIntegration(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	integrationSuite := envstest.NewIntegrationSuite(t, testDatabaseInstance)

	cfg := &config.E2ERunnerConfig{
		VerificationAdminAPIServer: integrationSuite.AdminAPIAddress,
		VerificationAdminAPIKey:    integrationSuite.AdminAPIKey,
		VerificationAPIServer:      integrationSuite.APIServerAddress,
		VerificationAPIServerKey:   integrationSuite.APIServerKey,
		DoRevise:                   true,
	}

	if err := clients.RunEndToEnd(ctx, cfg); err != nil {
		t.Fatal(err)
	}
}
