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

package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/flash"
)

// TemplateMap is a typemap for the gin template.
type TemplateMap map[string]interface{}

func NewTemplateMap(c *config.Config, g *gin.Context) TemplateMap {
	m := TemplateMap{}
	m["flash"] = flash.FromContext(g)
	m["server"] = c.ServerName
	m["title"] = c.ServerName
	return m
}

func NewTemplateMapFromSession(c *config.Config, g *gin.Context) TemplateMap {
	m := TemplateMap{}
	m["flash"] = flash.FromContext(g)
	m["server"] = c.ServerName
	m["title"] = c.ServerName
	return m
}
