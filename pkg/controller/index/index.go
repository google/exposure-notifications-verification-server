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

// Package index defines the controller for the index/landing page.
package index

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
)

type indexController struct {
	config *config.Config
}

// New creates a new index controller. The index controller is thread-safe.
func New(config *config.Config) controller.Controller {
	return &indexController{config}
}

func (ic *indexController) Execute(c *gin.Context) {
	m := controller.NewTemplateMap(ic.config, c)
	m["firebase"] = ic.config.Firebase
	c.HTML(http.StatusOK, "index", m)
}
