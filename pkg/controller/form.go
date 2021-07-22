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

package controller

import (
	"fmt"
	"net/http"

	"github.com/gorilla/schema"
)

// BindForm parses and binds the HTTP form to the provided data interface using
// the gorilla schema package.
func BindForm(w http.ResponseWriter, r *http.Request, data interface{}) error {
	if err := r.ParseForm(); err != nil {
		return err
	}

	decoder := schema.NewDecoder()
	decoder.SetAliasTag("form")

	// Set ignore unknown keys so that things like the action and submit button
	// don't need to be captured. By default schema decoder is very struct.
	decoder.IgnoreUnknownKeys(true)

	if err := decoder.Decode(data, r.PostForm); err != nil {
		return fmt.Errorf("failed to decode form: %w", err)
	}
	return nil
}
