// Copyright 2021 the Exposure Notifications Verification Server authors
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
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/appsync"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/hashicorp/go-multierror"
)

// DynamicTranslation stores a per-realm localized string that can be used
// for user-facing content (currently only user-report webview).
type DynamicTranslation struct {
	Errorable

	// ID is an auto-increment primary key
	ID uint

	// RealmID, MessageID, Locale is a unique index on this table.

	// RealmID realm that this translation belongs to.
	RealmID uint
	// MessageID is the ID of the message.
	MessageID string
	// Local is the local / language represented.
	Locale string

	// Message is the localized message
	Message string

	CreatedAt time.Time
	UpdatedAt time.Time
}

// Key returns the key for this translation, realmID-locale-msgID
func (d *DynamicTranslation) Key() string {
	return translationKey(d.RealmID, d.Locale, d.MessageID)
}

func translationKey(realmID uint, locale string, msgID string) string {
	return fmt.Sprintf("%d-%s-%s", realmID, locale, msgID)
}

// ListDynamicTranslationsCached is ListDynamicTranslations, but cached.
func (db *Database) ListDynamicTranslationsCached(ctx context.Context, cacher cache.Cacher) ([]*DynamicTranslation, error) {
	if cacher == nil {
		return nil, fmt.Errorf("cacher cannot be nil")
	}

	var translations []*DynamicTranslation
	cacheKey := &cache.Key{
		Namespace: "translations:dynamic",
		Key:       "all",
	}
	if err := cacher.Fetch(ctx, cacheKey, &translations, 10*time.Minute, func() (interface{}, error) {
		return db.ListDynamicTranslations()
	}); err != nil {
		return nil, err
	}
	return translations, nil
}

// ListDynamicTranslations returns all of the dynamic translations for all realms.
// The result of this read should be cached for some period of time.
func (db *Database) ListDynamicTranslations() ([]*DynamicTranslation, error) {
	var translations []*DynamicTranslation
	if err := db.db.
		Model(&DynamicTranslation{}).
		Order("realm_id ASC, locale ASC, message_id ASC").
		Find(&translations).
		Error; err != nil {
		return nil, err
	}
	return translations, nil
}

// localeToLangauge covers things like "en_US" to just "en" to match
// this applications localization strategy.
func localeToLanguage(l string) string {
	l = strings.ToLower(l)
	if len(l) <= 2 {
		return l
	}
	return l[0:2]
}

type TranslationSyncResult struct {
	Added   int
	Updated int
	Deleted int
}

func (db *Database) SyncRealmTranslations(realmID uint, localizations []*appsync.Localization) (*TranslationSyncResult, error) {
	// load all translations for a realm
	var translations []*DynamicTranslation
	if err := db.db.
		Model(&DynamicTranslation{}).
		Where("realm_id = ?", realmID).
		Find(&translations).
		Error; err != nil {
		return nil, err
	}

	existingTranslations := make(map[string]*DynamicTranslation, len(translations))
	for _, t := range translations {
		existingTranslations[t.Key()] = t
	}

	// Load the incoming translations into a map for ease.
	// This also ensures any de-duplication.
	sizeEst := len(localizations)
	if sizeEst > 0 {
		// unroll, assumption that all message IDs contains the same # of localizations.
		sizeEst *= len(localizations[0].Translations)
	}
	incomingTranslations := make(map[string]string, sizeEst)
	for _, l := range localizations {
		msgID := l.MessageID
		for _, t := range l.Translations {
			key := translationKey(realmID, localeToLanguage(t.Language), msgID)
			incomingTranslations[key] = t.Message
		}
	}

	var merr *multierror.Error

	// calculate diff set
	// anything left in existing translations at the end of this is considered "toDelete"
	toAdd := make(map[string]*DynamicTranslation)
	toUpdate := make(map[string]*DynamicTranslation)

	for key, message := range incomingTranslations {
		if cur, ok := existingTranslations[key]; ok {
			// Existing translation was found.
			// It will either need to be updated
			if cur.Message != message {
				cur.Message = message
				toUpdate[key] = cur
			} // or matches and can be dropped.
			delete(existingTranslations, key)
		} else {
			keyParts := strings.Split(key, "-")
			if len(keyParts) != 3 {
				merr = multierror.Append(merr, fmt.Errorf("invalid message key: %q", key))
				continue
			}
			// the key was not found
			toAdd[key] = &DynamicTranslation{
				RealmID:   realmID,
				Locale:    keyParts[1],
				MessageID: keyParts[2],
				Message:   message,
			}
		}
	}

	results := &TranslationSyncResult{}
	// add new translations
	for k, add := range toAdd {
		if err := db.db.Create(add).Error; err != nil {
			merr = multierror.Append(merr, fmt.Errorf("failed to add %q: %w", k, err))
			continue
		}
		results.Added++
	}

	// update existing translations
	for k, update := range toUpdate {
		if err := db.db.Save(update).Error; err != nil {
			merr = multierror.Append(merr, fmt.Errorf("failed to update %q: %w", k, err))
			continue
		}
		results.Updated++
	}

	// delete translations that we no longer have a reference to.
	for k, del := range existingTranslations {
		if err := db.db.Delete(del).Error; err != nil {
			merr = multierror.Append(merr, fmt.Errorf("failed to delete %q: %w", k, err))
			continue
		}
		results.Deleted++
	}

	return results, merr.ErrorOrNil()
}
