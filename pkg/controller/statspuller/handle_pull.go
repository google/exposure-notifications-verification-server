// Copyright 2021 Google LLC
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

package statspuller

import (
	"net/http"

	"github.com/google/exposure-notifications-server/pkg/logging"
)

// HandlePullStats pulls key-server statistics.
func (c *Controller) HandlePullStats() http.Handler {
	type Result struct {
		OK     bool    `json:"ok"`
		Errors []error `json:"errors,omitempty"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		logger := logging.FromContext(ctx).Named("rotation.HandlePullStats")
		logger.Debug("no-op stats pull") // TODO(whaught): remove this and put in logic

		c.h.RenderJSON(w, http.StatusOK, &Result{
			OK: true,
		})
	})
}
