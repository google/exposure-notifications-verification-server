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

package realmadmin_test

import (
	"fmt"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/realmadmin"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"github.com/gorilla/sessions"
	"github.com/lib/pq"
)

func TestHandleSettings(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServer(t, testDatabaseInstance)

	realm, user, _, err := harness.ProvisionAndLogin()
	if err != nil {
		t.Fatal(err)
	}
	realm.AbusePreventionEnabled = true

	h, err := render.New(ctx, envstest.ServerAssetsPath(), true)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		c := realmadmin.New(harness.Config, harness.Database, harness.RateLimiter, h)
		handler := c.HandleSettings()

		envstest.ExerciseSessionMissing(t, handler)
		envstest.ExerciseMembershipMissing(t, handler)
		envstest.ExercisePermissionMissing(t, handler)
	})

	t.Run("missing_upsert_permission", func(t *testing.T) {
		t.Parallel()

		c := realmadmin.New(harness.Config, harness.Database, harness.RateLimiter, h)
		handler := c.HandleSettings()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        user,
			Permissions: rbac.SettingsRead,
		})

		r := httptest.NewRequest("PUT", "/", nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "text/html")

		w := httptest.NewRecorder()

		handler.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 401; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Body.String(), "Unauthorized"; !strings.Contains(got, want) {
			t.Errorf("Expected %q to contain %q", got, want)
		}
	})

	t.Run("general", func(t *testing.T) {
		t.Parallel()

		realm := database.NewRealmWithDefaults("general")
		if err := harness.Database.SaveRealm(realm, database.SystemTest); err != nil {
			t.Fatal(err)
		}

		c := realmadmin.New(harness.Config, harness.Database, harness.RateLimiter, h)
		handler := c.HandleSettings()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        user,
			Permissions: rbac.SettingsRead | rbac.SettingsWrite,
		})

		r := httptest.NewRequest("PUT", "/", strings.NewReader(url.Values{
			"general":         []string{"1"},
			"name":            []string{"new-realmy"},
			"region_code":     []string{"TT2"},
			"welcome_message": []string{"hello there"},
		}.Encode()))
		r = r.Clone(ctx)
		r.Header.Set("Accept", "text/html")
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()

		handler.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 303; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}

		realm, err := harness.Database.FindRealm(realm.ID)
		if err != nil {
			t.Fatal(err)
		}

		if got, want := realm.Name, "new-realmy"; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
		if got, want := realm.RegionCode, "TT2"; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
		if got, want := realm.WelcomeMessage, "hello there"; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
	})

	t.Run("codes", func(t *testing.T) {
		t.Parallel()

		realm := database.NewRealmWithDefaults("codes")
		realm.EnableENExpress = false
		if err := harness.Database.SaveRealm(realm, database.SystemTest); err != nil {
			t.Fatal(err)
		}

		c := realmadmin.New(harness.Config, harness.Database, harness.RateLimiter, h)
		handler := c.HandleSettings()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        user,
			Permissions: rbac.SettingsRead | rbac.SettingsWrite,
		})

		r := httptest.NewRequest("PUT", "/", strings.NewReader(url.Values{
			"codes":              []string{"1"},
			"allowed_test_types": []string{"2"},
			"require_date":       []string{"1"},
			"allow_bulk":         []string{"1"},
			"code_length":        []string{"7"},
			"code_duration":      []string{"60"},
			"long_code_length":   []string{"22"},
			"long_code_duration": []string{"24"},
		}.Encode()))
		r = r.Clone(ctx)
		r.Header.Set("Accept", "text/html")
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()

		handler.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 303; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}

		realm, err := harness.Database.FindRealm(realm.ID)
		if err != nil {
			t.Fatal(err)
		}

		if got, want := realm.AllowedTestTypes, database.TestTypeConfirmed; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
		if got, want := realm.RequireDate, true; got != want {
			t.Errorf("expected %t to be %t", got, want)
		}
		if got, want := realm.AllowBulkUpload, true; got != want {
			t.Errorf("expected %t to be %t", got, want)
		}
		if got, want := realm.CodeLength, uint(7); got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := realm.CodeDuration.Duration, 60*time.Minute; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
		if got, want := realm.LongCodeLength, uint(22); got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := realm.LongCodeDuration.Duration, 24*time.Hour; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
	})

	t.Run("security", func(t *testing.T) {
		t.Parallel()

		realm := database.NewRealmWithDefaults("security")
		realm.EnableENExpress = false
		if err := harness.Database.SaveRealm(realm, database.SystemTest); err != nil {
			t.Fatal(err)
		}

		c := realmadmin.New(harness.Config, harness.Database, harness.RateLimiter, h)
		handler := c.HandleSettings()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        user,
			Permissions: rbac.SettingsRead | rbac.SettingsWrite,
		})

		r := httptest.NewRequest("PUT", "/", strings.NewReader(url.Values{
			"security":                       []string{"1"},
			"email_verified_mode":            []string{"2"},
			"mfa_mode":                       []string{"2"},
			"mfa_grace_period":               []string{"1"},
			"password_rotation_period_days":  []string{"7"},
			"password_rotation_warning_days": []string{"3"},
			"allowed_cidrs_adminapi":         []string{"0.0.0.0/0\n1.1.1.1/0"},
			"allowed_cidrs_apiserver":        []string{"0.0.0.0/0\n2.2.2.2/0"},
			"allowed_cidrs_server":           []string{"0.0.0.0/0\n3.3.3.3/0"},
		}.Encode()))
		r = r.Clone(ctx)
		r.Header.Set("Accept", "text/html")
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()

		handler.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 303; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}

		realm, err := harness.Database.FindRealm(realm.ID)
		if err != nil {
			t.Fatal(err)
		}

		if got, want := realm.EmailVerifiedMode, database.MFAOptional; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
		if got, want := realm.MFAMode, database.MFAOptional; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
		if got, want := realm.MFARequiredGracePeriod.Duration, 24*time.Hour; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
		if got, want := realm.PasswordRotationPeriodDays, uint(7); got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
		if got, want := realm.PasswordRotationWarningDays, uint(3); got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
		if got, want := realm.AllowedCIDRsAdminAPI, pq.StringArray([]string{"0.0.0.0/0", "1.1.1.1/0"}); !reflect.DeepEqual(got, want) {
			t.Errorf("Expected %q to be %q", got, want)
		}
		if got, want := realm.AllowedCIDRsAPIServer, pq.StringArray([]string{"0.0.0.0/0", "2.2.2.2/0"}); !reflect.DeepEqual(got, want) {
			t.Errorf("Expected %q to be %q", got, want)
		}
		if got, want := realm.AllowedCIDRsServer, pq.StringArray([]string{"0.0.0.0/0", "3.3.3.3/0"}); !reflect.DeepEqual(got, want) {
			t.Errorf("Expected %q to be %q", got, want)
		}
	})

	t.Run("security/bad_cidrs", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			field  string
			column string
		}{
			{"allowed_cidrs_adminapi", "allowedCIDRsAdminAPI"},
			{"allowed_cidrs_apiserver", "allowedCIDRsAPIServer"},
			{"allowed_cidrs_server", "allowedCIDRsServer"},
		}

		for _, tc := range cases {
			tc := tc

			t.Run(tc.field, func(t *testing.T) {
				t.Parallel()

				realm := database.NewRealmWithDefaults(fmt.Sprintf("security_bad_cidrs_%s", tc.field))

				c := realmadmin.New(harness.Config, harness.Database, harness.RateLimiter, h)
				handler := c.HandleSettings()

				ctx := ctx
				ctx = controller.WithSession(ctx, &sessions.Session{})
				ctx = controller.WithMembership(ctx, &database.Membership{
					Realm:       realm,
					User:        user,
					Permissions: rbac.SettingsRead | rbac.SettingsWrite,
				})

				r := httptest.NewRequest("PUT", "/", strings.NewReader(url.Values{
					"security": []string{"1"},
					tc.field:   []string{"bad"},
				}.Encode()))
				r = r.Clone(ctx)
				r.Header.Set("Accept", "text/html")
				r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

				w := httptest.NewRecorder()

				handler.ServeHTTP(w, r)
				w.Flush()

				if got, want := w.Code, 422; got != want {
					t.Errorf("Expected %d to be %d", got, want)
				}

				errs := realm.ErrorsFor(tc.column)
				if got, want := len(errs), 1; got != want {
					t.Fatalf("expected %d error, got %d", want, got)
				}

				if got, want := errs[0], "invalid CIDR address"; !strings.Contains(got, want) {
					t.Errorf("Expected %q to contain %q", got, want)
				}
			})
		}
	})

	t.Run("abuse_prevention", func(t *testing.T) {
		t.Parallel()

		realm := database.NewRealmWithDefaults("abuse_prevention")
		realm.EnableENExpress = false
		if err := harness.Database.SaveRealm(realm, database.SystemTest); err != nil {
			t.Fatal(err)
		}

		c := realmadmin.New(harness.Config, harness.Database, harness.RateLimiter, h)
		handler := c.HandleSettings()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        user,
			Permissions: rbac.SettingsRead | rbac.SettingsWrite,
		})

		r := httptest.NewRequest("PUT", "/", strings.NewReader(url.Values{
			"abuse_prevention":              []string{"1"},
			"abuse_prevention_enabled":      []string{"1"},
			"abuse_prevention_limit_factor": []string{"10.5"},
			"abuse_prevention_burst":        []string{"100"},
		}.Encode()))
		r = r.Clone(ctx)
		r.Header.Set("Accept", "text/html")
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()

		handler.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 303; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}

		realm, err := harness.Database.FindRealm(realm.ID)
		if err != nil {
			t.Fatal(err)
		}

		if got, want := realm.AbusePreventionEnabled, true; got != want {
			t.Errorf("expected %t to be %t", got, want)
		}
		if got, want := realm.AbusePreventionLimitFactor, float32(10.5); got != want {
			t.Errorf("expected %v to be %v", got, want)
		}
	})

	t.Run("sms", func(t *testing.T) {
		t.Parallel()

		realm := database.NewRealmWithDefaults("sms")
		realm.EnableENExpress = false
		if err := harness.Database.SaveRealm(realm, database.SystemTest); err != nil {
			t.Fatal(err)
		}

		c := realmadmin.New(harness.Config, harness.Database, harness.RateLimiter, h)
		handler := c.HandleSettings()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        user,
			Permissions: rbac.SettingsRead | rbac.SettingsWrite,
		})

		r := httptest.NewRequest("PUT", "/", strings.NewReader(url.Values{
			"sms":                 []string{"1"},
			"sms_text_label_0":    []string{"Default SMS template"},
			"sms_text_template_0": []string{"[longcode]"},
		}.Encode()))
		r = r.Clone(ctx)
		r.Header.Set("Accept", "text/html")
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()

		handler.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 303; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}

		realm, err := harness.Database.FindRealm(realm.ID)
		if err != nil {
			t.Fatal(err)
		}

		if got, want := realm.SMSTextTemplate, "[longcode]"; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
	})

	t.Run("sms/validation_error", func(t *testing.T) {
		t.Parallel()

		realm := database.NewRealmWithDefaults("sms_validation")
		realm.EnableENExpress = false
		if err := harness.Database.SaveRealm(realm, database.SystemTest); err != nil {
			t.Fatal(err)
		}

		c := realmadmin.New(harness.Config, harness.Database, harness.RateLimiter, h)
		handler := c.HandleSettings()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        user,
			Permissions: rbac.SettingsRead | rbac.SettingsWrite,
		})

		r := httptest.NewRequest("PUT", "/", strings.NewReader(url.Values{
			"sms":                 []string{"1"},
			"twilio_account_sid":  []string{"abcd1234"},
			"sms_text_label_0":    []string{"Default SMS template"},
			"sms_text_template_0": []string{"[longcode]"},
		}.Encode()))
		r = r.Clone(ctx)
		r.Header.Set("Accept", "text/html")
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()

		handler.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 422; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Body.String(), "all must be specified or all must be blank"; !strings.Contains(got, want) {
			t.Errorf("expected %s to include %s", got, want)
		}
	})

	t.Run("email/validation_error", func(t *testing.T) {
		t.Parallel()

		realm := database.NewRealmWithDefaults("email_validation")
		realm.EnableENExpress = false
		if err := harness.Database.SaveRealm(realm, database.SystemTest); err != nil {
			t.Fatal(err)
		}

		c := realmadmin.New(harness.Config, harness.Database, harness.RateLimiter, h)
		handler := c.HandleSettings()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        user,
			Permissions: rbac.SettingsRead | rbac.SettingsWrite,
		})

		r := httptest.NewRequest("PUT", "/", strings.NewReader(url.Values{
			"email":        []string{"1"},
			"smtp_account": []string{"abcd1234"},
		}.Encode()))
		r = r.Clone(ctx)
		r.Header.Set("Accept", "text/html")
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()

		handler.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 422; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Body.String(), "all must be specified or all must be blank"; !strings.Contains(got, want) {
			t.Errorf("expected %s to include %s", got, want)
		}
	})

	t.Run("validation", func(t *testing.T) {
		t.Parallel()

		c := realmadmin.New(harness.Config, harness.Database, harness.RateLimiter, h)
		handler := c.HandleSettings()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        user,
			Permissions: rbac.SettingsRead | rbac.SettingsWrite,
		})

		r := httptest.NewRequest("PUT", "/", strings.NewReader(url.Values{
			"codes":       []string{"1"},
			"code_length": []string{"2"},
		}.Encode()))
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "text/html")
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()

		handler.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 422; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}

		errs := realm.ErrorsFor("codeLength")
		if got, want := len(errs), 1; got != want {
			t.Fatalf("expected %d error, got %d", want, got)
		}

		if got, want := errs[0], "must be at least 6"; !strings.Contains(got, want) {
			t.Errorf("Expected %q to contain %q", got, want)
		}
	})

	t.Run("internal_error", func(t *testing.T) {
		t.Parallel()

		harness := envstest.NewServerConfig(t, testDatabaseInstance)
		harness.Database.SetRawDB(envstest.NewFailingDatabase())

		c := realmadmin.New(harness.Config, harness.Database, harness.RateLimiter, h)
		handler := c.HandleSettings()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        user,
			Permissions: rbac.SettingsRead | rbac.SettingsWrite,
		})

		r := httptest.NewRequest("PUT", "/", nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "text/html")

		w := httptest.NewRecorder()

		handler.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 500; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Body.String(), "Internal server error"; !strings.Contains(got, want) {
			t.Errorf("Expected %q to contain %q", got, want)
		}
	})
}
