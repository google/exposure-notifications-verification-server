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
	factorCount              = sessionKey("factorCount")
	mfaPrompted              = sessionKey("mfaPrompted")
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

// StoreSessionFactorCount stores count of MFA factors to session.
func StoreSessionFactorCount(session *sessions.Session, count uint) {
	if session == nil {
		return
	}
	session.Values[factorCount] = count
}

// ClearSessionFactorCount clears the MFA factor count from the session.
func ClearSessionFactorCount(session *sessions.Session) {
	sessionClear(session, factorCount)
}

// FactorCountFromSession extracts the number of MFA factors from the session.
func FactorCountFromSession(session *sessions.Session) uint {
	v := sessionGet(session, factorCount)
	if v == nil {
		return 0
	}

	f, ok := v.(uint)
	if !ok {
		delete(session.Values, factorCount)
		return 0
	}

	return f
}

// StoreSessionMFAPrompted stores if the user was prompted for MFA.
func StoreSessionMFAPrompted(session *sessions.Session, prompted bool) {
	if session == nil {
		return
	}
	session.Values[mfaPrompted] = prompted
}

// ClearMFAPrompted clears the MFA prompt bit.
func ClearMFAPrompted(session *sessions.Session) {
	sessionClear(session, mfaPrompted)
}

// MFAPromptedFromSession extracts if the user was prompted from MFA.
func MFAPromptedFromSession(session *sessions.Session) bool {
	v := sessionGet(session, mfaPrompted)
	if v == nil {
		return false
	}

	f, ok := v.(bool)
	if !ok {
		delete(session.Values, mfaPrompted)
		return false
	}

	return f
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
