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
	"fmt"
	"net/http"
	"strings"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
)

func (c *Controller) HandleIndex() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//ctx := r.Context()

		idx := strings.Index(r.RequestURI, "/v?")
		if idx < 0 {
			controller.InternalError(w, r, c.h, fmt.Errorf("don't do that."))
			return
		}
		keep := r.RequestURI[idx:]
		sendTo := fmt.Sprintf("ens:/%s", keep)

		http.Redirect(w, r, sendTo, http.StatusSeeOther)
	})
}
