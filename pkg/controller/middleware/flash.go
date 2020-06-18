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

package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/flash"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"
)

func FlashHandler(ctx context.Context) gin.HandlerFunc {
	logger := logging.FromContext(ctx)

	return func(c *gin.Context) {
		// Always mark the current flash cookie for clearing
		http.SetCookie(c.Writer, &http.Cookie{
			Name:    "flash",
			MaxAge:  -1,
			Expires: time.Unix(0, 0),
			Path:    "/",
		})

		// Start with an empty flash
		f := flash.New()

		// Attempt to load the flash data from a cookie
		cookie, err := c.Cookie("flash")
		if err != nil && !errors.Is(err, http.ErrNoCookie) {
			logger.Errorw("failed to load flash cookie", "error", err)
		}

		// Parse the cookie
		if cookie != "" {
			var err error
			f, err = flash.Load(cookie)
			if err != nil {
				logger.Errorw("failed to load flash from cookie", "error", err)
			}
		}

		// Put the flash on the context
		c.Set("flash", f)

		// Call other middlewares and do all the things
		c.Next()

		// Save the flash into a cookie for the next request
		val, err := f.Dump()
		if err != nil {
			logger.Errorw("failed to dump flash: %w", err)
		}

		if val != "" {
			// Update the cookie with the latest flash data
			http.SetCookie(c.Writer, &http.Cookie{
				Name:  "flash",
				Value: url.QueryEscape(val),
				Path:  "/",
			})
		}
	}
}
