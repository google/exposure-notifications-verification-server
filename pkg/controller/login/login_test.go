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

package login_test

import (
	"context"
	"testing"

	"github.com/chromedp/chromedp"
	"github.com/google/exposure-notifications-verification-server/internal/browser"
	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

var testDatabaseInstance *database.TestInstance

func TestMain(m *testing.M) {
	testDatabaseInstance = database.MustTestInstance()
	defer testDatabaseInstance.MustClose()
	m.Run()
}

func TestHandleLogin_ShowLogin(t *testing.T) {
	t.Parallel()

	harness := envstest.NewServer(t, testDatabaseInstance)

	browserCtx := browser.New(t)
	taskCtx, done := context.WithTimeout(browserCtx, project.TestTimeout())
	defer done()

	if err := chromedp.Run(taskCtx,
		chromedp.Navigate(`http://`+harness.Server.Addr()),
		chromedp.WaitVisible(`body#login`, chromedp.ByQuery),
	); err != nil {
		t.Fatal(err)
	}
}
