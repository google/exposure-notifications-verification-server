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
	"fmt"
	"sync"

	"github.com/gin-gonic/gin"
)

const (
	alertKey = "alert"
	errorKey = "error"
)

type Flash struct {
	// now is the current flash (loaded from a cookie in the current context).
	// next is the upcoming flash to be saved in the next cookie for the next
	// request.
	now, next map[string][]string
	lock      sync.Mutex
}

// New creates a new Flash instance.
func New() *Flash {
	return &Flash{
		now:  make(map[string][]string),
		next: make(map[string][]string),
	}
}

// FromContext returns the flash saved on the given context.
func FromContext(c *gin.Context) *Flash {
	f, ok := c.Get("flash")
	if !ok {
		f := New()
		c.Set("flash", f)
		return f
	}

	typ, ok := f.(*Flash)
	if !ok {
		f := New()
		c.Set("flash", f)
		return f
	}

	return typ
}

// Load parses a flash context from the given value. The value is a
// base64-encoded JSON dump of the flash data.
func Load(val string) (*Flash, error) {
	decoded, err := base64.StdEncoding.DecodeString(val)
	if err != nil {
		return nil, fmt.Errorf("failed to decode value: %w", err)
	}

	var m map[string][]string
	if err := json.Unmarshal(decoded, &m); err != nil {
		return nil, fmt.Errorf("failed to unmarshal: %w", err)
	}

	f := New()
	if len(m) != 0 {
		f.now = m
	}
	return f, nil
}

// Dump produces a saveable output for the next flash value.
func (f *Flash) Dump() (string, error) {
	if len(f.next) == 0 {
		return "", nil
	}

	b, err := json.Marshal(f.next)
	if err != nil {
		return "", fmt.Errorf("failed to marshal as json: %w", err)
	}
	return base64.StdEncoding.EncodeToString(b), nil
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
