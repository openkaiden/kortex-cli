# Copyright (C) 2026 Red Hat, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# SPDX-License-Identifier: Apache-2.0

.PHONY: help build test test-integration test-coverage fmt vet clean install check-fmt check-vet ci-checks all

# Binary name
BINARY_NAME=kdn
# Add .exe extension on Windows
ifeq ($(OS),Windows_NT)
    BINARY_EXT=.exe
else
    BINARY_EXT=
endif
# Build output directory
BUILD_DIR=.
# Go command
GO=go
# Go files
GOFILES=$(shell find . -type f -name '*.go' -not -path "./vendor/*")

# Default target
all: build

help: ## Display this help message
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {sub("\\\\n",sprintf("\n%22c"," "), $$2);printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the kdn binary
build:
	@echo "Building $(BINARY_NAME)$(BINARY_EXT)..."
	$(GO) build -o $(BUILD_DIR)/$(BINARY_NAME)$(BINARY_EXT) ./cmd/kdn

install: ## Install the binary to GOPATH/bin
	@echo "Installing $(BINARY_NAME)..."
	$(GO) install ./cmd/kdn

test: ## Run all tests
	@echo "Running tests..."
	$(GO) test -v -race ./...

test-integration: ## Run integration tests (requires Podman)
	@echo "Running integration tests..."
	$(GO) test -tags integration -timeout 30m -count=1 -v ./pkg/cmd/

test-coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	$(GO) test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

fmt: ## Format code with gofmt
	@echo "Formatting code..."
	@gofmt -w $(GOFILES)

vet: ## Run go vet
	@echo "Running go vet..."
	$(GO) vet ./...

check-fmt: ## Check if code is formatted (for CI)
	@echo "Checking code formatting..."
	@unformatted=$$(gofmt -l $(GOFILES)); \
	if [ -n "$$unformatted" ]; then \
		echo "The following files are not formatted:"; \
		echo "$$unformatted"; \
		echo "Run 'make fmt' to format the code."; \
		exit 1; \
	fi
	@echo "All files are properly formatted."

check-vet: ## Run go vet and fail on issues (for CI)
	@echo "Running go vet..."
	@$(GO) vet ./... || (echo "go vet found issues. Please fix them."; exit 1)

ci-checks: check-fmt check-vet test ## Run all CI checks
	@echo "All CI checks passed!"

clean: ## Remove build artifacts
	@echo "Cleaning build artifacts..."
	@rm -f $(BUILD_DIR)/$(BINARY_NAME)$(BINARY_EXT)
	@rm -f coverage.out coverage.html
	@rm -f *.test
	@echo "Clean complete."
