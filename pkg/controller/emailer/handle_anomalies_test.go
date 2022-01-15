// Copyright 2022 the Exposure Notifications Verification Server authors
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

package emailer

import (
	"strings"
	"testing"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/assets"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestSendAnomaliesEmail(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	h, err := render.New(ctx, assets.ServerFS(), true)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("no_contacts", func(t *testing.T) {
		t.Parallel()

		logCore, logObserver := observer.New(zap.DebugLevel)
		ctx := logging.WithLogger(ctx, zap.New(logCore).Sugar())

		db, _ := testDatabaseInstance.NewDatabase(t, nil)

		realm, err := db.FindRealm(1)
		if err != nil {
			t.Fatal(err)
		}

		realm.ContactEmailAddresses = []string{}
		if err := db.SaveRealm(realm, database.SystemTest); err != nil {
			t.Fatal(err)
		}

		cfg := &config.EmailerConfig{}

		c := New(cfg, db, h)

		if err := c.sendAnomaliesEmails(ctx, realm); err != nil {
			t.Fatal(err)
		}

		testExpectLog(t, logObserver, "no contact email addresses registered")
	})

	t.Run("not_anomalous", func(t *testing.T) {
		t.Parallel()

		logCore, logObserver := observer.New(zap.DebugLevel)
		ctx := logging.WithLogger(ctx, zap.New(logCore).Sugar())

		db, _ := testDatabaseInstance.NewDatabase(t, nil)

		realm, err := db.FindRealm(1)
		if err != nil {
			t.Fatal(err)
		}

		realm.ContactEmailAddresses = []string{"user@example.com"}
		realm.LastCodesClaimedRatio = 1.00
		realm.CodesClaimedRatioMean = 0.01
		realm.CodesClaimedRatioStddev = 0.01
		if err := db.SaveRealm(realm, database.SystemTest); err != nil {
			t.Fatal(err)
		}

		cfg := &config.EmailerConfig{}

		c := New(cfg, db, h)

		if err := c.sendAnomaliesEmails(ctx, realm); err != nil {
			t.Fatal(err)
		}

		testExpectLog(t, logObserver, "codes claimed ratio is not anomalous, skipping")
	})

	t.Run("renders", func(t *testing.T) {
		t.Parallel()

		db, _ := testDatabaseInstance.NewDatabase(t, nil)

		realm, err := db.FindRealm(1)
		if err != nil {
			t.Fatal(err)
		}

		cfg := &config.EmailerConfig{}

		c := New(cfg, db, h)

		t.Run("without_ccs", func(t *testing.T) {
			t.Parallel()

			msg, err := c.h.RenderEmail("email/anomalies", map[string]interface{}{
				"FromAddress": "from@example.com",
				"ToAddresses": []string{"to1@example.com", "to2@example.com"},
				"Realm":       realm,
				"RootURL":     "https://example.com",
			})
			if err != nil {
				t.Fatal(err)
			}

			if got, want := string(msg), "From: from@example.com\n"; !strings.Contains(got, want) {
				t.Errorf("expectd %q to contain %q", got, want)
			}
			if got, want := string(msg), "To: to1@example.com,to2@example.com\n"; !strings.Contains(got, want) {
				t.Errorf("expectd %q to contain %q", got, want)
			}
			if got, want := string(msg), "less than your historical average"; !strings.Contains(got, want) {
				t.Errorf("expectd %q to contain %q", got, want)
			}
		})

		t.Run("with_cc", func(t *testing.T) {
			t.Parallel()

			msg, err := c.h.RenderEmail("email/anomalies", map[string]interface{}{
				"FromAddress":  "from@example.com",
				"ToAddresses":  []string{"to1@example.com", "to2@example.com"},
				"CCAddresses":  []string{"cc1@example.com", "cc2@example.com"},
				"BCCAddresses": []string{"bcc1@example.com", "bcc2@example.com"},
				"Realm":        realm,
				"RootURL":      "https://example.com",
			})
			if err != nil {
				t.Fatal(err)
			}

			if got, want := string(msg), "From: from@example.com\n"; !strings.Contains(got, want) {
				t.Errorf("expectd %q to contain %q", got, want)
			}
			if got, want := string(msg), "To: to1@example.com,to2@example.com\n"; !strings.Contains(got, want) {
				t.Errorf("expectd %q to contain %q", got, want)
			}
			if got, want := string(msg), "CC: cc1@example.com,cc2@example.com\n"; !strings.Contains(got, want) {
				t.Errorf("expectd %q to contain %q", got, want)
			}
			if got, want := string(msg), "BCC: bcc1@example.com,bcc2@example.com\n"; !strings.Contains(got, want) {
				t.Errorf("expectd %q to contain %q", got, want)
			}
		})
	})
}
