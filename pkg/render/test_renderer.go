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
	"context"
	"net/http"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/icsv"
	"github.com/jba/templatecheck"
)

// TestRenderer defines a test version of the renderer
type TestRenderer struct {
	*ProdRenderer // the implementation under test

	T *testing.T
}

var _ Renderer = (*TestRenderer)(nil) // ensure interface satisfied

// NewTest creates a new renderer with the given details.
func NewTest(ctx context.Context, root string, t *testing.T) (Renderer, error) {
	i, err := newProdRenderer(ctx, root, true)
	if err != nil {
		return nil, err
	}
	return &TestRenderer{ProdRenderer: i, T: t}, nil
}

func (r *TestRenderer) RenderCSV(w http.ResponseWriter, code int, filename string, data icsv.Marshaler) {
	// Not supported for test. Doesn't use templates.
}

func (r *TestRenderer) RenderEmail(tmpl string, data interface{}) ([]byte, error) {
	if r.textTemplates == nil {
		r.T.Fatal("no email templates loaded")
	}
	t := r.textTemplates.Lookup(tmpl)
	if t == nil {
		r.T.Fatalf("could not find template %q", tmpl)
	}
	err := templatecheck.CheckText(t, data)
	if err != nil {
		r.T.Fatalf("failed to render test template %v", err)
	}
	return []byte{}, nil
}

func (r *TestRenderer) RenderHTML500(w http.ResponseWriter, err error) {
	r.RenderHTMLStatus(w, http.StatusInternalServerError, "500", map[string]string{"error": err.Error()})
}
func (r *TestRenderer) RenderHTML(w http.ResponseWriter, tmpl string, data interface{}) {
	r.RenderHTMLStatus(w, http.StatusOK, tmpl, data)
}
func (r *TestRenderer) RenderHTMLStatus(w http.ResponseWriter, code int, tmpl string, data interface{}) {
	if r.templates == nil {
		r.T.Fatal("no html templates loaded")
		return
	}
	t := r.templates.Lookup(tmpl)
	if t == nil {
		r.T.Fatalf("could not find template %q", tmpl)
		return
	}
	err := templatecheck.CheckHTML(t, data)
	if err != nil {
		r.T.Fatalf("failed to render test template %v", err)
		return
	}
}

func (r *TestRenderer) RenderJSON(w http.ResponseWriter, code int, data interface{}) {
	// Not supported for test. Doesn't use templates.
}
func (r *TestRenderer) RenderJSON500(w http.ResponseWriter, err error) {
	// Not supported for test. Doesn't use templates.
}
