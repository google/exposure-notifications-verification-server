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

package signer

import (
	"context"
	"crypto"
	"fmt"
	"sync"
)

type NewKeyManagerFunc func(context.Context) (KeyManager, error)

type KeyManager interface {
	NewSigner(ctx context.Context, keyID string) (crypto.Signer, error)
}

var (
	registry = map[string]NewKeyManagerFunc{}
	mu       sync.Mutex
)

func RegisterKeyManager(name string, f NewKeyManagerFunc) error {
	mu.Lock()
	defer mu.Unlock()

	if _, ok := registry[name]; ok {
		return fmt.Errorf("key manager is already registered: `%v`", name)
	}
	registry[name] = f
	return nil
}

func New(ctx context.Context, name string) (KeyManager, error) {
	mu.Lock()
	defer mu.Unlock()

	if f, ok := registry[name]; ok {
		return f(ctx)
	}
	return nil, fmt.Errorf("key manager driver not found: `%v`", name)
}

func NewDefault(ctx context.Context) (KeyManager, error) {
	mu.Lock()
	defer mu.Unlock()

	if len(registry) != 1 {
		return nil, fmt.Errorf("cannot use default key manager, more than one registered")
	}
	for _, v := range registry {
		return v(ctx)
	}
	return nil, fmt.Errorf("you reached unreachable code, congratulations.")
}
