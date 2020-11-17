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
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/leonelquinteros/gotext"
	"golang.org/x/text/language"

	"github.com/google/exposure-notifications-verification-server/internal/project"
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
}

// Lookup finds the best locale for the given ids. If none exists, the default
// locale is used.
func (l *LocaleMap) Lookup(ids ...string) *gotext.Locale {
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

// Load parses and loads the localization files from disk. It builds the locale
// matcher based on the currently available data (organized by folder).
//
// Due to the heavy I/O, callers should cache the resulting value and only call
// Load when data needs to be refreshed.
func Load(ctx context.Context) (*LocaleMap, error) {
	localesDir := filepath.Join(project.Root(), "internal", "i18n", "locales")

	entries, err := ioutil.ReadDir(localesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load locales: %w", err)
	}

	data := make(map[string]*gotext.Locale, len(entries))
	names := make([]language.Tag, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		names = append(names, language.Make(name))

		locale := gotext.NewLocale(localesDir, name)
		locale.AddDomain(defaultDomain)
		data[name] = locale
	}

	return &LocaleMap{
		data:    data,
		matcher: language.NewMatcher(names),
	}, nil
}
