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

// Package project defines global project helpers.
package project

import (
	"path/filepath"
	"runtime"
)

var _, self, _, _ = runtime.Caller(0)

// Root returns the filepath to the root of this project.
func Root() string {
	return filepath.Join(filepath.Dir(self), "..", "..")
}
