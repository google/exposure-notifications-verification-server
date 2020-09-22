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

package mobileapp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

var (
	ErrNoAppIDSpecified = errors.New("OS selection requires app id")
	ErrBadSHALength     = errors.New("sha wasn't 95 characters")
	ErrBadSHAFormat     = errors.New("sha must be of the form AA:...:BB")
)

type FormData struct {
	Name  string          `form:"name"`
	OS    database.OSType `form:"os"`
	AppID string          `form:"appID"`
	SHA   string          `form:"sha"`
}

// cleanSHA cleans an input string.
func cleanSHA(input string) string {
	input = strings.ToUpper(input)
	input = strings.Trim(input, " \n\t\r")
	input = strings.ReplaceAll(input, ",", "\n")
	input = strings.ReplaceAll(input, " ", "\n")
	input = strings.ReplaceAll(input, "\t", "\n")
	input = strings.ReplaceAll(input, "\r", "\n")
	res := []string{}
	for _, str := range strings.Split(input, "\n") {
		if len(str) == 0 {
			continue
		}
		res = append(res, str)
	}
	return strings.Join(res, "\n")
}

// verifySHA verifies that we have a valid sha.
func verifySHA(input string) error {
	for _, sha := range strings.Split(input, "\n") {
		if len(sha) != 95 {
			return ErrBadSHALength
		}
		if strings.Count(sha, ":") != 31 {
			return ErrBadSHAFormat
		}
		for _, c := range strings.Split(sha, ":") {
			if len(c) != 2 {
				return ErrBadSHAFormat
			}
		}
	}
	return nil
}

// verify verifies that the form is valid.
func (f FormData) verify() error {
	if f.OS == database.OSTypeIOS {
		if len(f.AppID) == 0 {
			return ErrNoAppIDSpecified
		}
	}

	if f.OS == database.OSTypeAndroid {
		if len(f.AppID) == 0 {
			return ErrNoAppIDSpecified
		}
		if err := verifySHA(f.SHA); err != nil {
			return err
		}
	}

	return nil
}

func (c *Controller) HandleCreate() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}
		flash := controller.Flash(session)

		realm := controller.RealmFromContext(ctx)
		if realm == nil {
			controller.MissingRealm(w, r, c.h)
			return
		}

		// Requested form, stop processing.
		if r.Method == http.MethodGet {
			var app database.MobileApp
			c.renderNew(ctx, w, &app)
			return
		}

		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			app := &database.MobileApp{
				Name:  form.Name,
				OS:    form.OS,
				AppID: form.AppID,
				SHA:   form.SHA,
			}

			flash.Error("Failed to process form: %v", err)
			c.renderNew(ctx, w, app)
			return
		}
		form.SHA = cleanSHA(form.SHA)

		// Build the authorized app struct
		app := &database.MobileApp{
			Name:  form.Name,
			OS:    form.OS,
			AppID: form.AppID,
			SHA:   form.SHA,
		}

		// Verify the form.
		if err := form.verify(); err != nil {
			flash.Error("Failed to verify: %v", err)
			c.renderNew(ctx, w, app)
		}

		err := realm.CreateMobileApp(c.db, app)
		if err != nil {
			flash.Error("Failed to create mobile app: %v", err)
			c.renderNew(ctx, w, app)
			return
		}

		// Store the app name in the session so we can properly display it on
		// the next page.
		session.Values["appName"] = form.Name

		flash.Alert("Successfully created Mobile App %v", form.Name)
		http.Redirect(w, r, fmt.Sprintf("/mobileapp/%d", app.ID), http.StatusSeeOther)
	})
}

// renderNew renders the new page.
func (c *Controller) renderNew(ctxt context.Context, w http.ResponseWriter, app *database.MobileApp) {
	m := templateMap(ctxt)
	m["app"] = app
	c.h.RenderHTML(w, "mobileapp/new", m)
}
