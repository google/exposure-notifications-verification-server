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

// Package jsonclient is a simple JSON over HTTP Client.
package jsonclient

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
)

// MakeRequest uses an HTTP client to send and receive JSON based on interface{}.
func MakeRequest(client *http.Client, url string, headers http.Header, input interface{}, output interface{}) error {
	data, err := json.Marshal(input)
	if err != nil {
		return err
	}

	buffer := bytes.NewBuffer(data)

	req, err := http.NewRequest("POST", url, buffer)
	if err != nil {
		return err
	}
	req.Header = headers
	req.Header.Add("content-type", "application/json")
	r, err := client.Do(req)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}

	return json.Unmarshal(body, output)
}
