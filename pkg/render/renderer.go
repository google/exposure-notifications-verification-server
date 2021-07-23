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

// Package render defines rendering functionality.
package render

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	htmltemplate "html/template"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	texttemplate "text/template"
	"time"

	"github.com/dustin/go-humanize"
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
	http.StatusOK:                    {},
	http.StatusBadRequest:            {},
	http.StatusUnauthorized:          {},
	http.StatusNotFound:              {},
	http.StatusMethodNotAllowed:      {},
	http.StatusConflict:              {},
	http.StatusPreconditionFailed:    {},
	http.StatusRequestEntityTooLarge: {},
	http.StatusTooManyRequests:       {},
	http.StatusInternalServerError:   {},
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

	fs fs.FS
}

// New creates a new renderer with the given details.
func New(ctx context.Context, fsys fs.FS, debug bool) (*Renderer, error) {
	logger := logging.FromContext(ctx)

	r := &Renderer{
		debug:  debug,
		logger: logger,
		rendererPool: &sync.Pool{
			New: func() interface{} {
				return bytes.NewBuffer(make([]byte, 0, 1024))
			},
		},
		fs: fsys,
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

	if r.fs == nil {
		return nil
	}

	htmltmpl := htmltemplate.New("").
		Option("missingkey=zero").
		Funcs(r.templateFuncs())

	texttmpl := texttemplate.New("").
		Funcs(r.textFuncs())

	if err := loadTemplates(r.fs, htmltmpl, texttmpl); err != nil {
		return fmt.Errorf("failed to load templates: %w", err)
	}

	r.templates = htmltmpl
	r.textTemplates = texttmpl
	return nil
}

func loadTemplates(fsys fs.FS, htmltmpl *htmltemplate.Template, texttmpl *texttemplate.Template) error {
	// You might be thinking to yourself, wait, why don't you just use
	// template.ParseFS(fsys, "**/*.html"). Well, still as of Go 1.16, glob
	// doesn't support shopt globbing, so you still have to walk the entire
	// filepath.
	return fs.WalkDir(fsys, ".", func(pth string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if strings.HasSuffix(info.Name(), ".html") {
			if _, err := htmltmpl.ParseFS(fsys, pth); err != nil {
				return fmt.Errorf("failed to parse %s: %w", pth, err)
			}
		}

		if strings.HasSuffix(info.Name(), ".txt") {
			if _, err := texttmpl.ParseFS(fsys, pth); err != nil {
				return fmt.Errorf("failed to parse %s: %w", pth, err)
			}
		}

		return nil
	})
}

// safeHTML un-escapes known safe html.
func safeHTML(s string) htmltemplate.HTML {
	return htmltemplate.HTML(s)
}

// translateWithFallback accepts a message printer and prints the translation,
// and also accepts a fallback string if the key isn't known.
func translateWithFallback(t gotext.Translator, fallback string, key string, vars ...interface{}) string {
	if t == nil {
		return fmt.Sprintf(fallback, vars...)
	}

	v := t.Get(key, vars...)
	if v == "" || v == key {
		return fmt.Sprintf(fallback, vars...)
	}
	return v
}

// translate accepts a message printer (populated by middleware) and prints the
// translated text for the given key. If the printer is nil, an error is
// returned.
func translate(t gotext.Translator, key string, vars ...interface{}) (string, error) {
	if t == nil {
		return "", fmt.Errorf("missing translator")
	}

	v := t.Get(key, vars...)
	if v == "" || v == key {
		return "", fmt.Errorf("unknown i18n key %q", key)
	}
	return v, nil
}

// humanizeTime prints time in a human-readable format.
func humanizeTime(i interface{}) (string, error) {
	switch typ := i.(type) {
	case time.Time:
		return humanize.Time(typ), nil
	case *time.Time:
		if typ == nil {
			return "never", nil
		}
		return humanize.Time(*typ), nil
	default:
		return "", fmt.Errorf("unsupported type %T", typ)
	}
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

func valueIfTruthy(s string) func(i interface{}) htmltemplate.HTMLAttr {
	return func(i interface{}) htmltemplate.HTMLAttr {
		if i == nil {
			return ""
		}

		v := reflect.ValueOf(i)
		if !v.IsValid() {
			return ""
		}

		//nolint // Complains about non-exhaustive search, but that's intentional
		switch v.Kind() {
		case reflect.Bool:
			if v.Bool() {
				return htmltemplate.HTMLAttr(s)
			}
		case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
			if v.Len() > 0 {
				return htmltemplate.HTMLAttr(s)
			}
		default:
		}

		return ""
	}
}

func (r *Renderer) templateFuncs() htmltemplate.FuncMap {
	return map[string]interface{}{
		"jsIncludeTag":  assetIncludeTag(r.fs, "static/js", jsIncludeTmpl, &jsIncludeTagCache, r.debug),
		"cssIncludeTag": assetIncludeTag(r.fs, "static/css", cssIncludeTmpl, &cssIncludeTagCache, r.debug),

		"joinStrings":      joinStrings,
		"toSentence":       toSentence,
		"trimSpace":        project.TrimSpace,
		"stringContains":   strings.Contains,
		"toLower":          strings.ToLower,
		"toUpper":          strings.ToUpper,
		"toJSON":           json.Marshal,
		"humanizeTime":     humanizeTime,
		"toBase64":         base64.StdEncoding.EncodeToString,
		"safeHTML":         safeHTML,
		"checkedIf":        valueIfTruthy("checked"),
		"requiredIf":       valueIfTruthy("required"),
		"selectedIf":       valueIfTruthy("selected"),
		"readonlyIf":       valueIfTruthy("readonly"),
		"disabledIf":       valueIfTruthy("disabled"),
		"invalidIf":        valueIfTruthy("is-invalid"),
		"t":                translate,
		"tDefault":         translateWithFallback,
		"passwordSentinel": pwdSentinel,
		"hasOne":           hasOne,
		"hasMany":          hasMany,

		"pathEscape":    url.PathEscape,
		"pathUnescape":  url.PathUnescape,
		"queryEscape":   url.QueryEscape,
		"queryUnescape": url.QueryUnescape,

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

func (r *Renderer) textFuncs() texttemplate.FuncMap {
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
