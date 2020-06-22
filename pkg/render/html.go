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

// Package render provides assistance for rendering templates
package render

import (
	"html/template"
	"io"
	"log"
	"strings"
)

type HTML struct {
	root *template.Template
}

func (h *HTML) Render(wr io.Writer, name string, data interface{}) error {
	return h.root.ExecuteTemplate(wr, name, data)
}

func LoadHTMLGlob(pattern string) *HTML {
	tmpl := template.Must(template.ParseGlob(pattern))

	definedTemplates := strings.Replace(tmpl.DefinedTemplates(), "; defined templates are: ", "", 1)
	names := strings.Split(definedTemplates, ", ")

	log.Printf("Loaded %v HTML Templates", len(names))
	for i, name := range names {
		names[i] = strings.TrimSuffix(strings.TrimPrefix(name, "\""), "\"")
		log.Printf("    - %v", names[i])
	}

	return &HTML{tmpl}
}
