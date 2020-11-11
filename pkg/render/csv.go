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
	"bytes"
	"errors"
	"fmt"
	"net/http"

	"github.com/microcosm-cc/bluemonday"
)

// RenderCSV renders the given CSV template by name. It attempts to
// gracefully handle any rendering errors to avoid partial responses sent to the
// response by writing to a buffer first, then flushing the buffer to the
// response.
//
// The buffers are fetched via a sync.Pool to reduce allocations and improve
// performance.
func (r *Renderer) RenderCSV(w http.ResponseWriter, tmpl string, data interface{}) error {
	if r.templates == nil {
		return errors.New("no templates found")
	}

	if r.debug {
		if err := r.loadTemplates(); err != nil {
			return fmt.Errorf("error loading templates %v", err)
		}
	}

	// Acquire a renderer
	b := r.rendererPool.Get().(*bytes.Buffer)
	b.Reset()
	defer r.rendererPool.Put(b)

	// Render into the renderer
	if err := r.textTemplates.ExecuteTemplate(b, tmpl, data); err != nil {
		return fmt.Errorf("error executing csv template %v", err)
	}
	body := bluemonday.UGCPolicy().SanitizeBytes(b.Bytes())

	w.Header().Add("Content-Disposition", "")
	w.Header().Add("Content-Type", "text/CSV")
	if _, err := w.Write(body); err != nil {
		// We couldn't write the buffer. We can't change the response header or
		// content type if we got this far, so the best option we have is to log the
		// error.
		r.logger.Errorw("failed to write html to response", "error", err)
	}
	return nil
}
