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

package database

import (
	"fmt"
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

	l := make([]string, 0, len(e.errors))
	for k, v := range e.errors {
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

func (e *Errorable) init() {
	if e.errors == nil {
		e.errors = make(map[string][]string)
	}
}
