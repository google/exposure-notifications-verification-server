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
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/browser"
	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/i18n"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/admin"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"

	"github.com/chromedp/chromedp"
)

func TestHandleRealmsIndex(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServer(t, testDatabaseInstance)

	locales, err := i18n.Load(harness.Config.LocalesPath)
	if err != nil {
		t.Fatal(err)
	}

	middlewares := []mux.MiddlewareFunc{
		middleware.InjectCurrentPath(),
		middleware.ProcessLocale(locales),
	}

	h, err := render.New(ctx, envstest.ServerAssetsPath(), true)
	if err != nil {
		t.Fatal(err)
	}

	c := admin.New(harness.Config, harness.Cacher, harness.Database, harness.AuthProvider, harness.RateLimiter, h)

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/{id}", c.HandleRealmsIndex())
		mux.Handle("/", c.HandleRealmsIndex())

		envstest.ExerciseUserMissing(t, mux)
		envstest.ExerciseBadPagination(t, &database.Membership{
			User: &database.User{},
		}, mux)
	})

	t.Run("failure", func(t *testing.T) {
		t.Parallel()

		harness := envstest.NewServerConfig(t, testDatabaseInstance)
		harness.Database.SetRawDB(envstest.NewFailingDatabase())

		c := admin.New(harness.Config, harness.Cacher, harness.Database, harness.AuthProvider, harness.RateLimiter, h)

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/", c.HandleRealmsIndex()).Methods("GET")

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		r := httptest.NewRequest("GET", "/", nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "text/html")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 500; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("lists_all", func(t *testing.T) {
		t.Parallel()

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/", c.HandleRealmsIndex()).Methods("GET")

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		r := httptest.NewRequest("GET", "/", nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "text/html")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 200; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("renders", func(t *testing.T) {
		t.Parallel()

		_, _, session, err := harness.ProvisionAndLogin()
		if err != nil {
			t.Fatal(err)
		}

		cookie, err := harness.SessionCookie(session)
		if err != nil {
			t.Fatal(err)
		}

		browserCtx := browser.New(t)
		taskCtx, done := context.WithTimeout(browserCtx, 120*time.Second)
		defer done()

		if err := chromedp.Run(taskCtx,
			browser.SetCookie(cookie),

			chromedp.Navigate(`http://`+harness.Server.Addr()+`/admin/realms`),
			chromedp.WaitVisible(`body#admin-realms-index`, chromedp.ByQuery),
		); err != nil {
			t.Fatal(err)
		}
	})
}

func TestHandleRealmsCreate(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServer(t, testDatabaseInstance)

	user, err := harness.Database.FindUser(1)
	if err != nil {
		t.Error(err)
	}

	locales, err := i18n.Load(harness.Config.LocalesPath)
	if err != nil {
		t.Fatal(err)
	}

	middlewares := []mux.MiddlewareFunc{
		middleware.InjectCurrentPath(),
		middleware.ProcessLocale(locales),
	}

	h, err := render.New(ctx, envstest.ServerAssetsPath(), true)
	if err != nil {
		t.Fatal(err)
	}

	c := admin.New(harness.Config, harness.Cacher, harness.Database, harness.AuthProvider, harness.RateLimiter, h)

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/{id}", c.HandleRealmsCreate())
		mux.Handle("/", c.HandleRealmsCreate())

		envstest.ExerciseSessionMissing(t, mux)
		envstest.ExerciseUserMissing(t, mux)
	})

	t.Run("failure", func(t *testing.T) {
		t.Parallel()

		harness := envstest.NewServerConfig(t, testDatabaseInstance)
		harness.Database.SetRawDB(envstest.NewFailingDatabase())

		c := admin.New(harness.Config, harness.Cacher, harness.Database, harness.AuthProvider, harness.RateLimiter, h)

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/", c.HandleRealmsCreate()).Methods("POST")

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, user)

		r := httptest.NewRequest("POST", "/", nil)
		r = r.Clone(ctx)
		r.Header.Set("Accept", "text/html")
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 500; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("validation", func(t *testing.T) {
		t.Parallel()

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/", c.HandleRealmsCreate()).Methods("POST")

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, user)

		r := httptest.NewRequest("POST", "/", strings.NewReader((&url.Values{
			"name": []string{""},
		}).Encode()))
		r = r.Clone(ctx)
		r.Header.Set("Accept", "text/html")
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 422; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Body.String(), "cannot be blank"; !strings.Contains(got, want) {
			t.Errorf("expected %q to contain %q", got, want)
		}
	})

	t.Run("renders", func(t *testing.T) {
		t.Parallel()

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/", c.HandleRealmsCreate()).Methods("GET")

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, user)

		r := httptest.NewRequest("GET", "/", nil)
		r = r.Clone(ctx)
		r.Header.Set("Accept", "text/html")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 200; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("creates", func(t *testing.T) {
		t.Parallel()

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/", c.HandleRealmsCreate()).Methods("POST")

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, user)

		r := httptest.NewRequest("POST", "/", strings.NewReader((&url.Values{
			"name":                        []string{"realmy"},
			"regionCode":                  []string{"TT-tt"},
			"useRealmCertificateKey":      []string{"1"},
			"certificateIssuer":           []string{"iss"},
			"certificateAudience":         []string{"aud"},
			"can_use_system_sms_config":   []string{"1"},
			"can_use_system_email_config": []string{"1"},
		}).Encode()))
		r = r.Clone(ctx)
		r.Header.Set("Accept", "text/html")
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 303; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Header().Get("Location"), "/admin/realms/2/edit"; got != want {
			t.Errorf("expected %q to be %q", got, want)
		}
	})
}

func TestHandleRealmsUpdate(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServer(t, testDatabaseInstance)

	user, err := harness.Database.FindUser(1)
	if err != nil {
		t.Error(err)
	}

	locales, err := i18n.Load(harness.Config.LocalesPath)
	if err != nil {
		t.Fatal(err)
	}

	middlewares := []mux.MiddlewareFunc{
		middleware.InjectCurrentPath(),
		middleware.ProcessLocale(locales),
	}

	h, err := render.New(ctx, envstest.ServerAssetsPath(), true)
	if err != nil {
		t.Fatal(err)
	}

	c := admin.New(harness.Config, harness.Cacher, harness.Database, harness.AuthProvider, harness.RateLimiter, h)

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/{id}", c.HandleRealmsUpdate())
		mux.Handle("/", c.HandleRealmsUpdate())

		envstest.ExerciseSessionMissing(t, mux)
		envstest.ExerciseUserMissing(t, mux)
	})

	t.Run("failure", func(t *testing.T) {
		t.Parallel()

		harness := envstest.NewServerConfig(t, testDatabaseInstance)
		harness.Database.SetRawDB(envstest.NewFailingDatabase())

		c := admin.New(harness.Config, harness.Cacher, harness.Database, harness.AuthProvider, harness.RateLimiter, h)

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/{id}", c.HandleRealmsUpdate()).Methods("POST")

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, user)

		r := httptest.NewRequest("POST", "/1", nil)
		r = r.Clone(ctx)
		r.Header.Set("Accept", "text/html")
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 500; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("renders", func(t *testing.T) {
		t.Parallel()

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/{id}", c.HandleRealmsUpdate()).Methods("GET")

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, user)

		r := httptest.NewRequest("GET", "/1", nil)
		r = r.Clone(ctx)
		r.Header.Set("Accept", "text/html")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 200; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("updates", func(t *testing.T) {
		t.Parallel()

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/{id}", c.HandleRealmsUpdate()).Methods("POST")

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, user)

		r := httptest.NewRequest("POST", "/1", strings.NewReader((&url.Values{
			"can_use_system_sms_config":   []string{"1"},
			"can_use_system_email_config": []string{"1"},
		}).Encode()))
		r = r.Clone(ctx)
		r.Header.Set("Accept", "text/html")
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 303; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Header().Get("Location"), "/admin/realms/1/edit"; got != want {
			t.Errorf("expected %q to be %q", got, want)
		}
	})
}

func TestHandleRealmsAdd(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServer(t, testDatabaseInstance)

	user, err := harness.Database.FindUser(1)
	if err != nil {
		t.Error(err)
	}

	locales, err := i18n.Load(harness.Config.LocalesPath)
	if err != nil {
		t.Fatal(err)
	}

	middlewares := []mux.MiddlewareFunc{
		middleware.InjectCurrentPath(),
		middleware.ProcessLocale(locales),
	}

	h, err := render.New(ctx, envstest.ServerAssetsPath(), true)
	if err != nil {
		t.Fatal(err)
	}

	c := admin.New(harness.Config, harness.Cacher, harness.Database, harness.AuthProvider, harness.RateLimiter, h)

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/{id}", c.HandleRealmsAdd())
		mux.Handle("/", c.HandleRealmsAdd())

		envstest.ExerciseSessionMissing(t, mux)
		envstest.ExerciseUserMissing(t, mux)
	})

	t.Run("realm_id_not_found", func(t *testing.T) {
		t.Parallel()

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/{realm_id}/{user_id}", c.HandleRealmsAdd()).Methods("POST")

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, user)

		r := httptest.NewRequest("POST", "/12345/1", nil)
		r = r.Clone(ctx)
		r.Header.Set("Accept", "text/html")
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 401; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("user_id_not_found", func(t *testing.T) {
		t.Parallel()

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/{realm_id}/{user_id}", c.HandleRealmsAdd()).Methods("POST")

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, user)

		r := httptest.NewRequest("POST", "/1/12345", nil)
		r = r.Clone(ctx)
		r.Header.Set("Accept", "text/html")
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 401; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("failure", func(t *testing.T) {
		t.Parallel()

		harness := envstest.NewServerConfig(t, testDatabaseInstance)
		harness.Database.SetRawDB(envstest.NewFailingDatabase())

		c := admin.New(harness.Config, harness.Cacher, harness.Database, harness.AuthProvider, harness.RateLimiter, h)

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/{id}", c.HandleRealmsAdd()).Methods("POST")

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, user)

		r := httptest.NewRequest("POST", "/1", nil)
		r = r.Clone(ctx)
		r.Header.Set("Accept", "text/html")
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 500; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("adds", func(t *testing.T) {
		t.Parallel()

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/{realm_id}/{user_id}", c.HandleRealmsAdd()).Methods("POST")

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, user)

		r := httptest.NewRequest("POST", "/1/1", nil)
		r = r.Clone(ctx)
		r.Header.Set("Accept", "text/html")
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Referer", "https://example.com/foo/bar")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 303; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Header().Get("Location"), "https://example.com/foo/bar"; got != want {
			t.Errorf("expected %q to be %q", got, want)
		}

		realm, err := harness.Database.FindRealm(1)
		if err != nil {
			t.Fatal(err)
		}

		memberships, err := realm.MembershipPermissionMap(harness.Database)
		if err != nil {
			t.Fatal(err)
		}
		perms, ok := memberships[1]
		if !ok {
			t.Errorf("expected user to be added to realm")
		}
		if got, want := perms, rbac.LegacyRealmAdmin; !rbac.Can(got, want) {
			t.Errorf("expected %q to be able to %q", got, want)
		}
	})
}

func TestHandleRealmsRemove(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServer(t, testDatabaseInstance)

	user, err := harness.Database.FindUser(1)
	if err != nil {
		t.Error(err)
	}

	locales, err := i18n.Load(harness.Config.LocalesPath)
	if err != nil {
		t.Fatal(err)
	}

	middlewares := []mux.MiddlewareFunc{
		middleware.InjectCurrentPath(),
		middleware.ProcessLocale(locales),
	}

	h, err := render.New(ctx, envstest.ServerAssetsPath(), true)
	if err != nil {
		t.Fatal(err)
	}

	c := admin.New(harness.Config, harness.Cacher, harness.Database, harness.AuthProvider, harness.RateLimiter, h)

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/{id}", c.HandleRealmsRemove())
		mux.Handle("/", c.HandleRealmsRemove())

		envstest.ExerciseSessionMissing(t, mux)
		envstest.ExerciseUserMissing(t, mux)
	})

	t.Run("realm_id_not_found", func(t *testing.T) {
		t.Parallel()

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/{realm_id}/{user_id}", c.HandleRealmsRemove()).Methods("POST")

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, user)

		r := httptest.NewRequest("POST", "/12345/1", nil)
		r = r.Clone(ctx)
		r.Header.Set("Accept", "text/html")
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 401; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("user_id_not_found", func(t *testing.T) {
		t.Parallel()

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/{realm_id}/{user_id}", c.HandleRealmsRemove()).Methods("POST")

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, user)

		r := httptest.NewRequest("POST", "/1/12345", nil)
		r = r.Clone(ctx)
		r.Header.Set("Accept", "text/html")
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 401; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("failure", func(t *testing.T) {
		t.Parallel()

		harness := envstest.NewServerConfig(t, testDatabaseInstance)
		harness.Database.SetRawDB(envstest.NewFailingDatabase())

		c := admin.New(harness.Config, harness.Cacher, harness.Database, harness.AuthProvider, harness.RateLimiter, h)

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/{id}", c.HandleRealmsRemove()).Methods("POST")

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, user)

		r := httptest.NewRequest("POST", "/1", nil)
		r = r.Clone(ctx)
		r.Header.Set("Accept", "text/html")
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 500; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("removes", func(t *testing.T) {
		t.Parallel()

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/{realm_id}/{user_id}", c.HandleRealmsRemove()).Methods("POST")

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, user)

		r := httptest.NewRequest("POST", "/1/1", nil)
		r = r.Clone(ctx)
		r.Header.Set("Accept", "text/html")
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Referer", "https://example.com/foo/bar")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 303; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Header().Get("Location"), "https://example.com/foo/bar"; got != want {
			t.Errorf("expected %q to be %q", got, want)
		}

		realm, err := harness.Database.FindRealm(1)
		if err != nil {
			t.Fatal(err)
		}

		memberships, err := realm.MembershipPermissionMap(harness.Database)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := memberships[1]; ok {
			t.Errorf("expected user to be remove from realm")
		}
	})
}

// func TestShowAdminRealms(t *testing.T) {
// 	t.Parallel()

// 	harness := envstest.NewServer(t, testDatabaseInstance)

// 	realm, _, session, err := harness.ProvisionAndLogin()
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	_ = realm

// 	// Mint a cookie for the session.
// 	cookie, err := harness.SessionCookie(session)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	// Create a browser runner.
// 	browserCtx := browser.New(t)
// 	taskCtx, done := context.WithTimeout(browserCtx, 120*time.Second)
// 	defer done()

// 	wantName := "Test Realm"
// 	wantRegionCode := "us-tst"
// 	certIssuer := "test issuer"
// 	certAudience := "test audience"

// 	// This accepts "are you sure" alert that pops up for "leave realm"
// 	chromedp.ListenTarget(taskCtx, func(ev interface{}) {
// 		if _, ok := ev.(*page.EventJavascriptDialogOpening); ok {
// 			go func() {
// 				if err := chromedp.Run(taskCtx,
// 					page.HandleJavaScriptDialog(true),
// 				); err != nil {
// 					panic(err)
// 				}
// 			}()
// 		}
// 	})

// 	if err := chromedp.Run(taskCtx,
// 		// Pre-authenticate the user.
// 		browser.SetCookie(cookie),

// 		// Visit /admin/realms
// 		chromedp.Navigate(`http://`+harness.Server.Addr()+`/admin/realms`),

// 		// Wait for render.
// 		chromedp.WaitVisible(`body#admin-realms-index`, chromedp.ByQuery),

// 		/* ----- Test New Realm -----  */
// 		chromedp.Click(`a#new`, chromedp.ByQuery),
// 		// Fill out the form.
// 		chromedp.SetValue(`input#name`, wantName, chromedp.ByQuery),
// 		chromedp.SetValue(`input#regionCode`, wantRegionCode, chromedp.ByQuery),
// 		chromedp.SetValue(`input#certificateIssuer`, certIssuer, chromedp.ByQuery),
// 		chromedp.SetValue(`input#certificateAudience`, certAudience, chromedp.ByQuery),
// 		chromedp.Submit(`form#new-form`, chromedp.ByQuery),

// 		/* ----- Test Search -----  */
// 		chromedp.Click(`a#realms`, chromedp.ByQuery),
// 		// Wait for render.
// 		chromedp.WaitVisible(`body#admin-realms-index`, chromedp.ByQuery),

// 		// Fill out the form with a non-existing realm
// 		chromedp.SetValue(`input#search`, "notexists", chromedp.ByQuery),
// 		chromedp.Submit(`form#search-form`, chromedp.ByQuery),

// 		// Assert no realms shown
// 		chromedp.WaitNotPresent(`table#results-table tr`, chromedp.ByQuery),

// 		// Fill out the form by realm name.
// 		chromedp.SetValue(`input#search`, " test realm ", chromedp.ByQuery),
// 		chromedp.Submit(`form#search-form`, chromedp.ByQuery),

// 		// Wait for the search result.
// 		chromedp.WaitVisible(`table#results-table tr`, chromedp.ByQuery),

// 		/* ----- Test Edit Realm -----  */
// 		// Visit the realm from the search
// 		chromedp.Click(`table#results-table tr td a`, chromedp.ByQuery),

// 		// Leave the realm
// 		chromedp.Click(`a#leave`, chromedp.ByQuery),

// 		// Wait for render.
// 		chromedp.WaitVisible(`a#join`, chromedp.ByQuery),
// 	); err != nil {
// 		t.Fatal(err)
// 	}

// 	// Get the newly created realm
// 	newRealm, err := harness.Database.FindRealm(2)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	if newRealm.Name != wantName {
// 		t.Errorf("got: %s, want: %s", newRealm.Name, wantName)
// 	}
// 	wantRegionCode = strings.ToUpper(wantRegionCode) // DB uppercases on save
// 	if newRealm.RegionCode != wantRegionCode {
// 		t.Errorf("got: %s, want: %s", newRealm.RegionCode, wantRegionCode)
// 	}
// 	if newRealm.CertificateIssuer != certIssuer {
// 		t.Errorf("got: %s, want: %s", newRealm.CertificateIssuer, certIssuer)
// 	}
// 	if newRealm.CertificateAudience != certAudience {
// 		t.Errorf("got: %s, want: %s", newRealm.CertificateAudience, certAudience)
// 	}
// }
