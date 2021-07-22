// Copyright 2021 the Exposure Notifications Verification Server authors
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
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	htmltemplate "html/template"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	texttemplate "text/template"

	"github.com/google/exposure-notifications-verification-server/internal/buildinfo"
)

const sriPrefix = "sha384-"

var (
	cssIncludeTmpl = texttemplate.Must(texttemplate.New(`cssIncludeTmpl`).Parse(strings.TrimSpace(`
{{ range . -}}
<link rel="stylesheet" href="/{{.Path}}?{{.BuildID}}"
  integrity="{{.SRI}}" crossorigin="anonymous">
{{ end }}
`)))

	cssIncludeTagCache htmltemplate.HTML
)

var (
	jsIncludeTmpl = texttemplate.Must(texttemplate.New(`jsIncludeTmpl`).Parse(strings.TrimSpace(`
{{ range . -}}
<script defer src="/{{.Path}}?{{.BuildID}}"
  integrity="{{.SRI}}" crossorigin="anonymous"></script>
{{ end }}
`)))
	jsIncludeTagCache htmltemplate.HTML
)

// asset represents a javascript or css asset.
type asset struct {
	// Path is the virtual path, relative to the URL root.
	Path string

	// SRI is the sha384 resource integrity.
	SRI string
}

// BuildID is a helper to get the current ID (for cache busting).
func (a *asset) BuildID() string {
	return buildinfo.BuildID
}

// assetIncludeTag searches the fs for all assets of the given search type and
// renders the template. In non-dev mode, the results are cached on the first
// invocation.
func assetIncludeTag(fsys fs.FS, search string, tmpl *texttemplate.Template, cache *htmltemplate.HTML, devMode bool) func() (htmltemplate.HTML, error) {
	var mu sync.Mutex

	return func() (htmltemplate.HTML, error) {
		if !devMode {
			mu.Lock()
			defer mu.Unlock()
			if *cache != "" {
				return *cache, nil
			}
		}

		entries, err := fs.ReadDir(fsys, search)
		if err != nil {
			return "", fmt.Errorf("failed to read entries: %w", err)
		}

		list := make([]*asset, 0, len(entries))
		for _, entry := range entries {
			name := entry.Name()
			pth := filepath.Join(search, name)

			f, err := fsys.Open(pth)
			if err != nil {
				return "", fmt.Errorf("failed to open %s: %w", name, err)
			}

			integrity, err := generateSRI(f)
			if err != nil {
				return "", fmt.Errorf("failed to generate SRI for %s: %w", name, err)
			}

			list = append(list, &asset{
				Path: pth,
				SRI:  integrity,
			})
		}

		var b bytes.Buffer
		if err := tmpl.Execute(&b, list); err != nil {
			return "", fmt.Errorf("failed to render %s asset: %w", search, err)
		}
		result := htmltemplate.HTML(b.String())

		if !devMode {
			*cache = result
		}

		return result, nil
	}
}

// generateSRI is a helper that generates an SRI from the given reader. It
// closes the given reader.
func generateSRI(r io.ReadCloser) (string, error) {
	defer r.Close()

	h := sha512.New384()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return sriPrefix + base64.RawStdEncoding.EncodeToString(h.Sum(nil)), nil
}
