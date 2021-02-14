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

package project

import (
	"fmt"
	"reflect"
	"testing"
)

func TestErrorsToStrings(t *testing.T) {
	t.Parallel()

	result := ErrorsToStrings([]error{
		fmt.Errorf("oh no"),
		nil,
		nil,
		fmt.Errorf("goodbye"),
	})

	if got, want := result, []string{"oh no", "goodbye"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Expected %q to be %q", got, want)
	}
}
