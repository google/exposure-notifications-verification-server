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

package user_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/browser"
	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"

	"github.com/chromedp/chromedp"
)

func TestUpdate(t *testing.T) {
	t.Parallel()

	harness := envstest.NewServer(t, testDatabaseInstance)

	realm, _, session, err := harness.ProvisionAndLogin()
	if err != nil {
		t.Fatal(err)
	}

	// Mint a cookie for the session.
	cookie, err := harness.SessionCookie(session)
	if err != nil {
		t.Fatal(err)
	}

	// Create another user.
	user := &database.User{
		Email: "user@example.com",
		Name:  "User",
	}
	if err := harness.Database.SaveUser(user, database.SystemTest); err != nil {
		t.Fatal(err)
	}
	if err := user.AddToRealm(harness.Database, realm, rbac.LegacyRealmAdmin, database.SystemTest); err != nil {
		t.Fatal(err)
	}

	// Create a browser runner.
	browserCtx := browser.New(t)
	taskCtx, done := context.WithTimeout(browserCtx, 30*time.Second)
	defer done()

	for _, permission := range rbac.NamePermissionMap {
		permission := permission
		targets := []string{fmt.Sprintf(`input#permission-%s`, permission)}

		// We also need to remove permissions that imply this permission, or it will
		// be added back in.
		for _, superPerm := range rbac.ImpliedBy(permission) {
			targets = append(targets, fmt.Sprintf(`input#permission-%s`, superPerm))
		}

		// Build the actions as an array prior to run since super-permissions actions aren't known until runtime.
		actions := []chromedp.Action{
			// Pre-authenticate the user.
			browser.SetCookie(cookie),

			// Visit /realm/users.
			chromedp.Navigate(fmt.Sprintf(`http://`+harness.Server.Addr()+`/realm/users/%d/edit`, user.ID)),

			// Wait for render.
			chromedp.WaitVisible(`body#users-edit`, chromedp.ByQuery),
		}

		// Fill out the form.
		for _, target := range targets {
			actions = append(actions, chromedp.RemoveAttribute(target, "checked", chromedp.ByQuery))
		}

		actions = append(actions, chromedp.Submit(`form#user-form`, chromedp.ByQuery))

		// Wait for render.
		actions = append(actions, chromedp.WaitVisible(`body#users-show`, chromedp.ByQuery))

		if err := chromedp.Run(taskCtx, actions...); err != nil {
			t.Fatal(err)
		}
	}

	// Assert the user has no permissions left
	membership, err := user.FindMembership(harness.Database, realm.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := int64(membership.Permissions), int64(0); got != want {
		t.Errorf("expected %v to be %v", got, want)
	}

	// Now add permissions back
	for _, permission := range rbac.NamePermissionMap {
		permission := permission
		target := fmt.Sprintf(`input#permission-%s`, permission)

		if err := chromedp.Run(taskCtx,
			// Pre-authenticate the user.
			browser.SetCookie(cookie),

			// Visit /realm/users.
			chromedp.Navigate(fmt.Sprintf(`http://`+harness.Server.Addr()+`/realm/users/%d/edit`, user.ID)),

			// Wait for render.
			chromedp.WaitVisible(`body#users-edit`, chromedp.ByQuery),

			// Fill out the form.
			chromedp.SetAttributeValue(target, "checked", "true", chromedp.ByQuery),
			chromedp.Submit(`form#user-form`, chromedp.ByQuery),

			// Wait for render.
			chromedp.WaitVisible(`body#users-show`, chromedp.ByQuery),
		); err != nil {
			t.Fatal(err)
		}
	}

	// Permissions should be back
	membership, err = user.FindMembership(harness.Database, realm.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := int64(membership.Permissions), int64(32766); got != want {
		t.Errorf("expected %v to be %v", got, want)
	}
}
