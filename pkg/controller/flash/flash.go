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
package flash

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	flashKey = "flash"
	alertKey = "alert"
	errorKey = "error"
)

type Flash struct {
	c *gin.Context

	// now is the current flash (loaded from a cookie in the current context).
	// next is the upcoming flash to be saved in the next cookie for the next
	// request.
	now, next map[string][]string
	lock      sync.Mutex
}

// new creates a flash with the context.
func new(c *gin.Context) *Flash {
	return &Flash{
		c:    c,
		now:  make(map[string][]string),
		next: make(map[string][]string),
	}
}

// Clear marks the request to delete the existing flash cookie.
func Clear(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:    flashKey,
		MaxAge:  -1,
		Expires: time.Unix(0, 0),
		Path:    "/",
	})
}

// FromContext returns the flash saved on the given context.
func FromContext(c *gin.Context) *Flash {
	if f, ok := c.Get(flashKey); ok {
		if typ, ok := f.(*Flash); ok {
			return typ
		}
	}

	f := new(c)
	c.Set(flashKey, f)
	return f
}

// LoadFromCookie parses flash data from a cookie.
func (f *Flash) LoadFromCookie() error {
	val, err := f.c.Cookie(flashKey)
	if err != nil && !errors.Is(err, http.ErrNoCookie) {
		return err
	}
	if val == "" {
		return nil
	}

	// Decode the string
	decoded, err := base64.StdEncoding.DecodeString(val)
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
	http.SetCookie(f.c.Writer, &http.Cookie{
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
