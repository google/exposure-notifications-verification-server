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
	"html"
	"net/http"
)

// RenderHTML calls RenderHTMLStatus with a http.StatusOK (200).
func (r *Renderer) RenderHTML(w http.ResponseWriter, tmpl string, data interface{}) {
	r.RenderHTMLStatus(w, http.StatusOK, tmpl, data)
}

// RenderHTMLStatus renders the given HTML template by name. It attempts to
// gracefully handle any rendering errors to avoid partial responses sent to the
// response by writing to a buffer first, then flushing the buffer to the
// response.
//
// If template rendering fails, a generic 500 page is returned. In dev mode, the
// error is included on the page. If flushing the buffer to the response fails,
// an error is logged, but no recovery is attempted.
//
// The buffers are fetched via a sync.Pool to reduce allocations and improve
// performance.
func (r *Renderer) RenderHTMLStatus(w http.ResponseWriter, code int, tmpl string, data interface{}) {
	if r.templates == nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, htmlErrTmpl, "No templates are defined")
		return
	}

	if r.debug {
		if err := r.loadTemplates(); err != nil {
			msg := html.EscapeString(err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, htmlErrTmpl, msg)
			return
		}
	}

	// Acquire a renderer
	b := r.rendererPool.Get().(*bytes.Buffer)
	defer r.rendererPool.Put(b)

	// Render into the renderer
	if err := r.templates.ExecuteTemplate(b, tmpl, data); err != nil {
		msg := "An internal error occurred."
		if r.debug {
			msg = err.Error()
		}
		msg = html.EscapeString(msg)

		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, htmlErrTmpl, msg)
		return
	}

	// Rendering worked, flush to the response
	w.WriteHeader(code)
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	if _, err := b.WriteTo(w); err != nil {
		// We couldn't write the buffer. We can't change the response header or
		// content type if we got this far, so the best option we have is to log the
		// error.
		r.logger.Errorw("failed to write html to response", "error", err)
	}
}

// htmlErrTmpl is the template to use when returning an HTML error. It is
// rendered using Printf, not html/template, so values must be escaped by the
// caller.
const htmlErrTmpl = `
<html>
  <head>
    <title>Internal server error</title>
  </head>

  <body>
    <h1>Internal server error</h1>
		<p>%s</p>
  </body>
</html>
`
