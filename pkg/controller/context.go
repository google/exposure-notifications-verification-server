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
	"context"

	"firebase.google.com/go/auth"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/gorilla/sessions"
)

// contextKey is a unique type to avoid clashing with other packages that use
// context's to pass data.
type contextKey string

const (
	contextKeyAuthorizedApp = contextKey("authorizedApp")
	contextKeyRealm         = contextKey("realm")
	contextKeySession       = contextKey("session")
	contextKeyTemplate      = contextKey("template")
	contextKeyUser          = contextKey("user")
	contextKeyFirebaseUser  = contextKey("firebaseUser")
)

// WithAuthorizedApp stores the authorized app on the context.
func WithAuthorizedApp(ctx context.Context, app *database.AuthorizedApp) context.Context {
	m := TemplateMapFromContext(ctx)
	m["currentAuthorizedApp"] = app
	ctx = WithTemplateMap(ctx, m)

	return context.WithValue(ctx, contextKeyAuthorizedApp, app)
}

// AuthorizedAppFromContext retrieves the authorized app from the context. If no
// value exists, it returns nil.
func AuthorizedAppFromContext(ctx context.Context) *database.AuthorizedApp {
	v := ctx.Value(contextKeyAuthorizedApp)
	if v == nil {
		return nil
	}

	t, ok := v.(*database.AuthorizedApp)
	if !ok {
		return nil
	}
	return t
}

// WithRealm stores the current realm on the context.
func WithRealm(ctx context.Context, r *database.Realm) context.Context {
	m := TemplateMapFromContext(ctx)
	m["currentRealm"] = r
	ctx = WithTemplateMap(ctx, m)

	return context.WithValue(ctx, contextKeyRealm, r)
}

// RealmFromContext retrieves the realm from the context. If no
// value exists, it returns nil.
func RealmFromContext(ctx context.Context) *database.Realm {
	v := ctx.Value(contextKeyRealm)
	if v == nil {
		return nil
	}

	t, ok := v.(*database.Realm)
	if !ok {
		return nil
	}
	return t
}

// WithSession stores the session on the request's context for retrieval later.
// Use Session(r) to retrieve the session.
func WithSession(ctx context.Context, session *sessions.Session) context.Context {
	return context.WithValue(ctx, contextKeySession, session)
}

// SessionFromContext retrieves the session on the provided context. If no
// session exists, or if the value in the context is not of the correct type, it
// returns nil.
func SessionFromContext(ctx context.Context) *sessions.Session {
	v := ctx.Value(contextKeySession)
	if v == nil {
		return nil
	}

	t, ok := v.(*sessions.Session)
	if !ok {
		return nil
	}
	return t
}

// TemplateMap is a typemap for the HTML templates.
type TemplateMap map[string]interface{}

// WithTemplateMap creates a context with the given template map.
func WithTemplateMap(ctx context.Context, m TemplateMap) context.Context {
	return context.WithValue(ctx, contextKeyTemplate, m)
}

// TemplateMapFromContext gets the template map on the context. If no map
// exists, it returns an empty map.
func TemplateMapFromContext(ctx context.Context) TemplateMap {
	v := ctx.Value(contextKeyTemplate)
	if v == nil {
		return make(TemplateMap)
	}

	m, ok := v.(TemplateMap)
	if !ok {
		return make(TemplateMap)
	}

	return m
}

// WithUser stores the current user on the context.
func WithUser(ctx context.Context, u *database.User) context.Context {
	m := TemplateMapFromContext(ctx)
	m["currentUser"] = u
	ctx = WithTemplateMap(ctx, m)

	return context.WithValue(ctx, contextKeyUser, u)
}

// UserFromContext retrieves the user from the context. If no value exists, it
// returns nil.
func UserFromContext(ctx context.Context) *database.User {
	v := ctx.Value(contextKeyUser)
	if v == nil {
		return nil
	}

	t, ok := v.(*database.User)
	if !ok {
		return nil
	}
	return t
}

// WithFirebaseUser stores the current firebase user on the context.
func WithFirebaseUser(ctx context.Context, u *auth.UserRecord) context.Context {
	return context.WithValue(ctx, contextKeyFirebaseUser, u)
}

// FirebaseUserFromContext retrieves the firebase user from the context. If no value exists, it
// returns nil.
func FirebaseUserFromContext(ctx context.Context) *auth.UserRecord {
	v := ctx.Value(contextKeyFirebaseUser)
	if v == nil {
		return nil
	}

	t, ok := v.(*auth.UserRecord)
	if !ok {
		return nil
	}
	return t
}
