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

// Package flash implements flash messages.
package flash

import (
	"encoding/gob"
	"fmt"
)

// flashKey is a custom type for inserting data into a map.
type flashKey string

const (
	// flashKeyAlert is the session values key used for alerts.
	flashKeyAlert flashKey = "_alert"

	// flashKeyAlert is the session values key used for errors.
	flashKeyError flashKey = "_error"
)

func init() {
	gob.Register(*new(flashKey))
}

// Flash is a collection of data that is discarded on read. It's designed to be
// compatible with sessions.Values.
type Flash struct {
	values map[interface{}]interface{}
}

// New creates a new flash handler.
func New(values map[interface{}]interface{}) *Flash {
	if values == nil {
		values = make(map[interface{}]interface{})
	}
	return &Flash{values}
}

// Error adds a new error to the upcoming flash instance.
func (f *Flash) Error(msg string, vars ...interface{}) {
	f.add(flashKeyError, msg, vars...)
}

// Errors returns the list of errors in flash, if any.
func (f *Flash) Errors() []string {
	return f.get(flashKeyError)
}

// Alert adds a new alert to the upcoming flash instance.
func (f *Flash) Alert(msg string, vars ...interface{}) {
	f.add(flashKeyAlert, msg, vars...)
}

// Alerts returns the list of errors in flash, if any.
func (f *Flash) Alerts() []string {
	return f.get(flashKeyAlert)
}

// Clear removes all items from the flash. It's rare to call Clear since flashes
// are cleared automatically upon reading.
func (f *Flash) Clear() {
	delete(f.values, flashKeyError)
	delete(f.values, flashKeyAlert)
}

// add inserts the message into the upcoming flash for the given key.
func (f *Flash) add(key flashKey, msg string, vars ...interface{}) {
	var data []string
	if v, ok := f.values[key]; ok {
		data, _ = v.([]string)
	}
	f.values[key] = append(data, fmt.Sprintf(msg, vars...))
}

// get returns the messages in the key, clearing the values stored at the key.
func (f *Flash) get(key flashKey) []string {
	if v, ok := f.values[key]; ok {
		delete(f.values, key)
		flashes, _ := v.([]string)
		return flashes
	}
	return nil
}
