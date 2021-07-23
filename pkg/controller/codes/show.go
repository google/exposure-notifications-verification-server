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

package codes

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/gorilla/mux"
)

// HandleShow renders html for the status of an issued verification code
func (c *Controller) HandleShow() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		vars := mux.Vars(r)

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}

		membership := controller.MembershipFromContext(ctx)
		if membership == nil {
			controller.MissingMembership(w, r, c.h)
			return
		}
		if !membership.Can(rbac.CodeRead) {
			controller.Unauthorized(w, r, c.h)
			return
		}

		currentRealm := membership.Realm
		currentUser := membership.User

		code, _, apiErr := c.checkCodeStatus(r, vars["uuid"])
		if apiErr != nil {
			var code database.VerificationCode
			code.UUID = vars["uuid"]
			code.AddError("uuid", apiErr.Error)

			if err := c.renderStatus(ctx, w, currentRealm, currentUser, &code); err != nil {
				controller.InternalError(w, r, c.h, err)
				return
			}
			return
		}

		// The code was valid, but it's not in the correct UUID format. This ensures
		// two different links don't point to the same resource.
		if code.UUID != vars["uuid"] {
			u := fmt.Sprintf("/codes/%s", code.UUID)
			http.Redirect(w, r, u, http.StatusSeeOther)
			return
		}

		retCode, err := c.responseCode(ctx, code)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		c.renderShow(ctx, w, retCode)
	})
}

func (c *Controller) responseCode(ctx context.Context, code *database.VerificationCode) (*Code, error) {
	if code == nil {
		return nil, fmt.Errorf("code is nil")
	}

	// Build initial code.
	var retCode Code
	retCode.UUID = code.UUID
	retCode.TestType = strings.Title(code.TestType)

	// Get realm from context.
	_, _, realm, err := c.getAuthorizationFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Return "best" status message but looking up issuer.
	if code.IssuingUserID != 0 {
		user, err := realm.FindUser(c.db, code.IssuingUserID)
		if err != nil {
			if !database.IsNotFound(err) {
				return nil, err
			}

			// User has since been deleted.
			user = &database.User{Name: "Unknown user"}
		}

		retCode.IssuerType = "Issuing user"
		retCode.Issuer = user.Name
	} else if code.IssuingAppID != 0 {
		authApp, err := realm.FindAuthorizedApp(c.db, code.IssuingAppID)
		if err != nil {
			if !database.IsNotFound(err) {
				return nil, err
			}

			// App has since been deleted
			authApp = &database.AuthorizedApp{Name: "Unknown app"}
		}

		retCode.IssuerType = "Issuing app"
		retCode.Issuer = authApp.Name
	}

	retCode.Claimed = code.Claimed
	if code.Claimed {
		retCode.Status = "Claimed by user"
	} else {
		retCode.Status = "Not yet claimed"
	}

	if !code.IsExpired() && !code.Claimed {
		retCode.Expires = code.ExpiresAt.UTC().Unix()
		retCode.LongExpires = code.LongExpiresAt.UTC().Unix()
		retCode.HasLongExpires = retCode.LongExpires > retCode.Expires
	}

	return &retCode, nil
}

type Code struct {
	UUID           string `json:"uuid"`
	Claimed        bool   `json:"claimed"`
	Status         string `json:"status"`
	TestType       string `json:"testType"`
	IssuerType     string `json:"issuerType"`
	Issuer         string `json:"issuer"`
	Expires        int64  `json:"expires"`
	LongExpires    int64  `json:"longExpires"`
	HasLongExpires bool   `json:"hasLongExpires"`
}

func (c *Controller) renderShow(ctx context.Context, w http.ResponseWriter, code *Code) {
	m := controller.TemplateMapFromContext(ctx)
	m.Title("Verification code status")
	m["code"] = code
	c.h.RenderHTML(w, "codes/show", m)
}
