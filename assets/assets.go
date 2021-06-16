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

// Package assets defines the templates and embedded filesystems.
package assets

import (
	"embed"
	"io/fs"
	"os"

	"github.com/google/exposure-notifications-verification-server/internal/project"
)

//go:embed server server/**/*
var _serverFS embed.FS

// This gets around an inconsistency where the embed is rooted at server/, but
// the os.DirFS is rooted after server/.
var serverFS, _ = fs.Sub(_serverFS, "server")

// ServerFS returns the file system for the server assets.
func ServerFS() fs.FS {
	if project.DevMode() {
		return os.DirFS(project.Root("assets", "server"))
	}

	return serverFS
}

var serverStaticFS, _ = fs.Sub(serverFS, "static")

// ServerStaticFS returns the file system for the server static assets, rooted
// at static/.
func ServerStaticFS() fs.FS {
	if project.DevMode() {
		return os.DirFS(project.Root("assets", "server", "static"))
	}

	return serverStaticFS
}

//go:embed enx-redirect/*
var enxRedirectFS embed.FS

// ENXRedirectFS returns the file system for the enx-redirect assets.
func ENXRedirectFS() fs.FS {
	if project.DevMode() {
		return os.DirFS(project.Root("assets", "enx-redirect"))
	}
	return enxRedirectFS
}
