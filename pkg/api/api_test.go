// Copyright 2021 Google LLC
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

package api

import (
	"fmt"
	"testing"
)

func TestInternalError(t *testing.T) {
	t.Parallel()

	result := InternalError()

	if got, want := result.Error, "internal error"; got != want {
		t.Errorf("Expected %q to be %q", got, want)
	}
	if got, want := result.ErrorCode, "internal_server_error"; got != want {
		t.Errorf("Expected %q to be %q", got, want)
	}
	if got, want := result.ErrorCodeLegacy, "internal_server_error"; got != want {
		t.Errorf("Expected %q to be %q", got, want)
	}
}

func TestErrorf(t *testing.T) {
	t.Parallel()

	result := Errorf("hello %s", "world")

	if got, want := result.Error, "hello world"; got != want {
		t.Errorf("Expected %q to be %q", got, want)
	}
	if got, want := result.ErrorCode, ""; got != want {
		t.Errorf("Expected %q to be %q", got, want)
	}
}

func TestError(t *testing.T) {
	t.Parallel()

	{
		result := Error(fmt.Errorf("oops"))

		if got, want := result.Error, "oops"; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
		if got, want := result.ErrorCode, ""; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
	}

	{
		result := Error(nil)
		if result != nil {
			t.Errorf("expected %v to be nil", result)
		}
	}
}

func TestPadding_MarshalJSON(t *testing.T) {
	t.Parallel()

	var p Padding
	b, err := p.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if len(b) < 1024 {
		t.Errorf("expected at least 1024 bytes, got %d", len(b))
	}

	if err := p.UnmarshalJSON(b); err != nil {
		t.Fatal(err)
	}

	if got, want := fmt.Sprintf(`"%s"`, p), string(b); got != want {
		t.Errorf("expected %q to be %q", p, b)
	}
}
