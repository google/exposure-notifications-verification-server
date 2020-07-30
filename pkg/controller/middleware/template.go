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

// Package html provides a gorilla middleware that injects the HTML template
// map into the context.
// This is application specific in that it sets up common things based on config.
package html

import (
	"context"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/config"
)

// contextKey is a unique type to avoid clashing with other packages that use
// context's to pass data.
type contextKey struct{}

// contextKeyTemplateMap is a context key used for the user.
var contextKeyTemplateMap = &contextKey{}

// WithTemplateMap sets the user in the context.
func WithTemplateMap(ctx context.Context, m TemplateMap) context.Context {
	return context.WithValue(ctx, contextKeyTemplateMap, m)
}

// TemplateMapFromContext gets the template map on the context. If no map
// exists, it returns nil.
func TemplateMapFromContext(ctx context.Context) TemplateMap {
	v := ctx.Value(contextKeyTemplateMap)
	if v == nil {
		return nil
	}

	m, ok := v.(TemplateMap)
	if !ok {
		return nil
	}
	return m
}

// TemplateMap is a typemap for the HTML templates.
type TemplateMap map[string]interface{}

type VariableInjector struct {
	config *config.ServerConfig
}

func New(config *config.ServerConfig) *VariableInjector {
	return &VariableInjector{config}
}

// GetTemplateMap gets the TemplateMap on the request's context. If the map does
// not exist, a new one is created and persisted on the request's context.
func GetTemplateMap(r *http.Request) TemplateMap {
	m := TemplateMapFromContext(r.Context())
	if m != nil {
		return m
	}

	m = make(TemplateMap)
	*r = *r.WithContext(WithTemplateMap(r.Context(), m))
	return m
}

func (vi *VariableInjector) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m := GetTemplateMap(r)
		if _, ok := m["server"]; !ok {
			m["server"] = vi.config.ServerName
		}
		if _, ok := m["title"]; !ok {
			m["title"] = vi.config.ServerName
		}

		r = r.WithContext(WithTemplateMap(r.Context(), m))
		next.ServeHTTP(w, r)
	})
}
