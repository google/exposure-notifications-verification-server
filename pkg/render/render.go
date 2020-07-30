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

// Package render defines rendering functionality.
package render

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"text/template"

	"github.com/google/exposure-notifications-verification-server/pkg/logging"
	"go.uber.org/zap"
)

// Renderer is responsible for rendering various content and templates like HTML
// and JSON responses.
type Renderer struct {
	// debug indicates templates should be reloaded on each invocation and real
	// error responses should be rendered. Do not enable in production.
	debug bool

	// logger is the log writer.
	logger *zap.SugaredLogger

	// rendererPool is a pool of *bytes.Buffer, used as a rendering buffer to
	// prevent partial responses being sent to clients.
	rendererPool *sync.Pool

	// templates is the actually collection of templates. templatesLoader is a
	// function for (re)loading templates. templatesLock is a mutex to prevent
	// concurrent modification of the templates field.
	templates     *template.Template
	templatesGlob string
}

// New creates a new renderer with the given details.
func New(ctx context.Context, glob string, debug bool) (*Renderer, error) {
	logger := logging.FromContext(ctx)

	r := &Renderer{
		debug:  debug,
		logger: logger,
		rendererPool: &sync.Pool{
			New: func() interface{} {
				return bytes.NewBuffer(make([]byte, 0, 1024))
			},
		},
		templatesGlob: glob,
	}

	// Load initial templates
	if err := r.loadTemplates(); err != nil {
		return nil, err
	}

	return r, nil
}

// loadTemplates loads or reloads all templates.
func (r *Renderer) loadTemplates() error {
	if r.templatesGlob == "" {
		return nil
	}

	tmpl, err := template.ParseGlob(r.templatesGlob)
	if err != nil {
		return fmt.Errorf("failed to load templates: %w", err)
	}
	r.templates = tmpl
	return nil
}
