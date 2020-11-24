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

package appsync

type AppsResponse struct {
	Apps []App `json:"apps"`
}

type App struct {
	Region        string `json:"region"`
	IsEnx         bool   `json:"is_enx"`
	AndroidTarget `json:"android_target"`
}

type AndroidTarget struct {
	Namespace              string `json:"namespace"`
	PackageName            string `json:"package_name"`
	SHA256CertFingerprints string `json:"sha256_cert_fingerprints"`
}
