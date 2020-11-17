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
	"io/ioutil"

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

	path   string
	reload bool
}

// Lookup finds the best locale for the given ids. If none exists, the default
// locale is used.
//
// If reloading is enabled, the locales are reloaded before lookup. If reloading
// fails, it panics. For this reason, you should not enable reloading in
// production.
func (l *LocaleMap) Lookup(ids ...string) *gotext.Locale {
	if l.reload {
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
func (l *LocaleMap) Canonicalize(id string) (string, error) {
	desired, _, err := language.ParseAcceptLanguage(id)
	if err != nil {
		return "", err
	}
	if tag, _, conf := l.matcher.Match(desired...); conf != language.No {
		raw, _, _ := tag.Raw()
		return raw.String(), nil
	}
	return "", fmt.Errorf("unknown language %q", id)
}

// load loads the locales into the LocaleMap.
func (l *LocaleMap) load() error {
	entries, err := ioutil.ReadDir(l.path)
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

	if err := l.load(); err != nil {
		return nil, err
	}
	return l, nil
}
