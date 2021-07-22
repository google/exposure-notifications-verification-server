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

package database

import (
	"errors"
	"fmt"
	"sort"
)

var (
	// ErrNoSigningKeyManager is the error returned when the key manager cannot be
	// used as a SigningKeyManager.
	ErrNoSigningKeyManager = errors.New("configured key manager cannot be used to manage per-realm keys")

	// ErrValidationFailed is the error returned when validation failed. This
	// should always be considered user error.
	ErrValidationFailed = errors.New("validation failed")
)

// Errorable defines an embeddable struct for managing errors on models.
type Errorable struct {
	// errors is the list of errors on the model, usually from validation. The
	// string key is the column name (or virtual column name) of the field that
	// has errors.
	errors map[string][]string
}

// AddError adds a new error to the list.
func (e *Errorable) AddError(key, err string) {
	e.init()
	e.errors[key] = append(e.errors[key], err)
}

// Errors returns the list of errors.
func (e *Errorable) Errors() map[string][]string {
	e.init()
	return e.errors
}

// ErrorMessages returns the list of error messages.
func (e *Errorable) ErrorMessages() []string {
	e.init()

	// Sort keys so the response is in predictable ordering.
	keys := make([]string, 0, len(e.errors))
	for k := range e.errors {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	l := make([]string, 0, len(e.errors))
	for _, k := range keys {
		v := e.errors[k]
		for _, msg := range v {
			l = append(l, fmt.Sprintf("%s %s", k, msg))
		}
	}
	return l
}

// ErrorsFor returns the list of errors for the key
func (e *Errorable) ErrorsFor(key string) []string {
	e.init()
	return e.errors[key]
}

// ErrorOrNil returns ErrValidationFailed if there are any errors, or nil if
// there are none.
func (e *Errorable) ErrorOrNil() error {
	e.init()
	if len(e.errors) == 0 {
		return nil
	}
	return ErrValidationFailed
}

func (e *Errorable) init() {
	if e.errors == nil {
		e.errors = make(map[string][]string)
	}
}
