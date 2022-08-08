# Copyright 2020 the Exposure Notifications Verification Server authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

GOFMT_FILES = $(shell go list -f '{{.Dir}}' ./...)
HTML_FILES = $(shell find . -name \*.html)
GO_FILES = $(shell find . -name \*.go)
MD_FILES = $(shell find . -name \*.md)
PO_FILES = $(shell find . -name \*.po)

# diff-check runs git-diff and fails if there are any changes.
diff-check:
	@FINDINGS="$$(git status -s -uall)" ; \
		if [ -n "$${FINDINGS}" ]; then \
			echo "Changed files:\n\n" ; \
			echo "$${FINDINGS}\n\n" ; \
			echo "Diffs:\n\n" ; \
			git diff ; \
			git diff --cached ; \
			exit 1 ; \
		fi
.PHONY: diff-check

generate:
	@go generate ./...
.PHONY: generate

generate-check: generate diff-check
.PHONY: generate-check

# lint uses the same linter as CI and tries to report the same results running
# locally. There is a chance that CI detects linter errors that are not found
# locally, but it should be rare.
lint:
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint
	@golangci-lint run --config .golangci.yaml
.PHONY: lint

tabcheck:
	@FINDINGS="$$(awk '/\t/ {printf "%s:%s:found tab character\n",FILENAME,FNR}' $(HTML_FILES))"; \
		if [ -n "$${FINDINGS}" ]; then \
			echo "$${FINDINGS}\n\n"; \
			exit 1; \
		fi
.PHONY: tabcheck

test:
	@go test \
		-count=1 \
		-short \
		-shuffle=on \
		-tags=google \
		-timeout=5m \
		./...
.PHONY: test

test-acc:
	@go test \
		-count=1 \
		-race \
		-shuffle=on \
		-tags=google \
		-timeout=10m \
		./... \
		-coverprofile=./coverage.out
.PHONY: test-acc

test-coverage:
	@go tool cover -func=./coverage.out
.PHONY: test-coverage

zapcheck:
	@go install github.com/sethvargo/zapw/cmd/zapw
	@zapw ./...
.PHONY: zapcheck

pofmt:
	@go run ./tools/pofmt $(PO_FILES)
.PHONY: pofmt
