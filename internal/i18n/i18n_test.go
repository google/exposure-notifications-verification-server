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

package i18n

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/project"

	"github.com/leonelquinteros/gotext"
)

var localesPath = filepath.Join(project.Root(), "internal", "i18n", "locales")

// TestI18n_matching constructs the superset of all i18n strings and then
// ensures all translation files define said strings.
func TestI18n_matching(t *testing.T) {
	t.Parallel()

	var pos []*gotext.Po
	localesDir := filepath.Join(project.Root(), "internal", "i18n", "locales")
	if err := filepath.Walk(localesDir, func(pth string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if filepath.Ext(info.Name()) != ".po" {
			return nil
		}

		po := new(gotext.Po)
		po.ParseFile(pth)
		pos = append(pos, po)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// This will almost certainly come back to bite me, but the only way to access
	// the actual "list" of translations is to access a private field with
	// reflect. Please don't try this at home kids.
	translations := make(map[string]struct{})
	translationsByLocale := make(map[string]map[string]struct{})
	for _, po := range pos {
		keys := reflect.ValueOf(po).Elem().FieldByName("translations").MapKeys()
		for _, v := range keys {
			if s := v.String(); s != "" {
				translations[s] = struct{}{}

				if translationsByLocale[po.Language] == nil {
					translationsByLocale[po.Language] = make(map[string]struct{})
				}
				translationsByLocale[po.Language][s] = struct{}{}
			}
		}
	}

	for k := range translations {
		for loc, existing := range translationsByLocale {
			if _, ok := existing[k]; !ok {
				t.Errorf("locale %q is missing translation %q", loc, k)
			}
		}
	}
}

func TestLocaleMap_Lookup(t *testing.T) {
	t.Parallel()

	langOf := func(l gotext.Translator) string {
		typ, ok := l.(*gotext.Po)
		if !ok {
			t.Fatalf("%T is not *gotext.Po", l)
		}
		return typ.Language
	}

	localeMap, err := Load(WithReloading(true))
	if err != nil {
		t.Fatal(err)
	}

	t.Run("found", func(t *testing.T) {
		t.Parallel()

		name := langOf(localeMap.Lookup("es"))
		if got, want := name, "es"; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		t.Parallel()

		name := langOf(localeMap.Lookup("totes_not_a_real_language"))
		if got, want := name, "en"; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
	})
}

func TestLocaleMap_Canonicalize(t *testing.T) {
	t.Parallel()

	localeMap, err := Load(WithReloading(true))
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		in  string
		exp string
		err bool
	}{
		{
			in:  "zzzzzzzzz",
			err: true,
		},
		{
			in:  "us",
			err: true,
		},
		{
			in:  "en-us",
			exp: "en",
		},
		{
			in:  "en-US",
			exp: "en",
		},
		{
			in:  "en-US, de",
			exp: "en",
		},
		{
			in:  "en-zz",
			err: true,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()

			result, err := localeMap.Canonicalize(tc.in)
			if (err != nil) != tc.err {
				t.Fatal(err)
			}

			if got, want := result, tc.exp; got != want {
				t.Errorf("Expected %q to be %q", got, want)
			}
		})
	}
}
