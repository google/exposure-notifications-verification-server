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
	"reflect"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/project"
)

func TestCacherFor(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	cases := []struct {
		name   string
		config *Config
		exp    reflect.Type
		err    bool
	}{
		{
			name:   "noop",
			config: &Config{Type: TypeNoop},
			exp:    reflect.TypeOf(&noop{}),
		},
		{
			name:   "in_memory",
			config: &Config{Type: TypeInMemory},
			exp:    reflect.TypeOf(&inMemory{}),
		},
		{
			name:   "redis",
			config: &Config{Type: TypeRedis},
			exp:    reflect.TypeOf(&redisCacher{}),
		},
		{
			name:   "unknown",
			config: &Config{Type: "BANANA"},
			err:    true,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result, err := CacherFor(ctx, tc.config, nil)
			if (err != nil) != tc.err {
				t.Fatal(err)
			}

			if got, want := reflect.TypeOf(result), tc.exp; got != want {
				t.Errorf("expected %T to be %T", got, want)
			}
		})
	}
}
