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

package controller

import (
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/gorilla/sessions"
)

// sessionKey is a unique type to avoid overwriting over values in the session
// map. It also makes it much more difficult to modify values in the session map
// unintentionally.
type sessionKey string

const (
	sessionKeyFirebaseCookie = sessionKey("firebaseCookie")
	sessionKeyRealmID        = sessionKey("realmID")
)

// StoreSessionFirebaseCookie stores the firebase cookie in the session. If the
// provided session or cookie is nil/empty, it does nothing.
func StoreSessionFirebaseCookie(session *sessions.Session, firebaseCookie string) {
	if session == nil || firebaseCookie == "" {
		return
	}
	session.Values[sessionKeyFirebaseCookie] = firebaseCookie
}

// ClearSessionFirebaseCookie clears the firebase cookie from the session.
func ClearSessionFirebaseCookie(session *sessions.Session) {
	sessionClear(session, sessionKeyFirebaseCookie)
}

// FirebaseCookieFromSession extracts the firebase cookie from the session.
func FirebaseCookieFromSession(session *sessions.Session) string {
	v := sessionGet(session, sessionKeyFirebaseCookie)
	if v == nil {
		return ""
	}

	t, ok := v.(string)
	if !ok {
		delete(session.Values, sessionKeyFirebaseCookie)
		return ""
	}

	return t
}

// StoreSessionRealm stores the realm's ID in the session.
func StoreSessionRealm(session *sessions.Session, realm *database.Realm) {
	if session == nil || realm == nil {
		return
	}
	session.Values[sessionKeyRealmID] = realm.ID
}

// ClearSessionRealm clears the realm from the session.
func ClearSessionRealm(session *sessions.Session) {
	sessionClear(session, sessionKeyRealmID)
}

// RealmIDFromSession extracts the realm from the session.
func RealmIDFromSession(session *sessions.Session) uint {
	v := sessionGet(session, sessionKeyRealmID)
	if v == nil {
		return 0
	}

	t, ok := v.(uint)
	if !ok {
		delete(session.Values, sessionKeyRealmID)
		return 0
	}

	return t
}

func sessionGet(session *sessions.Session, key sessionKey) interface{} {
	if session == nil || session.Values == nil {
		return nil
	}
	return session.Values[key]
}

func sessionClear(session *sessions.Session, key sessionKey) {
	if session == nil {
		return
	}
	delete(session.Values, key)
}
