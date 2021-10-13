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

package webhooks

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestComputeSignature(t *testing.T) {
	t.Parallel()

	// The data in this test comes from a real webhook invocation. Yes, the auth
	// token below is real. Yes, it has been rotated and is no longer valid.
	expSignature := "WFIk+mfEDAoujjBhziUSmJJn3Lw="
	authToken := "1d2a" + "7f480d4e6913a72f2febbe63379d"

	vals := make(url.Values)
	vals.Add("ParentAccountSid", "ACffcbcc79af64899e720a5076b4e6b217")
	vals.Add("Payload", `{"resource_sid":"SMba2e5f029d1b1630bbf01232ff9b814c","service_sid":"SM736f35d231bd230a80c8929e50a2c24c","error_code":"30007"}`)
	vals.Add("PayloadType", "application/json")
	vals.Add("AccountSid", "AC9a5a39b6ac47a0061bb00da10efd8264")
	vals.Add("Timestamp", "2021-10-12T23:43:53.289Z")
	vals.Add("Level", "ERROR")
	vals.Add("Sid", "NO1436992f15326b2603d79db507cedba2")

	req := httptest.NewRequest(http.MethodPost, "https://example.com", strings.NewReader(vals.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")

	x, err := computeSignature(req, authToken)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := x, expSignature; got != want {
		t.Errorf("expected %q to be %q", got, want)
	}
}
