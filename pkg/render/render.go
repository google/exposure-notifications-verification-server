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
	"bytes"
	"context"
	"fmt"
	htmltemplate "html/template"
	"os"
	"path/filepath"
	"strings"
	"sync"
	texttemplate "text/template"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/leonelquinteros/gotext"

	"go.uber.org/zap"
)

// allowedResponseCodes are the list of allowed response codes. This is
// primarily here to catch if someone, in the future, accidentally includes a
// bad status code.
var allowedResponseCodes = map[int]struct{}{
	200: {},
	400: {},
	401: {},
	404: {},
	405: {},
	409: {},
	412: {},
	413: {},
	429: {},
	500: {},
}

// Renderer is responsible for rendering various content and templates like HTML
// and JSON responses.
type Renderer struct {
	// debug indicates templates should be reloaded on each invocation and real
	// error responses should be rendered. Do not enable in production.
	debug bool

	// logger is the log writer.
	logger *zap.SugaredLogger

	// rendererPool is a pool of *bytes.Buffer, used as a rendering buffer to
	// prevent partial responses being sent to clients.
	rendererPool *sync.Pool

	// templates is the actually collection of templates. templatesLoader is a
	// function for (re)loading templates. templatesLock is a mutex to prevent
	// concurrent modification of the templates field.
	templates     *htmltemplate.Template
	textTemplates *texttemplate.Template
	templatesRoot string
}

// New creates a new renderer with the given details.
func New(ctx context.Context, root string, debug bool) (*Renderer, error) {
	logger := logging.FromContext(ctx)

	r := &Renderer{
		debug:  debug,
		logger: logger,
		rendererPool: &sync.Pool{
			New: func() interface{} {
				return bytes.NewBuffer(make([]byte, 0, 1024))
			},
		},
		templatesRoot: root,
	}

	// Load initial templates
	if err := r.loadTemplates(); err != nil {
		return nil, err
	}

	return r, nil
}

// loadTemplates loads or reloads all templates.
func (r *Renderer) loadTemplates() error {
	if r.templatesRoot == "" {
		return nil
	}

	tmpl := htmltemplate.New("").
		Option("missingkey=zero").
		Funcs(templateFuncs())
	txttmpl := texttemplate.New("").
		Funcs(textFuncs())
	if err := loadTemplates(tmpl, txttmpl, r.templatesRoot); err != nil {
		return fmt.Errorf("failed to load templates: %w", err)
	}

	r.templates = tmpl
	r.textTemplates = txttmpl
	return nil
}

func loadTemplates(tmpl *htmltemplate.Template, txttmpl *texttemplate.Template, root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if strings.HasSuffix(info.Name(), ".html") {
			if _, err := tmpl.ParseFiles(path); err != nil {
				return fmt.Errorf("failed to parse %s: %w", path, err)
			}
		}

		if strings.HasSuffix(info.Name(), ".txt") {
			if _, err := txttmpl.ParseFiles(path); err != nil {
				return fmt.Errorf("failed to parse %s: %w", path, err)
			}
		}

		return nil
	})
}

// safeHTML un-escapes known safe html.
func safeHTML(s string) htmltemplate.HTML {
	return htmltemplate.HTML(s)
}

func selectedIf(v bool) htmltemplate.HTML {
	if v {
		return htmltemplate.HTML("selected")
	}
	return ""
}

func readonlyIf(v bool) htmltemplate.HTML {
	if v {
		return htmltemplate.HTML("readonly")
	}
	return ""
}

func disabledIf(v bool) htmltemplate.HTML {
	if v {
		return htmltemplate.HTML("disabled")
	}
	return ""
}

// translate accepts a message printer (populated by middleware) and prints the
// translated text for the given key. If the printer is nil, an error is
// returned.
func translate(l *gotext.Locale, key string, vars ...interface{}) (string, error) {
	if l == nil {
		return "", fmt.Errorf("missing locale")
	}

	v := l.Get(key, vars...)
	if v == "" || v == key {
		return "", fmt.Errorf("unknown i18n key %q", key)
	}
	return v, nil
}

func templateFuncs() htmltemplate.FuncMap {
	return map[string]interface{}{
		"joinStrings":      strings.Join,
		"trimSpace":        project.TrimSpace,
		"stringContains":   strings.Contains,
		"toLower":          strings.ToLower,
		"toUpper":          strings.ToUpper,
		"safeHTML":         safeHTML,
		"selectedIf":       selectedIf,
		"readonlyIf":       readonlyIf,
		"disabledIf":       disabledIf,
		"t":                translate,
		"passwordSentinel": pwdSentinel,
	}
}

func pwdSentinel() string {
	return project.PasswordSentinel
}

func textFuncs() texttemplate.FuncMap {
	return map[string]interface{}{
		"trimSpace": project.TrimSpace,
	}
}

// AllowedResponseCode returns true if the code is a permitted response code,
// false otherwise.
func (r *Renderer) AllowedResponseCode(code int) bool {
	_, ok := allowedResponseCodes[code]
	return ok
}
