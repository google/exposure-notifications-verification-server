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

package render

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	htmltemplate "html/template"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	texttemplate "text/template"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
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
// and JSON responses. This implementation caches templates and uses a pool of buffers.
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
	templatesLock sync.RWMutex
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

// executeHTMLTemplate executes a single HTML template with the provided data.
func (r *Renderer) executeHTMLTemplate(w io.Writer, name string, data interface{}) error {
	r.templatesLock.RLock()
	defer r.templatesLock.RUnlock()

	if r.templates == nil {
		return fmt.Errorf("no templates are defined")
	}

	return r.templates.ExecuteTemplate(w, name, data)
}

// executeTextTemplate executes a single text template with the provided data.
func (r *Renderer) executeTextTemplate(w io.Writer, name string, data interface{}) error {
	r.templatesLock.RLock()
	defer r.templatesLock.RUnlock()

	if r.templates == nil {
		return fmt.Errorf("no templates are defined")
	}

	return r.textTemplates.ExecuteTemplate(w, name, data)
}

// loadTemplates loads or reloads all templates.
func (r *Renderer) loadTemplates() error {
	r.templatesLock.Lock()
	defer r.templatesLock.Unlock()

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

func selectedIf(v bool) htmltemplate.HTMLAttr {
	if v {
		return "selected"
	}
	return ""
}

func readonlyIf(v bool) htmltemplate.HTMLAttr {
	if v {
		return "readonly"
	}
	return ""
}

func checkedIf(v bool) htmltemplate.HTMLAttr {
	if v {
		return "checked"
	}
	return ""
}

func disabledIf(v bool) htmltemplate.HTMLAttr {
	if v {
		return "disabled"
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

// toStringSlice converts the input slice to strings. The values must be
// primitive or implement the fmt.Stringer interface.
func toStringSlice(i interface{}) ([]string, error) {
	t := reflect.TypeOf(i)
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Slice && t.Kind() != reflect.Array {
		return nil, fmt.Errorf("value is not a slice: %T", i)
	}

	s := reflect.ValueOf(i)
	for s.Kind() == reflect.Ptr {
		s = s.Elem()
	}

	l := make([]string, 0, s.Len())
	for i := 0; i < s.Len(); i++ {
		val := s.Index(i).Interface()
		switch t := val.(type) {
		case fmt.Stringer:
			l = append(l, t.String())
		case string:
			l = append(l, t)
		case int:
			l = append(l, strconv.FormatInt(int64(t), 10))
		case int8:
			l = append(l, strconv.FormatInt(int64(t), 10))
		case int16:
			l = append(l, strconv.FormatInt(int64(t), 10))
		case int32:
			l = append(l, strconv.FormatInt(int64(t), 10))
		case int64:
			l = append(l, strconv.FormatInt(t, 10))
		case uint:
			l = append(l, strconv.FormatUint(uint64(t), 10))
		case uint8:
			l = append(l, strconv.FormatUint(uint64(t), 10))
		case uint16:
			l = append(l, strconv.FormatUint(uint64(t), 10))
		case uint32:
			l = append(l, strconv.FormatUint(uint64(t), 10))
		case uint64:
			l = append(l, strconv.FormatUint(t, 10))
		}
	}

	return l, nil
}

// joinStrings joins a list of strings or string-like things.
func joinStrings(i interface{}, sep string) (string, error) {
	l, err := toStringSlice(i)
	if err != nil {
		return "", nil
	}
	return strings.Join(l, sep), nil
}

// toSentence joins a list of string like things into a human-friendly sentence.
func toSentence(i interface{}, joiner string) (string, error) {
	l, err := toStringSlice(i)
	if err != nil {
		return "", nil
	}

	switch len(l) {
	case 0:
		return "", nil
	case 1:
		return l[0], nil
	case 2:
		return l[0] + " " + joiner + " " + l[1], nil
	default:
		parts, last := l[0:len(l)-1], l[len(l)-1]
		return strings.Join(parts, ", ") + " " + joiner + ", " + last, nil
	}
}

func templateFuncs() htmltemplate.FuncMap {
	return map[string]interface{}{
		"joinStrings":      joinStrings,
		"toSentence":       toSentence,
		"trimSpace":        project.TrimSpace,
		"stringContains":   strings.Contains,
		"toLower":          strings.ToLower,
		"toUpper":          strings.ToUpper,
		"toJSON":           json.Marshal,
		"toBase64":         base64.StdEncoding.EncodeToString,
		"safeHTML":         safeHTML,
		"checkedIf":        checkedIf,
		"selectedIf":       selectedIf,
		"readonlyIf":       readonlyIf,
		"disabledIf":       disabledIf,
		"t":                translate,
		"passwordSentinel": pwdSentinel,
		"hasOne":           hasOne,
		"hasMany":          hasMany,

		"rbac": func() map[string]rbac.Permission {
			return rbac.NamePermissionMap
		},
	}
}

func hasOne(a interface{}) bool {
	s := reflect.ValueOf(a)
	if s.Kind() != reflect.Slice && s.Kind() != reflect.Array {
		return false
	}
	return s.Len() == 1
}

func hasMany(a interface{}) bool {
	s := reflect.ValueOf(a)
	if s.Kind() != reflect.Slice && s.Kind() != reflect.Array {
		return false
	}
	return s.Len() > 1
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
