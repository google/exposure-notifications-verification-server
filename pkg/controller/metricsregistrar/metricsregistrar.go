// Copyright 2021 the Exposure Notifications Verification Server authors
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

//go:generate go run github.com/google/exposure-notifications-server/tools/gen-metrics-registrar -module-name=github.com/google/exposure-notifications-verification-server -module-root=../../.. -pkg=metricsregistrar -dest=./all_metrics.go

// Package metricsregistrar implements metrics registration on load.
package metricsregistrar

import (
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

// Controller is a controller for the metrics registration service.
type Controller struct {
	config *config.MetricsRegistrarConfig
	h      *render.Renderer
}

// New creates a new controller.
func New(cfg *config.MetricsRegistrarConfig, h *render.Renderer) *Controller {
	return &Controller{
		config: cfg,
		h:      h,
	}
}
