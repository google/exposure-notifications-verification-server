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

package flash

import (
	"reflect"
	"testing"
)

func TestNew(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		values map[interface{}]interface{}
	}{
		{
			name:   "nil_values",
			values: nil,
		},
		{
			name: "given_values",
			values: map[interface{}]interface{}{
				"a": "b",
			},
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := New(tc.values)
			if f.values == nil {
				t.Fatal("expected not-nil values")
			}

			if tc.values != nil {
				if got, want := f.values, tc.values; !reflect.DeepEqual(got, want) {
					t.Errorf("expected %v to be %v", got, want)
				}
			}
		})
	}
}

func TestFlash_Error(t *testing.T) {
	t.Parallel()

	f := New(nil)
	f.Error("hello %s", "world")

	if got, want := f.values[flashKeyError], map[string]struct{}{"hello world": {}}; !reflect.DeepEqual(got, want) {
		t.Errorf("expected %v to be %v", got, want)
	}
}

func TestFlash_Errors(t *testing.T) {
	t.Parallel()

	f := New(nil)
	f.Error("hello")
	f.Error("goodbye")

	if got, want := f.Errors(), []string{"goodbye", "hello"}; !reflect.DeepEqual(got, want) {
		t.Errorf("expected %v to be %v", got, want)
	}
	if f.values[flashKeyError] != nil {
		t.Errorf("expected empty map key: %#v", f.values[flashKeyError])
	}
}

func TestFlash_Warning(t *testing.T) {
	t.Parallel()

	f := New(nil)
	f.Warning("hello %s", "world")

	if got, want := f.values[flashKeyWarning], map[string]struct{}{"hello world": {}}; !reflect.DeepEqual(got, want) {
		t.Errorf("expected %v to be %v", got, want)
	}
}

func TestFlash_Warnings(t *testing.T) {
	t.Parallel()

	f := New(nil)
	f.Warning("hello")
	f.Warning("goodbye")

	if got, want := f.Warnings(), []string{"goodbye", "hello"}; !reflect.DeepEqual(got, want) {
		t.Errorf("expected %v to be %v", got, want)
	}
	if f.values[flashKeyWarning] != nil {
		t.Errorf("expected empty map key: %#v", f.values[flashKeyWarning])
	}
}

func TestFlash_Alert(t *testing.T) {
	t.Parallel()

	f := New(nil)
	f.Alert("hello %s", "world")

	if got, want := f.values[flashKeyAlert], map[string]struct{}{"hello world": {}}; !reflect.DeepEqual(got, want) {
		t.Errorf("expected %v to be %v", got, want)
	}
}

func TestFlash_Alerts(t *testing.T) {
	t.Parallel()

	f := New(nil)
	f.Alert("hello")
	f.Alert("goodbye")

	if got, want := f.Alerts(), []string{"goodbye", "hello"}; !reflect.DeepEqual(got, want) {
		t.Errorf("expected %v to be %v", got, want)
	}
	if f.values[flashKeyAlert] != nil {
		t.Errorf("expected empty map key: %#v", f.values[flashKeyAlert])
	}
}

func TestFlash_Clear(t *testing.T) {
	t.Parallel()

	f := New(nil)
	f.Error("hello")
	f.Warning("goodbye")
	f.Alert("welcome")

	f.Clear()

	if f.values[flashKeyError] != nil {
		t.Errorf("expected empty error map key: %#v", f.values[flashKeyError])
	}
	if f.values[flashKeyWarning] != nil {
		t.Errorf("expected empty warning map key: %#v", f.values[flashKeyWarning])
	}
	if f.values[flashKeyAlert] != nil {
		t.Errorf("expected empty alert map key: %#v", f.values[flashKeyAlert])
	}
}

func TestFlash_add(t *testing.T) {
	t.Run("map", func(t *testing.T) {
		t.Parallel()

		f := New(nil)
		f.values[flashKeyError] = map[string]struct{}{"b": {}, "c": {}}
		f.add(flashKeyError, "a")

		if got, want := f.values[flashKeyError], map[string]struct{}{"a": {}, "b": {}, "c": {}}; !reflect.DeepEqual(got, want) {
			t.Errorf("expected %v to be %v", got, want)
		}
	})

	// Legacy implementation for when storage was []string.
	// TODO(sethvargo): remove slice handling in 1.1.0+.
	t.Run("slice", func(t *testing.T) {
		t.Parallel()

		f := New(nil)
		f.values[flashKeyError] = []string{"b", "c"}
		f.add(flashKeyError, "a")

		if got, want := f.values[flashKeyError], map[string]struct{}{"a": {}, "b": {}, "c": {}}; !reflect.DeepEqual(got, want) {
			t.Errorf("expected %v to be %v", got, want)
		}
	})
}

func TestFlash_get(t *testing.T) {
	t.Run("map", func(t *testing.T) {
		t.Parallel()

		f := New(nil)
		f.values[flashKeyError] = map[string]struct{}{"b": {}, "c": {}, "a": {}}

		if got, want := f.get(flashKeyError), []string{"a", "b", "c"}; !reflect.DeepEqual(got, want) {
			t.Errorf("expected %v to be %v", got, want)
		}
	})

	// Legacy implementation for when storage was []string.
	// TODO(sethvargo): remove slice handling in 1.1.0+.
	t.Run("slice", func(t *testing.T) {
		t.Parallel()

		f := New(nil)
		f.values[flashKeyError] = []string{"b", "c", "a"}

		if got, want := f.get(flashKeyError), []string{"b", "c", "a"}; !reflect.DeepEqual(got, want) {
			t.Errorf("expected %v to be %v", got, want)
		}
	})
}
