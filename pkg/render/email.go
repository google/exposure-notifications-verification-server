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
	"fmt"

	"github.com/microcosm-cc/bluemonday"
)

// RenderEmail renders the given email HTML template by name. It attempts to
// gracefully handle any rendering errors to avoid partial responses sent to the
// response by writing to a buffer first, then flushing the buffer to the
// response.
//
// The buffers are fetched via a sync.Pool to reduce allocations and improve
// performance.
func (r *ProdRenderer) RenderEmail(tmpl string, data interface{}) ([]byte, error) {
	if r.debug {
		if err := r.loadTemplates(); err != nil {
			return nil, fmt.Errorf("error loading templates %v", err)
		}
	}

	// Acquire a renderer
	b := r.rendererPool.Get().(*bytes.Buffer)
	b.Reset()
	defer r.rendererPool.Put(b)

	// Render into the renderer
	if err := r.executeTextTemplate(b, tmpl, data); err != nil {
		return nil, fmt.Errorf("error executing email template %v", err)
	}
	return bluemonday.UGCPolicy().SanitizeBytes(b.Bytes()), nil
}
