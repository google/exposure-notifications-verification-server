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
	"fmt"

	"firebase.google.com/go/auth"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"
	"github.com/gorilla/sessions"
)

// contextKey is a unique type to avoid clashing with other packages that use
// context's to pass data.
type contextKey string

const (
	contextKeyAuthorizedApp = contextKey("authorizedApp")
	contextKeyFirebaseUser  = contextKey("firebaseUser")
	contextKeyMembership    = contextKey("membership")
	contextKeyMemberships   = contextKey("memberships")
	contextKeyRealm         = contextKey("realm")
	contextKeyRequestID     = contextKey("requestID")
	contextKeySession       = contextKey("session")
	contextKeyTemplate      = contextKey("template")
	contextKeyUser          = contextKey("user")
	contextKeyOS            = contextKey("os")
)

// WithOperatingSystem stores the operating system enum in the context.
func WithOperatingSystem(ctx context.Context, os database.OSType) context.Context {
	return context.WithValue(ctx, contextKeyOS, os)
}

// OperatingSystemFromContext retrieves the operating system enum from the context. If
// no value exists, UnknownOS is returned.
func OperatingSystemFromContext(ctx context.Context) database.OSType {
	v := ctx.Value(contextKeyOS)
	if v == nil {
		return database.OSTypeUnknown
	}

	t, ok := v.(database.OSType)
	if !ok {
		return database.OSTypeUnknown
	}
	return t
}

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

// WithRealm stores the current realm on the context and the realm.ID
// on the observability context.
func WithRealm(ctx context.Context, r *database.Realm) context.Context {
	m := TemplateMapFromContext(ctx)
	m["currentRealm"] = r
	ctx = WithTemplateMap(ctx, m)
	ctx = observability.WithRealmID(ctx, uint64(r.ID))

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

// WithRequestID stores the request ID on the context.
func WithRequestID(ctx context.Context, id string) context.Context {
	m := TemplateMapFromContext(ctx)
	m["requestID"] = id
	ctx = WithTemplateMap(ctx, m)

	return context.WithValue(ctx, contextKeyRequestID, id)
}

// RequestIDFromContext retrieves the request ID from the context. If no value
// exists, it returns the empty string.
func RequestIDFromContext(ctx context.Context) string {
	v := ctx.Value(contextKeyRequestID)
	if v == nil {
		return ""
	}

	t, ok := v.(string)
	if !ok {
		return ""
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

// Title sets the title on the template map. If a title already exists, the new
// value is prepended.
func (m TemplateMap) Title(f string, args ...interface{}) {
	if f == "" {
		return
	}

	s := f
	if len(args) > 0 {
		s = fmt.Sprintf(f, args...)
	}

	if current := m["title"]; current != nil && current != "" {
		m["title"] = fmt.Sprintf("%s | %s", s, current)
		return
	}

	m["title"] = s
}

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

// WithMemberships stores the user's available memberships on the context.
func WithMemberships(ctx context.Context, u []*database.Membership) context.Context {
	m := TemplateMapFromContext(ctx)
	m["currentMemberships"] = u
	ctx = WithTemplateMap(ctx, m)

	return context.WithValue(ctx, contextKeyMemberships, u)
}

// MembershipsFromContext retrieves the membership from the context. If no value
// exists, it returns nil.
func MembershipsFromContext(ctx context.Context) []*database.Membership {
	v := ctx.Value(contextKeyMemberships)
	if v == nil {
		return nil
	}

	t, ok := v.([]*database.Membership)
	if !ok {
		return nil
	}
	return t
}

// WithMembership stores the current membership on the context.
func WithMembership(ctx context.Context, u *database.Membership) context.Context {
	m := TemplateMapFromContext(ctx)
	m["currentMembership"] = u
	ctx = WithTemplateMap(ctx, m)

	return context.WithValue(ctx, contextKeyMembership, u)
}

// MembershipFromContext retrieves the membership from the context. If no value
// exists, it returns nil.
func MembershipFromContext(ctx context.Context) *database.Membership {
	v := ctx.Value(contextKeyMembership)
	if v == nil {
		return nil
	}

	t, ok := v.(*database.Membership)
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
