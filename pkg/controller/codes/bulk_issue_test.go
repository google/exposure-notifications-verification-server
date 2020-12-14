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

package codes_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/codes"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"github.com/google/exposure-notifications-verification-server/pkg/sms"
)

func TestRenderBulkIssue(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)
	realm := database.NewRealmWithDefaults("Test Realm")
	realm.AllowBulkUpload = true
	ctx = controller.WithRealm(ctx, realm)
	if err := db.SaveRealm(realm, database.SystemTest); err != nil {
		t.Fatalf("failed to save realm: %v", err)
	}

	ctx = controller.WithMembership(ctx, &database.Membership{
		RealmID:     realm.ID,
		Realm:       realm,
		Permissions: rbac.CodeBulkIssue,
	})

	config := &config.ServerConfig{}
	h, err := render.NewTest(ctx, project.Root()+"/cmd/server/assets", t)
	if err != nil {
		t.Fatalf("failed to create renderer: %v", err)
	}
	c := codes.NewServer(ctx, config, db, h)

	sms := &database.SMSConfig{
		RealmID:          realm.ID,
		ProviderType:     sms.ProviderType("TWILIO"),
		TwilioAccountSid: "abc123",
		TwilioAuthToken:  "def123",
		TwilioFromNumber: "+11234567890",
	}
	if err := db.SaveSMSConfig(sms); err != nil {
		t.Fatalf("failed to save SMSConfig: %v", err)
	}

	r := &http.Request{}
	r = r.WithContext(ctx)

	handleFunc := c.HandleBulkIssue()
	handleFunc.ServeHTTP(nil, r)
}
