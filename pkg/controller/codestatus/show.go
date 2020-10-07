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

// Package codestatus defines a web controller for the code status page of the verification
// server. This view allows users to view the status of previously-issued OTP codes.
package codestatus

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

func (c *Controller) HandleShow() http.Handler {
	type FormData struct {
		UUID string `form:"uuid"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		realm := controller.RealmFromContext(ctx)
		if realm == nil {
			controller.MissingRealm(w, r, c.h)
			return
		}

		currentUser := controller.UserFromContext(ctx)
		if currentUser == nil {
			controller.MissingUser(w, r, c.h)
			return
		}

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}
		flash := controller.Flash(session)

		retCode := Code{}

		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			flash.Error("Failed to process form: %v", err)
			c.renderShow(ctx, w, retCode)
			return
		}

		if form.UUID == "" {
			var code database.VerificationCode
			code.AddError("uuid", "cannot be blank")

			if err := c.renderStatus(ctx, w, realm, currentUser, &code); err != nil {
				controller.InternalError(w, r, c.h, err)
			}
			return
		}

		code, _, apiErr := c.CheckCodeStatus(r, form.UUID)
		if apiErr != nil {
			var code database.VerificationCode
			code.UUID = form.UUID
			code.AddError("uuid", apiErr.Error)

			if err := c.renderStatus(ctx, w, realm, currentUser, &code); err != nil {
				controller.InternalError(w, r, c.h, err)
			}
			return
		}

		c.responseCode(ctx, r, code, &retCode)
		c.renderShow(ctx, w, retCode)
	})
}

func (c *Controller) responseCode(ctx context.Context, r *http.Request, code *database.VerificationCode, retCode *Code) {
	retCode.UUID = code.UUID
	retCode.TestType = strings.Title(code.TestType)

	if code.IssuingUserID != 0 {
		retCode.IssuerType = "Issuing user"
		retCode.Issuer = c.getUserName(ctx, r, code.IssuingUserID)
	} else if code.IssuingAppID != 0 {
		retCode.IssuerType = "Issuing app"
		retCode.Issuer = c.getAuthAppName(ctx, r, code.IssuingAppID)
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
}

func (c *Controller) getUserName(ctx context.Context, r *http.Request, id uint) (userName string) {
	userName = "Unknown user"
	_, user, err := c.getAuthorizationFromContext(r)
	if err != nil {
		return
	}

	// The current user is the issuer
	if user != nil && user.ID == id {
		return user.Name
	}

	// The current user is admin, issuer is someone else

	realm := controller.RealmFromContext(ctx)
	if realm == nil {
		return
	}

	user, err = realm.FindUser(c.db, id)
	if err != nil {
		return
	}

	return user.Name
}

func (c *Controller) getAuthAppName(ctx context.Context, r *http.Request, id uint) (appName string) {
	appName = "Unknown app"
	authApp, _, err := c.getAuthorizationFromContext(r)
	if err != nil {
		return
	}

	// The current app is the issuer
	if authApp != nil && authApp.ID == id {
		return authApp.Name
	}

	// The current app is admin, issuer is a different app

	realm := controller.RealmFromContext(ctx)
	if realm == nil {
		return
	}

	authApp, err = realm.FindAuthorizedApp(c.db, authApp.ID)
	if err != nil {
		return
	}

	return authApp.Name
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

func (c *Controller) renderShow(ctx context.Context, w http.ResponseWriter, code Code) {
	m := controller.TemplateMapFromContext(ctx)
	m["code"] = code
	c.h.RenderHTML(w, "code/show", m)
}
