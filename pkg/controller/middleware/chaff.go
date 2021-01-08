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

package middleware

import (
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/gorilla/mux"
	"github.com/mikehelmick/go-chaff"
)

// ChaffHeader is the chaff header key.
const ChaffHeader = "X-Chaff"

// ChaffHeaderDetector returns a chaff header detector.
func ChaffHeaderDetector() chaff.Detector {
	return chaff.HeaderDetector(ChaffHeader)
}

// ProcessChaff injects the chaff processing middleware. If chaff requests send
// a value of "daily" (case-insensitive), they will be counted toward the
// realm's total active users and return a chaff response. Any other values will
// only return a chaff response.
//
// This must come after RequireAPIKey.
func ProcessChaff(db *database.Database, t *chaff.Tracker, det chaff.Detector) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return t.HandleTrack(det, next)
	}
}
