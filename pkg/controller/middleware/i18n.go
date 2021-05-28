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

package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/internal/i18n"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"golang.org/x/net/context"

	"github.com/gorilla/mux"
)

const (
	HeaderAcceptLanguage = "Accept-Language"
	QueryKeyLanguage     = "lang"

	LeftAlign  = "ltr"
	RightAlign = "rtl"
)

var rightAlignLanguages = map[string]struct{}{
	"ar": {},
}

// translationReloader maintains a pointer to the LocaleMap
// and will manage refreshing translations from the database
// on the configured interval.
type translationReloader struct {
	locales    *i18n.LocaleMap
	db         *database.Database
	cacher     cache.Cacher
	lastUpdate time.Time
	period     time.Duration
	mu         sync.RWMutex
}

// reload will see if we are due for a refresh, otherwise exit quickly.
func (tr *translationReloader) reload(ctx context.Context) error {
	// Check and see if we need to reload.
	now := time.Now().UTC()
	tr.mu.RLock()
	if tr.lastUpdate.Add(tr.period).After(now) {
		tr.mu.RUnlock()
		return nil
	}
	tr.mu.RUnlock()

	// Needs refresh.
	tr.mu.Lock()
	defer tr.mu.Unlock()

	// This thread lost the race to actually do the refresh, shucks.
	if tr.lastUpdate.Add(tr.period).After(now) {
		return nil
	}

	logger := logging.FromContext(ctx)

	// Read the translations.
	translations, err := tr.db.ListDynamicTranslationsCached(ctx, tr.cacher)
	if err != nil {
		logger.Errorw("unable to read dynamic_translations", "error", err)
		return fmt.Errorf("unable to load realm translations: %w", err)
	}

	tr.locales.SetDynamicTranslations(translations)
	tr.lastUpdate = now
	return nil
}

func LoadDynamicTranslations(locales *i18n.LocaleMap, db *database.Database, cacher cache.Cacher, period time.Duration) (mux.MiddlewareFunc, error) {
	state := &translationReloader{
		locales: locales,
		db:      db,
		cacher:  cacher,
		period:  period,
		// default lastUpdate to time.Zero
	}
	ctx := context.Background()
	if err := state.reload(ctx); err != nil {
		return nil, err
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := state.reload(r.Context()); err != nil {
				logging.FromContext(r.Context()).Errorw("failed to refresh translations, could be stale", "error", err)
			}
			next.ServeHTTP(w, r)
		})
	}, nil
}

// ProcessLocale extracts the locale from the various possible locations and
// sets the template translator to the correct language.
//
// This must be called after the template map has been created.
func ProcessLocale(locales *i18n.LocaleMap) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			param := r.URL.Query().Get(QueryKeyLanguage)
			header := r.Header.Get(HeaderAcceptLanguage)

			// Find the "best" language from the given parameters. They are in
			// priority order.
			m := controller.TemplateMapFromContext(ctx)
			locale := locales.Lookup(param, header)
			m["locale"] = locale
			m["acceptLanguage"] = []string{param, header}

			ctx = controller.WithLocale(ctx, locale)

			// by default, no CSS is needed for left aligned languages.
			textLanguage := i18n.TranslatorLanguage(locale)
			textDirection := LeftAlign
			if _, ok := rightAlignLanguages[textLanguage]; ok {
				textDirection = RightAlign
			}
			m["textDirection"] = textDirection
			m["textLanguage"] = textLanguage

			// Save the template map on the context.
			ctx = controller.WithTemplateMap(ctx, m)
			r = r.Clone(ctx)

			next.ServeHTTP(w, r)
		})
	}
}
