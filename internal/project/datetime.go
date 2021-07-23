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

package project

const (
	// RFC3339Date formats time as RFC3339 but without a time component (e.g.
	// 2020-11-24 for November 24, 20202).
	RFC3339Date = "2006-01-02"

	// RFC3339Squish is the RFC3339 datetime but all dashes and timezone
	// indicators are removed. This is useful for filenames.
	RFC3339Squish = "20060102150405"
)
