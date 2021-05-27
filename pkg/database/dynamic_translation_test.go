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

package database

import (
	"strings"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/appsync"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestDynamicTranslations(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)
	realm, err := db.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}

	opts := []cmp.Option{
		cmpopts.IgnoreFields(DynamicTranslation{}, "ID", "CreatedAt", "UpdatedAt"),
		cmpopts.IgnoreUnexported(Errorable{}),
		cmpopts.SortSlices(
			func(a, b *DynamicTranslation) bool {
				return strings.Compare(a.Key(), b.Key()) < 0
			}),
	}

	{ // Add some initial translations
		localizations := []appsync.Localization{
			{
				MessageID: "greeting",
				Translations: []appsync.Translation{
					{
						Language: "en_US",
						Message:  "Hello, world!",
					},
					{
						Language: "es_US",
						Message:  "Hola",
					},
				},
			},
			{
				MessageID: "direction",
				Translations: []appsync.Translation{
					{
						Language: "en_US",
						Message:  "on the right",
					},
					{
						Language: "es_US",
						Message:  "a la derecha",
					},
				},
			},
		}

		syncResult, err := db.SyncRealmTranslations(realm.ID, localizations)
		if err != nil {
			t.Fatal(err)
		}

		wantSyncResult := TranslatcionSyncResult{
			Added:   4,
			Updated: 0,
			Deleted: 0,
		}
		if diff := cmp.Diff(wantSyncResult, *syncResult); diff != "" {
			t.Fatalf("mismatch (-want, +got):\n%s", diff)
		}

		got, err := db.LoadDynamicTranslations()
		if err != nil {
			t.Fatal(err)
		}

		want := []*DynamicTranslation{
			{
				RealmID:   realm.ID,
				MessageID: "greeting",
				Locale:    "en",
				Message:   "Hello, world!",
			},
			{
				RealmID:   realm.ID,
				MessageID: "greeting",
				Locale:    "es",
				Message:   "Hola",
			},
			{
				RealmID:   realm.ID,
				MessageID: "direction",
				Locale:    "en",
				Message:   "on the right",
			},
			{
				RealmID:   realm.ID,
				MessageID: "direction",
				Locale:    "es",
				Message:   "a la derecha",
			},
		}

		if diff := cmp.Diff(want, got, opts...); diff != "" {
			t.Fatalf("mismatch (-want, +got):\n%s", diff)
		}
	}

	{ // Add, modify, and delete
		localizations := []appsync.Localization{
			{
				MessageID: "greeting",
				Translations: []appsync.Translation{
					{
						Language: "en_US",
						Message:  "Hello",
					},
					{
						Language: "es_US",
						Message:  "Hola",
					},
					{
						Language: "ar_KW",
						Message:  "مرحبا",
					},
				},
			},
		}

		syncResult, err := db.SyncRealmTranslations(realm.ID, localizations)
		if err != nil {
			t.Fatal(err)
		}

		wantSyncResult := TranslatcionSyncResult{
			Added:   1,
			Updated: 1,
			Deleted: 2,
		}
		if diff := cmp.Diff(wantSyncResult, *syncResult); diff != "" {
			t.Fatalf("mismatch (-want, +got):\n%s", diff)
		}

		got, err := db.LoadDynamicTranslations()
		if err != nil {
			t.Fatal(err)
		}

		want := []*DynamicTranslation{
			{
				RealmID:   realm.ID,
				MessageID: "greeting",
				Locale:    "en",
				Message:   "Hello",
			},
			{
				RealmID:   realm.ID,
				MessageID: "greeting",
				Locale:    "es",
				Message:   "Hola",
			},
			{
				RealmID:   realm.ID,
				MessageID: "greeting",
				Locale:    "ar",
				Message:   "مرحبا",
			},
		}

		if diff := cmp.Diff(want, got, opts...); diff != "" {
			t.Fatalf("mismatch (-want, +got):\n%s", diff)
		}
	}
}
