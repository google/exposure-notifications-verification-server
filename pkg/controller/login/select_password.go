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

// Package login defines the controller for the login page.
package login

import (
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
)

type Requirements struct {
	Length    int
	Uppercase int
	Lowercase int
	Number    int
	Special   int
}

func (c *Controller) HandleSelectPassword() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// TODO: get these values from env
		reqs := Requirements{
			Length:    8,
			Uppercase: 1,
			Number:    1,
			Special:   1,
		}

		m := controller.TemplateMapFromContext(ctx)
		m["firebase"] = c.config.Firebase
		m["requirements"] = reqs
		c.h.RenderHTML(w, "login/select-password", m)
	})
}
