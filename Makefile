K6_VERSION ?= latest
K6_DOCS_PATH ?= ./k6-docs
K6_BIN ?= ./k6
WORKFLOW ?= .github/workflows/k6-ci.yml
K6_CI_REF := $(shell grep -oE 'grafana/k6-ci/[^@[:space:]]+@[A-Za-z0-9._/-]+' $(WORKFLOW) | head -n1 | cut -d@ -f2)
LINT_BASE ?= .golangci-base.yml
LINT_CONFIG ?= .golangci.yml
LINT_PATCH ?= .golangci.patch
LINT_CONFIG_URL := https://raw.githubusercontent.com/grafana/k6-ci/$(K6_CI_REF)/.golangci.yml

.PHONY: help lint clean-lint test test-gh build prepare

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

$(LINT_BASE): $(WORKFLOW)
	curl -fsSL $(LINT_CONFIG_URL) -o $@

$(LINT_CONFIG): $(LINT_BASE) $(LINT_PATCH)
	cp $(LINT_BASE) $(LINT_CONFIG)
	git apply $(LINT_PATCH)

lint: $(LINT_CONFIG) ## Run linters
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$$(head -n1 $(LINT_BASE) | tr -d '# ') \
	  run --config=$(LINT_CONFIG) ./...
	cd docs && go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$$(head -n1 ../$(LINT_BASE) | tr -d '# ') \
	  run --config=../$(LINT_CONFIG) ./...
	xk6 lint --preset official

clean-lint: ## Remove generated lint configuration
	rm -f $(LINT_BASE) $(LINT_CONFIG)

test: test-gh ## Run tests
	RUN_SMOKE_E2E=1 K6_BIN=$(K6_BIN) go test -race -count=1 ./...
	cd docs && go test -race -count=1 ./...

test-gh: ## Run GitHub Actions script tests
	@for t in .github/scripts/*_test.sh; do echo "=== $$t ==="; bash "$$t" || exit 1; done

build: ## Build k6 with this extension
	xk6 build --with github.com/grafana/xk6-docs=.

prepare: ## Prepare docs bundle
	go run ./cmd/prepare --k6-version=$(K6_VERSION) --k6-docs-path=$(K6_DOCS_PATH)

.DEFAULT_GOAL := help
