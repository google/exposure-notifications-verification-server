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

package auth

import (
	"encoding/gob"

	"github.com/gorilla/sessions"
)

// sessionKey is a custom type for keys in the session values.
type sessionKey string

// init registers the session key gob.
func init() {
	gob.Register(sessionKey(""))
}

// sessionSet sets the value in the session.
func sessionSet(session *sessions.Session, key sessionKey, data interface{}) error {
	if session == nil {
		return ErrSessionMissing
	}

	if session.Values == nil {
		session.Values = make(map[interface{}]interface{})
	}

	session.Values[key] = data
	return nil
}

// sessionGet retrieves the given key as a string from the session.
func sessionGet(session *sessions.Session, key sessionKey) (interface{}, error) {
	if session == nil || session.Values == nil {
		return "", ErrSessionMissing
	}

	v, ok := session.Values[key]
	if !ok || v == nil {
		return "", ErrSessionMissing
	}
	return v, nil
}

// sessionClear clears the value from the session.
func sessionClear(session *sessions.Session, key sessionKey) {
	if session == nil {
		return
	}
	delete(session.Values, key)
}

func splitAuthCookie(mainSession *sessions.Session, authSession *sessions.Session, key sessionKey) error {
	v, err := sessionGet(mainSession, key)
	if err != nil {
		return nil
	}
	if err := sessionSet(authSession, key, v); err != nil {
		return err
	}
	sessionClear(mainSession, key)
	return nil
}

func joinAuthCookie(mainSession *sessions.Session, authSession *sessions.Session, key sessionKey) error {
	v, err := sessionGet(authSession, key)
	if err != nil {
		return nil
	}
	sessionSet(mainSession, key, v)
	sessionClear(authSession, key)
	return nil
}
