// Copyright 2021 the Exposure Notifications Verification Server authors
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

package cache

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/project"
)

func Test_noop(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	key := &Key{}

	t.Run("stopped", func(t *testing.T) {
		t.Parallel()

		cacher, err := NewNoop()
		if err != nil {
			t.Fatal(err)
		}
		if err := cacher.Close(); err != nil {
			t.Fatal(err)
		}
		var out interface{}

		if err := cacher.Fetch(ctx, key, &out, 1*time.Second, nil); !errors.Is(err, ErrStopped) {
			t.Errorf("expected %v to be %v", err, ErrStopped)
		}
		if err := cacher.Write(ctx, key, &out, 1*time.Second); !errors.Is(err, ErrStopped) {
			t.Errorf("expected %v to be %v", err, ErrStopped)
		}
		if err := cacher.Read(ctx, key, &out); !errors.Is(err, ErrStopped) {
			t.Errorf("expected %v to be %v", err, ErrStopped)
		}
		if err := cacher.Delete(ctx, key); !errors.Is(err, ErrStopped) {
			t.Errorf("expected %v to be %v", err, ErrStopped)
		}
		if err := cacher.DeletePrefix(ctx, "foo"); !errors.Is(err, ErrStopped) {
			t.Errorf("expected %v to be %v", err, ErrStopped)
		}
		if err := cacher.Close(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("fetch", func(t *testing.T) {
		t.Parallel()

		cacher, err := NewNoop()
		if err != nil {
			t.Fatal(err)
		}
		var out interface{}

		if err := cacher.Fetch(ctx, key, &out, 1*time.Second, nil); !errors.Is(err, ErrMissingFetchFunc) {
			t.Errorf("expected %v to be %v", err, ErrMissingFetchFunc)
		}
		if err := cacher.Fetch(ctx, key, &out, 1*time.Second, func() (interface{}, error) {
			t := &testCycleStruct{}
			t.Me = t
			return t, nil
		}); err == nil {
			t.Errorf("expected error")
		}
		if err := cacher.Fetch(ctx, key, &out, 1*time.Second, func() (interface{}, error) {
			return "", fmt.Errorf("oops")
		}); err == nil {
			t.Errorf("expected error")
		}
		if err := cacher.Fetch(ctx, key, &out, 1*time.Second, func() (interface{}, error) {
			return "foo", nil
		}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("write", func(t *testing.T) {
		t.Parallel()

		cacher, err := NewNoop()
		if err != nil {
			t.Fatal(err)
		}
		var out interface{}

		if err := cacher.Write(ctx, key, &out, 1*time.Second); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("read", func(t *testing.T) {
		t.Parallel()

		cacher, err := NewNoop()
		if err != nil {
			t.Fatal(err)
		}
		var out interface{}

		if err := cacher.Read(ctx, key, &out); !errors.Is(err, ErrNotFound) {
			t.Errorf("expected %v to be %v", err, ErrNotFound)
		}
	})

	t.Run("delete", func(t *testing.T) {
		t.Parallel()

		cacher, err := NewNoop()
		if err != nil {
			t.Fatal(err)
		}

		if err := cacher.Delete(ctx, key); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("delete_prefix", func(t *testing.T) {
		t.Parallel()

		cacher, err := NewNoop()
		if err != nil {
			t.Fatal(err)
		}

		if err := cacher.DeletePrefix(ctx, "foo"); err != nil {
			t.Fatal(err)
		}
	})
}
