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
	"context"
	"fmt"
	"sync"
	"time"
)

type TransmissionRisk struct {
	Risk               int `json:"risk"`
	PastRollingPeriods int `json:"pastRollingPeriods"`
}

type IssuedPIN interface {
	Pin() string
	TransmissionRisks() []TransmissionRisk
	Claims() map[string]string
	EndTime() time.Time
	IsClaimed() bool
}

type NewDBFunc func(context.Context) (Database, error)

type Database interface {
	InsertPIN(pin string, risks []TransmissionRisk, addClaims map[string]string, duration time.Duration) (IssuedPIN, error)
	RetrievePIN(pin string) (IssuedPIN, error)
	MarkPINClaimed(pin string) error
}

var (
	registry = map[string]NewDBFunc{}
	mu       sync.Mutex
)

func RegisterDatabase(name string, f NewDBFunc) error {
	mu.Lock()
	defer mu.Unlock()

	if _, ok := registry[name]; ok {
		return fmt.Errorf("database is already registered: `%v`", name)
	}
	registry[name] = f
	return nil
}

func New(ctx context.Context, name string) (Database, error) {
	mu.Lock()
	defer mu.Unlock()

	if f, ok := registry[name]; ok {
		return f(ctx)
	}
	return nil, fmt.Errorf("database driver not found: `%v`", name)
}

func NewDefault(ctx context.Context) (Database, error) {
	mu.Lock()
	defer mu.Unlock()

	if len(registry) != 1 {
		return nil, fmt.Errorf("cannot use default database, more than one registered")
	}
	for _, v := range registry {
		return v(ctx)
	}
	return nil, fmt.Errorf("you reached unreachable code, congratulations.")
}
