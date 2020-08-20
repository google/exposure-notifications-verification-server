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

// Package cache implements an caches for objects.
package cache

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"time"
)

var (
	ErrMissingFetchFunc = fmt.Errorf("missing fetch function")
	ErrNotFound         = fmt.Errorf("key not found")
	ErrStopped          = fmt.Errorf("cacher is stopped")
)

// FetchFunc is a function used to Fetch in a cacher.
type FetchFunc func() (interface{}, error)

// Cacher is an interface that defines caching.
type Cacher interface {
	// Closer closes the cache, cleaning up any stale entries. A closed cache is
	// no longer valid and attempts to call methods should return an error or
	// (less preferred) panic.
	io.Closer

	// Fetch retrieves the named item from the cache. If the item does not exist,
	// it calls FetchFunc to create the item. If FetchFunc returns an error, the
	// error is bubbled up the stack and no value is cached. If FetchFunc
	// succeeds, the value is cached for the provided TTL.
	Fetch(context.Context, string, interface{}, time.Duration, FetchFunc) error

	// Read gets an item from the cache and reads it into the provided interface.
	// If it does not exist, it returns ErrNotFound.
	Read(context.Context, string, interface{}) error

	// Write adds an item to the cache, overwriting if it already exists, caching
	// for TTL. It returns any errors that occur on writing.
	Write(context.Context, string, interface{}, time.Duration) error

	// Delete removes an item from the cache, returning any errors that occur.
	Delete(context.Context, string) error
}

// toValue converts the given interface into it's value, making sure its a
// concrete type.
func toValue(i interface{}) (reflect.Value, error) {
	v := reflect.ValueOf(i)

	if !v.IsValid() {
		return reflect.Value{}, fmt.Errorf("value is invalid")
	}

	for v.CanAddr() {
		v = v.Addr()
	}

	for v.Type().Kind() == reflect.Ptr {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}

	return v, nil
}

// readInto reads in into out, setting the value of out to the value of in.
func readInto(in, out interface{}) error {
	ov, err := toValue(out)
	if err != nil {
		return err
	}

	if !ov.CanSet() {
		return fmt.Errorf("cannot decode into unassignable value")
	}

	iv, err := toValue(in)
	if err != nil {
		return err
	}

	// Get a pointer to the in interface, since the pointer might be the thing
	// that implements the out interface.
	iptr := reflect.New(iv.Type())
	iptr.Elem().Set(iv)

	ovt := ov.Type()
	ivt := iv.Type()
	iptrt := iptr.Type()

	switch {
	case ivt.ConvertibleTo(ovt):
		ov.Set(iv.Convert(ovt))
	case iptrt.ConvertibleTo(ovt):
		ov.Set(iptr.Convert(ovt))
	default:
		return fmt.Errorf("cannot convert from %v to %v", ivt, ovt)
	}

	return nil
}
