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

package jsonclient

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
)

func MakeRequest(client *http.Client, url string, input interface{}, output interface{}) error {

	data, err := json.Marshal(input)
	if err != nil {
		return err
	}

	buffer := bytes.NewBuffer(data)
	r, err := client.Post(url, "application/json", buffer)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}

	err = json.Unmarshal(body, output)
	return nil
}
