// Copyright 2020 the Exposure Notifications Verification Server authors
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
	"embed"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/leonelquinteros/gotext"
	"golang.org/x/text/language"
)

const (
	// defaultLocale is the default fallback locale when all else fails.
	defaultLocale = "en"
)

type Source int

const (
	DefaultSource = iota
	RedirectSource
)

var dirMap map[Source]string = map[Source]string{
	DefaultSource:  "default",
	RedirectSource: "redirect",
}

//go:embed locales/**/*
var localesFS embed.FS

// LocalesFS returns the file system for the server assets.
func LocalesFS() fs.FS {
	if project.DevMode() {
		return os.DirFS(project.Root("internal", "i18n"))
	}
	return localesFS
}

// LocaleMap is a map of locale names to their data structure.
type LocaleMap struct {
	data    map[string]gotext.Translator
	matcher language.Matcher

	dynamic        map[uint]map[string]gotext.Translator
	dynamicMatcher map[uint]language.Matcher
	dynamicLock    sync.Mutex

	source     Source
	reload     bool
	reloadLock sync.Mutex
}

func TranslatorLanguage(l gotext.Translator) string {
	typ, ok := l.(*gotext.Po)
	if !ok {
		return ""
	}
	return typ.Language
}

// poHeader is used when constructing new po file buffers in memory.
const poHeader = `msgid ""
msgstr ""
"Language: %s\n"
"MIME-Version: 1.0\n"
"Content-Type: text/plain; charset=UTF-8\n"
"Content-Transfer-Encoding: 8bit\n"
"Plural-Forms: nplurals=2; plural=(n != 1);\n"

`

// SetDynamicTranslations creates realm specific locals with translations
// based on what's in the database.
func (l *LocaleMap) SetDynamicTranslations(incoming []*database.DynamicTranslation) {
	poFiles := make(map[uint]map[string]string)

	// Convert incoming tranlations into PO files, grouped by realm, Language
	for _, dt := range incoming {
		realm, ok := poFiles[dt.RealmID]
		if !ok {
			realm = make(map[string]string)
			poFiles[dt.RealmID] = realm
		}

		locale := strings.ToLower(dt.Locale)
		curFile, ok := realm[locale]
		if !ok {
			// build a new file with the po header fields.
			curFile = fmt.Sprintf(poHeader, locale)
		}

		addOn := fmt.Sprintf("msgid \"%s\"\nmsgstr \"%s\"\n\n", dt.MessageID, dt.Message)
		curFile = curFile + addOn

		realm[locale] = curFile
	}

	// Parse the completed files into gotext.Translator
	next := make(map[uint]map[string]gotext.Translator, len(poFiles))
	nextMatcher := make(map[uint]language.Matcher, len(poFiles))
	for realmID, realmLocales := range poFiles {
		parsed := make(map[string]gotext.Translator, len(realmLocales))

		names := make([]language.Tag, 0, len(realmLocales))
		for locale, poContent := range realmLocales {
			translator := gotext.NewPo()
			translator.Parse([]byte(poContent))
			parsed[locale] = translator

			names = append(names, language.Make(locale))
		}

		next[realmID] = parsed
		nextMatcher[realmID] = language.NewMatcher(names)
	}

	// Under a lock, replace the current translation map.
	l.dynamicLock.Lock()
	defer l.dynamicLock.Unlock()
	l.dynamic = next
	l.dynamicMatcher = nextMatcher
}

// LookupDynamic finds the best locale for the given ids.
func (l *LocaleMap) LookupDynamic(realmID uint, realmDefaultLocale string, ids ...string) gotext.Translator {
	// Pull a realm's translations out of the map under a lock to avoid data races.
	l.dynamicLock.Lock()
	data := l.dynamic[realmID]
	matcher := l.dynamicMatcher[realmID]
	l.dynamicLock.Unlock()

	for _, id := range ids {
		// Convert the id to the "canonical" form.
		canonical, err := l.Canonicalize(id, matcher)
		if err != nil {
			continue
		}
		locale, ok := data[canonical]
		if !ok {
			continue
		}
		return locale
	}

	if def, ok := data[realmDefaultLocale]; ok {
		return def
	}
	return gotext.NewPo()
}

// Lookup finds the best locale for the given ids. If none exists, the default
// locale is used.
//
// If reloading is enabled, the locales are reloaded before lookup. If reloading
// fails, it panics. For this reason, you should not enable reloading in
// production.
func (l *LocaleMap) Lookup(ids ...string) gotext.Translator {
	if l.reload {
		l.reloadLock.Lock()
		defer l.reloadLock.Unlock()

		if err := l.load(); err != nil {
			panic(err)
		}
	}

	for _, id := range ids {
		// Convert the id to the "canonical" form.
		canonical, err := l.Canonicalize(id, l.matcher)
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
func (l *LocaleMap) Canonicalize(id string, matcher language.Matcher) (result string, retErr error) {
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
	if tag, _, conf := matcher.Match(desired...); conf != language.No {
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
	fsys := LocalesFS()

	subDir, ok := dirMap[l.source]
	if !ok {
		return fmt.Errorf("no translation source specified")
	}

	entries, err := fs.ReadDir(fsys, fmt.Sprintf("locales/%s", subDir))
	if err != nil {
		return fmt.Errorf("failed to load locales: %w", err)
	}

	log.Printf("locales: %v  dir: %v entries: %v", l.source, subDir, entries)

	data := make(map[string]gotext.Translator, len(entries))
	names := make([]language.Tag, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		names = append(names, language.Make(name))

		b, err := fs.ReadFile(fsys, path.Join("locales", subDir, name, "default.po"))
		if err != nil {
			return fmt.Errorf("failed to read %q: %w", name, err)
		}

		po := gotext.NewPo()
		po.Parse(b)
		data[name] = po
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

// WithDefaultSource loads the default translations
// for the main Web UI (server).
func WithDefaultSource() Option {
	return func(l *LocaleMap) *LocaleMap {
		l.source = DefaultSource
		return l
	}
}

// WithRedirectSource loads the translations
// for the redirect server.
func WithRedirectSource() Option {
	return func(l *LocaleMap) *LocaleMap {
		l.source = RedirectSource
		return l
	}
}

// Load parses and loads the localization files from disk. It builds the locale
// matcher based on the currently available data (organized by folder).
//
// Due to the heavy I/O, callers should cache the resulting value and only call
// Load when data needs to be refreshed.
func Load(opts ...Option) (*LocaleMap, error) {
	l := &LocaleMap{}

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
