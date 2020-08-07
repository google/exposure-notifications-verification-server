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

package controller

import (
	"github.com/google/exposure-notifications-verification-server/pkg/controller/flash"
	"github.com/gorilla/sessions"
)

// Flash gets or creates the flash data for the provided session.
func Flash(session *sessions.Session) *flash.Flash {
	var values map[interface{}]interface{}
	if session != nil {
		values = session.Values
	}

	return flash.New(values)
}
