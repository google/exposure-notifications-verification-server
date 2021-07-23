// Copyright 2020 the Exposure Notifications Verification Server authors
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

package admin

import (
	"context"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/gorilla/mux"
)

type cacheItem struct {
	Name        string
	Description string
}

var caches = map[string]*cacheItem{
	"apps:":               {"Mobile apps", "Registered mobile apps for the redirector service"},
	"authorized_apps:":    {"API keys", "Authentication for API keys"},
	"jwks:":               {"JWKs", "JSON web key sets"},
	"memberships:":        {"Memberships", "All membership information"},
	"public_keys:":        {"Public keys", "PEM data from upstream key provider"},
	"realms:":             {"Realms", "All realm data"},
	"stats:":              {"Statistics", "API key, user, and realm statistics"},
	"token_signing_keys:": {"Token signing keys", "All token signing keys, including currently active"},
	"translations:":       {"Translations", "Realm specific tranlations for the user report webview"},
	"users:":              {"Users", "All user data"},
}

// HandleCachesIndex shows the caches page.
func (c *Controller) HandleCachesIndex() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		c.renderCachesIndex(ctx, w)
	})
}

// HandleCachesClear clears the given caches.
func (c *Controller) HandleCachesClear() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		vars := mux.Vars(r)

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}
		flash := controller.Flash(session)

		id := vars["id"]
		item, ok := caches[id]
		if !ok {
			flash.Error("Unknown cache type: %q", id)
			controller.Back(w, r, c.h)
			return
		}

		if err := c.cacher.DeletePrefix(ctx, id); err != nil {
			flash.Error("Failed to clear cache for %s: %v", item.Name, err)
			controller.Back(w, r, c.h)
			return
		}

		flash.Alert("Successfully cleared cache for %s!", item.Name)
		controller.Back(w, r, c.h)
		return
	})
}

func (c *Controller) renderCachesIndex(ctx context.Context, w http.ResponseWriter) {
	m := controller.TemplateMapFromContext(ctx)
	m.Title("Caches - System Admin")
	m["caches"] = caches
	c.h.RenderHTML(w, "admin/caches/index", m)
}
