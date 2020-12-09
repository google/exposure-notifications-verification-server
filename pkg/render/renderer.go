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

// Package render defines rendering functionality.
package render

import (
	"net/http"

	"github.com/google/exposure-notifications-verification-server/internal/icsv"
)

// Renderer is responsible for rendering various content and templates like HTML
// and JSON responses.
type Renderer interface {
	RenderCSV(w http.ResponseWriter, code int, filename string, data icsv.Marshaler)
	RenderEmail(tmpl string, data interface{}) ([]byte, error)

	RenderHTML(w http.ResponseWriter, tmpl string, data interface{})
	RenderHTML500(w http.ResponseWriter, err error)
	RenderHTMLStatus(w http.ResponseWriter, code int, tmpl string, data interface{})

	RenderJSON(w http.ResponseWriter, code int, data interface{})
	RenderJSON500(w http.ResponseWriter, err error)
}
