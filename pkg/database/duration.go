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
	"database/sql"
	"database/sql/driver"
	"fmt"
	"time"
)

var _ sql.Scanner = (*DurationSeconds)(nil)
var _ driver.Valuer = (*DurationSeconds)(nil)

// DurationSeconds is a custom type for writing and reating a time.Duration to be stored
// as seconds in the database.
type DurationSeconds struct {
	Duration time.Duration

	// AsString allows this value to be updated and parsed using the Update() method.
	AsString string
}

func FromDuration(d time.Duration) DurationSeconds {
	return DurationSeconds{
		Duration: d,
		AsString: d.String(),
	}
}

func (d *DurationSeconds) Days() int64 {
	return int64(d.Duration.Hours() / 24.0)
}

// Update attempts to parse the AsString value and set is as the duration
func (d *DurationSeconds) Update() error {
	newDuration, err := time.ParseDuration(d.AsString)
	if err != nil {
		return err
	}
	d.Duration = newDuration
	return nil
}

// Scan takes a int64 value in seconds and converts that to a time.Duration
func (d *DurationSeconds) Scan(src interface{}) error {
	if src == nil {
		d.Duration = 0
		return nil
	}
	v, ok := src.(int64)
	if !ok {
		return fmt.Errorf("invalid scan type")
	}
	d.Duration = time.Duration(v) * time.Second
	d.AsString = d.Duration.String()
	return nil
}

// Value converts the internal time.Duration value to seconds and returns
// that value as an int64 for saving to the database.
func (d DurationSeconds) Value() (driver.Value, error) {
	v := int64(d.Duration.Seconds())
	return v, nil
}
