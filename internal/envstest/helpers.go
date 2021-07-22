// Copyright 2021 the Exposure Notifications Verification Server authors
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

package envstest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// BuildFormRequest builds an http request and http response recorder for the
// given form values (expressed as url.Values). It sets the proper headers and
// response types to post as a form and expect HTML in return.
func BuildFormRequest(ctx context.Context, tb testing.TB, meth, pth string, v *url.Values) (*httptest.ResponseRecorder, *http.Request) {
	tb.Helper()

	var body io.Reader
	if v != nil {
		body = strings.NewReader(v.Encode())
	}

	req, err := http.NewRequestWithContext(ctx, meth, pth, body)
	if err != nil {
		tb.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Accept", "text/html")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", "/back")
	return httptest.NewRecorder(), req
}

// BuildJSONRequest builds an http request and http response recorder for the
// given payload. marshaled as JSON. It sets the proper headers and response
// types.
func BuildJSONRequest(ctx context.Context, tb testing.TB, meth, pth string, v interface{}) (*httptest.ResponseRecorder, *http.Request) {
	tb.Helper()

	var body bytes.Buffer
	if v != nil {
		if err := json.NewEncoder(&body).Encode(v); err != nil {
			tb.Fatalf("failed to marshal as json: %v", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, meth, pth, &body)
	if err != nil {
		tb.Fatal(err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Referer", "/back")
	return httptest.NewRecorder(), req
}
