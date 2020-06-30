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
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"

	"github.com/gorilla/context"
)

const (
	Variables = "variables"
)

// TemplateMap is a typemap for the HTML templates.
type TemplateMap map[string]interface{}

type VariableInjector struct {
	config *config.ServerConfig
}

func New(config *config.ServerConfig) *VariableInjector {
	return &VariableInjector{config}
}

func GetTemplateMap(r *http.Request) TemplateMap {
	m := context.Get(r, Variables)
	if m == nil {
		newMap := TemplateMap{}
		context.Set(r, Variables, newMap)
		return newMap
	}

	typed, ok := m.(TemplateMap)
	if ok {
		return typed
	}
	logging.FromContext(r.Context()).Errorf("request context item '%v' is of the wrong type", Variables)
	newMap := TemplateMap{}
	context.Set(r, Variables, newMap)
	return newMap
}

func (vi *VariableInjector) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m := TemplateMap{}
		m["server"] = vi.config.ServerName
		m["title"] = vi.config.ServerName
		context.Set(r, Variables, m)
		next.ServeHTTP(w, r)
	})
}
