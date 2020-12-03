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

package issueapi

import (
	"errors"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"

	"go.opencensus.io/tag"
)

type issueResult struct {
	httpCode    int
	errorReturn *api.ErrorReturn
	obsBlame    tag.Mutator
	obsResult   tag.Mutator
}

func (c *Controller) HandleIssue() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c.config.IsMaintenanceMode() {
			c.h.RenderJSON(w, http.StatusTooManyRequests,
				api.Errorf("server is read-only for maintenance").WithCode(api.ErrMaintenanceMode))
			return
		}

		ctx := observability.WithBuildInfo(r.Context())

		result := &issueResult{
			httpCode:  http.StatusOK,
			obsBlame:  observability.BlameNone,
			obsResult: observability.ResultOK(),
		}
		defer recordObservability(&ctx, result)

		var request api.IssueCodeRequest
		if err := controller.BindJSON(w, r, &request); err != nil {
			result.obsBlame = observability.BlameClient
			result.obsResult = observability.ResultError("FAILED_TO_PARSE_JSON_REQUEST")
			c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err))
			return
		}

		authApp, user, err := c.getAuthorizationFromContext(r)
		if err != nil {
			result.obsBlame = observability.BlameClient
			result.obsResult = observability.ResultError("MISSING_AUTHORIZED_APP")
			c.h.RenderJSON(w, http.StatusUnauthorized, api.Error(err))
			return
		}

		var realm *database.Realm
		if authApp != nil {
			realm, err = authApp.Realm(c.db)
			if err != nil {
				result.obsBlame = observability.BlameClient
				result.obsResult = observability.ResultError("UNAUTHORIZED")
				c.h.RenderJSON(w, http.StatusUnauthorized, nil)
				return
			}
		} else {
			// if it's a user logged in, we can pull realm from the context.
			realm = controller.RealmFromContext(ctx)
		}
		if realm == nil {
			result.obsBlame = observability.BlameServer
			result.obsResult = observability.ResultError("MISSING_REALM")
			c.h.RenderJSON(w, http.StatusBadRequest, api.Errorf("missing realm"))
			return
		}

		// Add realm so that metrics are groupable on a per-realm basis.
		ctx = observability.WithRealmID(ctx, realm.ID)

		result, resp := c.issue(ctx, &request, realm, user, authApp)
		if result.errorReturn != nil {
			if result.httpCode == http.StatusInternalServerError {
				controller.InternalError(w, r, c.h, errors.New(result.errorReturn.Error))
				return
			}
			c.h.RenderJSON(w, result.httpCode, result.errorReturn)
			return
		}

		c.h.RenderJSON(w, http.StatusOK, resp)
	})
}
