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

	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

// contextKey is a unique type to avoid clashing with other packages that use
// context's to pass data.
type contextKey string

var (
	// contextKeyAuthorizedApp is a context key used for the authorized app.
	contextKeyAuthorizedApp = contextKey("authapp")

	// contextKeyRealm is a context key for the realm.
	contextKeyRealm = contextKey("realm")

	// contextKeyTemplate is a context key for the template.
	contextKeyTemplate = contextKey("template")

	// contextKeyUser is a context key used for the user.
	contextKeyUser = contextKey("user")
)

// WithAuthorizedApp sets the AuthorizedApp in the context.
func WithAuthorizedApp(ctx context.Context, u *database.AuthorizedApp) context.Context {
	return context.WithValue(ctx, contextKeyAuthorizedApp, u)
}

// AuthorizedAppFromContext gets the currently logged in AuthorizedApp from the request context.
// If no AuthorizedApp exists on the request context, or if the value in the request is
// not a AuthorizedApp object, the result will be nil.
func AuthorizedAppFromContext(ctx context.Context) *database.AuthorizedApp {
	v := ctx.Value(contextKeyAuthorizedApp)
	if v == nil {
		return nil
	}

	authorizedApp, ok := v.(*database.AuthorizedApp)
	if !ok {
		return nil
	}
	return authorizedApp
}

// WithRealm sets the realm in the cotnext.
func WithRealm(ctx context.Context, r *database.Realm) context.Context {
	return context.WithValue(ctx, contextKeyRealm, r)
}

// RealmFromContext gets the currently selected realm for the current user session.
// If none is selected, nil is returned.
func RealmFromContext(ctx context.Context) *database.Realm {
	v := ctx.Value(contextKeyRealm)
	if v == nil {
		return nil
	}

	realm, ok := v.(*database.Realm)
	if !ok {
		return nil
	}
	return realm
}

// TemplateMap is a typemap for the HTML templates.
type TemplateMap map[string]interface{}

// WithTemplateMap sets the user in the context.
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

// WithUser sets the user in the context.
func WithUser(ctx context.Context, u *database.User) context.Context {
	return context.WithValue(ctx, contextKeyUser, u)
}

// UserFromContext gets the currently logged in user from the request context.
// If no user exists on the request context, or if the value in the request is
// not a user object, the result will be nil.
func UserFromContext(ctx context.Context) *database.User {
	v := ctx.Value(contextKeyUser)
	if v == nil {
		return nil
	}

	user, ok := v.(*database.User)
	if !ok {
		return nil
	}
	return user
}
