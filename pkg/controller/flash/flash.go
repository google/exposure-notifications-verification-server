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

// Package flash defines flash session behavior.
// TODO(mikehelmick||sethvargo): Now that we're on gorilla, we could move to build in session / flash support.
package flash

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const (
	flashKey = "flash"
	alertKey = "alert"
	errorKey = "error"
)

// contextKey is a unique type to avoid clashing with other packages that use
// context's to pass data.
type contextKey struct{}

// contextKeyFlash is a context key used for flash.
var contextKeyFlash = &contextKey{}

// Flash represents a handle to the current request's flash structure.
type Flash struct {
	w http.ResponseWriter
	r *http.Request

	// now is the current flash (loaded from a cookie in the current context).
	// next is the upcoming flash to be saved in the next cookie for the next
	// request.
	now, next map[string][]string
	lock      sync.Mutex
}

// new creates a flash with the context.
func new(w http.ResponseWriter, r *http.Request) *Flash {
	return &Flash{
		w:    w,
		r:    r,
		now:  make(map[string][]string),
		next: make(map[string][]string),
	}
}

// Clear marks the request to delete the existing flash cookie.
func Clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:    flashKey,
		MaxAge:  -1,
		Expires: time.Unix(0, 0),
		Path:    "/",
	})
}

// FromContext returns the flash saved on the given context.
func FromContext(w http.ResponseWriter, r *http.Request) *Flash {
	f, ok := r.Context().Value(contextKeyFlash).(*Flash)
	if ok {
		return f
	}

	f = new(w, r)
	*r = *r.WithContext(context.WithValue(r.Context(), contextKeyFlash, f))
	return f
}

// LoadFromCookie parses flash data from a cookie.
func (f *Flash) LoadFromCookie() error {
	cookie, err := f.r.Cookie(flashKey)
	if err != nil && !errors.Is(err, http.ErrNoCookie) {
		return err
	}
	if cookie == nil {
		return nil
	}
	// Un-url-encode
	unescaped, err := url.QueryUnescape(cookie.Value)
	if err != nil {
		return fmt.Errorf("failed to unescape value: %w", err)
	}

	// Decode the string
	decoded, err := base64.StdEncoding.DecodeString(unescaped)
	if err != nil {
		return fmt.Errorf("failed to decode value: %w", err)
	}

	// Parse as JSON
	var m map[string][]string
	if err := json.Unmarshal(decoded, &m); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}

	// If there's any data, set it as now, overwriting any existing flash data at
	// that key if it exists.
	for k, v := range m {
		f.now[k] = v
	}
	return nil
}

// Error adds a new error to the upcoming flash instance.
func (f *Flash) Error(msg string, vars ...interface{}) {
	f.Add(errorKey, msg, vars...)
}

// ErrorNow adds a new error to the current flash instance.
func (f *Flash) ErrorNow(msg string, vars ...interface{}) {
	f.AddNow(errorKey, msg, vars...)
}

// Errors returns the list of errors in flash, if any.
func (f *Flash) Errors() []string {
	return f.GetNow(errorKey)
}

// Alert adds a new alert to the upcoming flash instance.
func (f *Flash) Alert(msg string, vars ...interface{}) {
	f.Add(alertKey, msg, vars...)
}

// AlertNow adds a new alert to the current flash instance.
func (f *Flash) AlertNow(msg string, vars ...interface{}) {
	f.AddNow(alertKey, msg, vars...)
}

// Alerts returns the list of errors in flash, if any.
func (f *Flash) Alerts() []string {
	return f.GetNow(alertKey)
}

// Add inserts the message into the upcoming flash for the given key.
func (f *Flash) Add(key, msg string, vars ...interface{}) {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.next[key] = append(f.next[key], fmt.Sprintf(msg, vars...))

	// Set the cookie on write.
	http.SetCookie(f.w, &http.Cookie{
		Name:   flashKey,
		Value:  f.toCookieValue(f.next),
		MaxAge: 300,
		Path:   "/",
	})
}

// AddNow inserts the message into the current flash for the given key.
func (f *Flash) AddNow(key, msg string, vars ...interface{}) {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.now[key] = append(f.now[key], fmt.Sprintf(msg, vars...))
}

// Get returns the upcoming list of messages at the given key.
func (f *Flash) Get(key string) []string {
	f.lock.Lock()
	defer f.lock.Unlock()
	return f.next[key]
}

// GetNow returns the current list of messages at the given key.
func (f *Flash) GetNow(key string) []string {
	f.lock.Lock()
	defer f.lock.Unlock()
	return f.now[key]
}

// toCookieValue converts the given map to its cookie value.
func (f *Flash) toCookieValue(m map[string][]string) string {
	b, err := json.Marshal(m)
	if err != nil {
		// This should never happen because JSON only fails to marshal when the
		// values are unmarshallable (func, chan), or with circular references,
		// neither of which can happen here.
		log.Printf("failed to marshal as json: %v", err)
	}

	// Base64-encode the json.
	encoded := base64.StdEncoding.EncodeToString(b)

	// Escape any characters (cookies are finicky that way).
	escaped := url.QueryEscape(encoded)
	return escaped
}
