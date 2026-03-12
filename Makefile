K6_VERSION ?= latest
K6_DOCS_PATH ?= ./k6-docs
K6_BIN ?= ./k6

.PHONY: help lint test build prepare

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

lint: ## Run linters
	golangci-lint run ./...

test: ## Run tests
	RUN_SMOKE_E2E=1 K6_BIN=$(K6_BIN) go test -race -count=1 ./...

build: ## Build k6 with this extension
	xk6 build --with github.com/grafana/xk6-docs=.

prepare: ## Prepare docs bundle
	go run ./cmd/prepare --k6-version=$(K6_VERSION) --k6-docs-path=$(K6_DOCS_PATH)

.DEFAULT_GOAL := help
