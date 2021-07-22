// Copyright 2020 the Exposure Notifications Verification Server authors
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

// System represents the system and actions it has taken. It's not stored in the
// database.
var System Auditable = new(system)

type system struct{}

func (s *system) AuditID() string {
	return "system:1"
}

func (s *system) AuditDisplay() string {
	return "System"
}

// SystemTest represents the system and actions it has taken. It's not stored in the
// database.
var SystemTest Auditable = new(systemTest)

type systemTest struct{}

func (s *systemTest) AuditID() string {
	return "system_test:1"
}

func (s *systemTest) AuditDisplay() string {
	return "SystemTest"
}
