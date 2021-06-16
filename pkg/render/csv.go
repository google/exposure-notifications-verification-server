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

package render

import (
	"fmt"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/internal/icsv"
)

// RenderCSV renders the input as a CSV. It attempts to gracefully handle
// any rendering errors to avoid partial responses sent to the response by
// writing to a buffer first, then flushing the buffer to the response.
func (r *Renderer) RenderCSV(w http.ResponseWriter, code int, filename string, data icsv.Marshaler) {
	// Avoid marshaling nil data.
	if data == nil {
		w.Header().Set("Content-Type", "text/csv")
		w.WriteHeader(code)
	}

	// Create CSV.
	b, err := data.MarshalCSV()
	if err != nil {
		r.logger.Errorw("failed to marshal csv", "error", err)

		msg := "An internal error occurred."
		if r.debug {
			msg = err.Error()
		}

		w.Header().Set("Content-Type", "text/csv")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "%s", msg)
		return
	}

	// Ensure there's a filename.
	if filename == "" {
		filename = "data.csv"
	}

	// Rendering worked, flush to the response. Force as a download.
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment;filename=%s", filename))
	w.WriteHeader(code)
	if _, err := w.Write(b); err != nil {
		// We couldn't write the buffer. We can't change the response header or
		// content type if we got this far, so the best option we have is to log the
		// error.
		r.logger.Errorw("failed to write csv to response", "error", err)
	}
}
