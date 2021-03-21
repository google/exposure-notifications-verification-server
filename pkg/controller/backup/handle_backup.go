// Copyright 2021 Google LLC
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

package backup

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"go.opencensus.io/stats"
)

type Result struct {
	OK     bool     `json:"ok"`
	Errors []string `json:"errors,omitempty"`
}

func (c *Controller) HandleBackup() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		logger := logging.FromContext(ctx).Named("backup.HandleBackup")

		req, err := c.buildBackupRequest(ctx)
		if err != nil {
			logger.Errorw("failed to build request", "error", err)
			c.h.RenderJSON(w, http.StatusInternalServerError, &Result{
				OK:     false,
				Errors: []string{err.Error()},
			})
			return
		}

		ok, err := c.db.TryLock(ctx, lockName, c.config.MinTTL)
		if err != nil {
			logger.Errorw("failed to acquire lock", "error", err)
			c.h.RenderJSON(w, http.StatusInternalServerError, &Result{
				OK:     false,
				Errors: []string{err.Error()},
			})
			return
		}
		if !ok {
			logger.Debugw("skipping (too early)")
			c.h.RenderJSON(w, http.StatusOK, &Result{
				OK:     false,
				Errors: []string{"too early"},
			})
			return
		}

		if err := c.executeBackup(req); err != nil {
			logger.Errorw("failed to execute backup", "error", err)
			c.h.RenderJSON(w, http.StatusInternalServerError, &Result{
				OK:     false,
				Errors: []string{err.Error()},
			})
			return
		}

		stats.Record(ctx, mSuccess.M(1))
		c.h.RenderJSON(w, http.StatusOK, &Result{
			OK: true,
		})
	})
}

func (c *Controller) buildBackupRequest(ctx context.Context) (*http.Request, error) {
	u, err := url.Parse(c.config.DatabaseInstanceURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database instance url: %w", err)
	}
	u.Path = path.Join(u.Path, "export")

	var b bytes.Buffer
	if err := json.NewEncoder(&b).Encode(&backupRequest{
		ExportContext: &exportContext{
			FileType:  "SQL",
			URI:       fmt.Sprintf("gs://%s/database/%s", c.config.Bucket, c.config.DatabaseName),
			Databases: []string{c.config.DatabaseName},

			// Specifically disable offloading because we want this request to run
			// in-band so we can verify the return status.
			Offload: false,
		},
	}); err != nil {
		return nil, fmt.Errorf("failed to create body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), &b)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	return req, nil
}

// executeBackup calls the backup API. This is a *blocking* operation that can
// take O(minutes) in some cases.
func (c *Controller) executeBackup(req *http.Request) error {
	client := &http.Client{
		Timeout: c.config.Timeout,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if got, want := resp.StatusCode, http.StatusOK; got != want {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unsuccessful response from backup (got %d, want %d): %s", got, want, b)
	}

	return nil
}

type backupRequest struct {
	ExportContext *exportContext `json:"exportContext"`
}

type exportContext struct {
	FileType  string   `json:"fileType"`
	URI       string   `json:"uri"`
	Databases []string `json:"databases"`
	Offload   bool     `json:"offload"`
}
