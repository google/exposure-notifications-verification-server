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
	"encoding/json"
	"fmt"
	"net/http"
)

func writeHeaders(w http.ResponseWriter, status int) {
	w.WriteHeader(status)
	w.Header().Set("Content-Type", "application/json")
}

// WriteJSON marshals the provided value as JSON and writes it has the HTTP
// response with the specified status.
func WriteJSON(w http.ResponseWriter, status int, value interface{}) {
	if value == nil {
		writeHeaders(w, status)
		return
	}

	data, err := json.Marshal(value)
	if err != nil {
		writeHeaders(w, http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("{\"error\": \"%v\"}", err.Error())))
		return
	}

	writeHeaders(w, status)
	w.Write(data)
}
