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

package realm

import (
	"fmt"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/config"
)

// setRealmCookie sets the realm cookie. This must be called before any body data
// is written for the request.
func setRealmCookie(w http.ResponseWriter, c *config.ServerConfig, realmID uint) {
	http.SetCookie(w, &http.Cookie{
		Name:     "realm",
		Value:    fmt.Sprintf("%v", realmID),
		Path:     "/",
		Secure:   !c.DevMode,
		SameSite: http.SameSiteStrictMode,
	})
}
