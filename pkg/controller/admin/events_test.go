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

package admin_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/browser"
	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"

	"github.com/chromedp/chromedp"
)

// This goes to the value of a <input type="datetime-local">
const rfc3339PartialLocal = "2006-01-02T15:04:05"

func TestShowAdminEvents(t *testing.T) {
	t.Parallel()

	harness := envstest.NewServer(t, testDatabaseInstance)

	// Get the default realm
	realm, err := harness.Database.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}

	// Create a system admin
	admin := &database.User{
		Email:       "admin@example.com",
		Name:        "Admin",
		SystemAdmin: true,
		Realms:      []*database.Realm{realm},
		AdminRealms: []*database.Realm{realm},
	}
	if err := harness.Database.SaveUser(admin, database.SystemTest); err != nil {
		t.Fatal(err)
	}

	eventTime, err := time.Parse(time.RFC3339, "2020-03-11T12:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	audit := &database.AuditEntry{
		RealmID:       0, // system entry
		Action:        "test action",
		TargetID:      "testTargetID",
		TargetDisplay: "test target",
		ActorID:       "testActorID",
		ActorDisplay:  "test actor",
		CreatedAt:     eventTime,
	}
	harness.Database.RawDB().Save(audit)

	// Log in the user.
	session, err := harness.LoggedInSession(nil, admin.Email)
	if err != nil {
		t.Fatal(err)
	}

	// Set the current realm.
	controller.StoreSessionRealm(session, realm)

	// Mint a cookie for the session.
	cookie, err := harness.SessionCookie(session)
	if err != nil {
		t.Fatal(err)
	}
	// Create a browser runner.
	browserCtx := browser.New(t)
	taskCtx, done := context.WithTimeout(browserCtx, 30*time.Second)
	defer done()

	if err := chromedp.Run(taskCtx,
		// Pre-authenticate the user.
		browser.SetCookie(cookie),

		// Visit /admin
		chromedp.Navigate(`http://`+harness.Server.Addr()+`/admin/events`),

		// Wait for render.
		chromedp.WaitVisible(`body#admin-events-index`, chromedp.ByQuery),

		// Search from and hour before to and hour after our event
		chromedp.SetValue(`#from`, eventTime.Add(-time.Hour).Format(rfc3339PartialLocal), chromedp.ByQuery),
		chromedp.SetValue(`#to`, eventTime.Add(time.Hour).Format(rfc3339PartialLocal), chromedp.ByQuery),
		chromedp.Submit(`form#search-form`, chromedp.ByQuery),

		// Wait for the search result.
		chromedp.WaitVisible(`#results #event`, chromedp.ByQuery),

		// Search an hour before the event.
		chromedp.SetValue(`#from`, eventTime.Add(-2*time.Hour).Format(rfc3339PartialLocal), chromedp.ByQuery),
		chromedp.SetValue(`#to`, eventTime.Add(-time.Hour).Format(rfc3339PartialLocal), chromedp.ByQuery),
		chromedp.Submit(`form#search-form`, chromedp.ByQuery),

		// Assert no event found
		chromedp.WaitNotPresent(`#results #event`, chromedp.ByQuery),
	); err != nil {
		t.Fatal(err)
	}
}
