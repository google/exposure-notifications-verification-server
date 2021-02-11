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
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	// Max request size of 64KB. None of the current API requests for this
	// server are near that limit. Prevents us from unnecessarily parsing JSON
	// payloads that are much large than we anticipate.
	maxBodyBytes = 64_000
)

// BindJSON provides a common implementation of JSON unmarshaling with well defined error handling.
func BindJSON(w http.ResponseWriter, r *http.Request, data interface{}) error {
	if !IsJSONContentType(r) {
		return fmt.Errorf("content-type is not application/json")
	}

	defer r.Body.Close()
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()

	if err := d.Decode(&data); err != nil {
		var syntaxErr *json.SyntaxError
		var unmarshalError *json.UnmarshalTypeError
		switch {
		case errors.As(err, &syntaxErr):
			return fmt.Errorf("malformed json at position %d", syntaxErr.Offset)
		case errors.Is(err, io.ErrUnexpectedEOF):
			return fmt.Errorf("malformed json")
		case errors.As(err, &unmarshalError):
			return fmt.Errorf("invalid value %q at position %d", unmarshalError.Field, unmarshalError.Offset)
		case strings.HasPrefix(err.Error(), "json: unknown field"):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			return fmt.Errorf("unknown field %q", fieldName)
		case errors.Is(err, io.EOF):
			return fmt.Errorf("body must not be empty")
		case err.Error() == "http: request body too large":
			return err
		default:
			return fmt.Errorf("failed to decode json: %w", err)
		}
	}
	if d.More() {
		return fmt.Errorf("body must contain only one JSON object")
	}

	return nil
}
