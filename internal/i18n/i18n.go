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

// Package i18n defines internationalization and localization.
package i18n

import (
	"fmt"
	"os"
	"sync"

	"github.com/leonelquinteros/gotext"
	"golang.org/x/text/language"
)

const (
	// defaultLocale is the default fallback locale when all else fails.
	defaultLocale = "en"

	// defaultDomain is the domain to load.
	defaultDomain = "default"
)

// LocaleMap is a map of locale names to their data structure.
type LocaleMap struct {
	data    map[string]*gotext.Locale
	matcher language.Matcher

	path       string
	reload     bool
	reloadLock sync.Mutex
}

// Lookup finds the best locale for the given ids. If none exists, the default
// locale is used.
//
// If reloading is enabled, the locales are reloaded before lookup. If reloading
// fails, it panics. For this reason, you should not enable reloading in
// production.
func (l *LocaleMap) Lookup(ids ...string) *gotext.Locale {
	if l.reload {
		l.reloadLock.Lock()
		defer l.reloadLock.Unlock()

		if err := l.load(); err != nil {
			panic(err)
		}
	}

	for _, id := range ids {
		// Convert the id to the "canonical" form.
		canonical, err := l.Canonicalize(id)
		if err != nil {
			continue
		}
		locale, ok := l.data[canonical]
		if !ok {
			continue
		}
		return locale
	}

	return l.data[defaultLocale]
}

// Canonicalize converts the given ID to the expected name.
func (l *LocaleMap) Canonicalize(id string) (result string, retErr error) {
	// go/text panics when given an invalid language. These are often supplied by
	// users or browsers: https://github.com/golang/text/pull/17
	defer func() {
		if r := recover(); r != nil {
			retErr = fmt.Errorf("unknown language %q", id)
			return
		}
	}()

	desired, _, err := language.ParseAcceptLanguage(id)
	if err != nil {
		retErr = err
		return
	}
	if tag, _, conf := l.matcher.Match(desired...); conf != language.No {
		raw, _, _ := tag.Raw()
		result = raw.String()
		return
	}

	retErr = fmt.Errorf("malformed language %q", id)
	return
}

// load loads the locales into the LocaleMap. Callers must take out a mutex
// before calling.
func (l *LocaleMap) load() error {
	entries, err := os.ReadDir(l.path)
	if err != nil {
		return fmt.Errorf("failed to load locales: %w", err)
	}

	data := make(map[string]*gotext.Locale, len(entries))
	names := make([]language.Tag, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		names = append(names, language.Make(name))

		locale := gotext.NewLocale(l.path, name)
		locale.AddDomain(defaultDomain)
		data[name] = locale
	}

	l.data = data
	l.matcher = language.NewMatcher(names)
	return nil
}

// Option is an option to creating a locale map.
type Option func(*LocaleMap) *LocaleMap

// WithReloading enables hot reloading for the map.
func WithReloading(v bool) Option {
	return func(l *LocaleMap) *LocaleMap {
		l.reload = v
		return l
	}
}

// Load parses and loads the localization files from disk. It builds the locale
// matcher based on the currently available data (organized by folder).
//
// Due to the heavy I/O, callers should cache the resulting value and only call
// Load when data needs to be refreshed.
func Load(pth string, opts ...Option) (*LocaleMap, error) {
	l := &LocaleMap{
		path: pth,
	}

	for _, opt := range opts {
		l = opt(l)
	}

	l.reloadLock.Lock()
	defer l.reloadLock.Unlock()

	if err := l.load(); err != nil {
		return nil, err
	}
	return l, nil
}
