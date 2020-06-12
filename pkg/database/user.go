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

import "time"

// User represents immutable qualities of a user.
// Authentication is delegated to Firebase Auth, but the underlying data store
// needs to put specific users on the allow list.
type User interface {
	Email() string
	Admin() bool
	Disabled() bool
	LastRevokeCheck() time.Time
}
