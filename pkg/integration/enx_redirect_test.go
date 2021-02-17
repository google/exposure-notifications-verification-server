// Copyright 2021 Google LLC
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

package integration_test

import (
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/clients"
	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

func TestENXRedirect(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	server := envstest.NewENXRedirectServer(t, testDatabaseInstance)

	bs, err := envstest.Bootstrap(ctx, server.Database)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := bs.Cleanup(); err != nil {
			t.Fatal(err)
		}
	})

	realm := bs.Realm
	realm.EnableENExpress = true
	realm.SMSTextTemplate = "[enslink]"
	if err := server.Database.SaveRealm(realm, database.SystemTest); err != nil {
		t.Fatal(err)
	}

	client, err := clients.NewENXRedirectClient("http://"+server.Server.Addr(),
		clients.WithHostOverride("e2e-test.test.local"))
	if err != nil {
		t.Fatal(err)
	}

	if err := client.RunE2E(ctx); err != nil {
		t.Fatal(err)
	}
}
