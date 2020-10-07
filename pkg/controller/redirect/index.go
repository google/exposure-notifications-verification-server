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

package redirect

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
)

func (c *Controller) HandleIndex() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		path := strings.ToLower(r.URL.RequestURI())
		host := strings.ToLower(r.Host)
		// Strip of the port if that was passed along in the host header.
		if i := strings.Index(host, ":"); i > 0 {
			host = host[0:i]
		}

		for hostname, region := range c.hostnameToRegion {
			if host == hostname {
				sendTo := buildURL(path, region)
				http.Redirect(w, r, sendTo, http.StatusSeeOther)
				return
			}
		}

		c.logger.Warnw("unknown host", "host", host)
		ctx := r.Context()
		m := controller.TemplateMapFromContext(ctx)
		m["requestURI"] = (&url.URL{
			Scheme: "https",
			Host:   host,
			Path:   strings.TrimPrefix(path, "/"),
		}).String()
		c.h.RenderHTMLStatus(w, http.StatusNotFound, "404", m)
	})
}

// buildURL returns the ens:// URL for the given path and region.
func buildURL(path, region string) string {
	u := &url.URL{
		Scheme: "ens",
		Path:   strings.TrimPrefix(path, "/"),
	}
	q := u.Query()
	q.Set("r", region)
	u.RawQuery = q.Encode()

	return u.String()
}
