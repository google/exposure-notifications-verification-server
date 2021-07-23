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
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
)

func (c *Controller) HandleCheckCodeStatus() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request api.CheckCodeStatusRequest
		if err := controller.BindJSON(w, r, &request); err != nil {
			c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err))
			return
		}

		code, errCode, err := c.checkCodeStatus(r, request.UUID)
		if err != nil {
			c.h.RenderJSON(w, errCode, err)
			return
		}

		c.h.RenderJSON(w, http.StatusOK,
			&api.CheckCodeStatusResponse{
				Claimed:                code.Claimed,
				ExpiresAtTimestamp:     code.ExpiresAt.UTC().Unix(),
				LongExpiresAtTimestamp: code.LongExpiresAt.UTC().Unix(),
			})
	})
}
