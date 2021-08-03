.DEFAULT_GOAL = build
SHED = go run main.go
COVERPKGS = ./cache,./client,./internal/spinner,./internal/util,./lockfile,./tool

# Absolutely awesome: http://marmelab.com/blog/2016/02/29/auto-documented-makefile.html
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
.PHONY: help

setup: ## Install all dependencies
	@echo Installing dependencies
	@go mod tidy
# Self-hoisted!
	@echo Installing tool dependencies
	@$(SHED) get
	@$(SHED) run go-fish install
.PHONY: setup

build: ## Build shed
	@go build
.PHONY: build

build-snapshot: ## Create a snapshot release build
	@$(SHED) run goreleaser build --snapshot --rm-dist
.PHONY: build-snapshot

release: ## Create a new release of shed
	$(if $(version),,$(error version variable is not set))
	git tag -a v$(version) -m "v$(version)"
	git push origin v$(version)
	$(SHED) run goreleaser release --rm-dist
.PHONY: release

completions: ## Generate shell completions for distribution
	@mkdir -p completions
	@$(SHED) completions bash > completions/shed.bash
	@$(SHED) completions zsh > completions/_shed
.PHONY: completions

clean: ## Clean all build artifacts
	@rm -rf completions
	@rm -rf coverage
	@rm -rf dist
	@rm -f shed
.PHONY: clean

fmt: ## Format all go files
	@$(SHED) run goimports -w .
.PHONY: fmt

check-fmt: ## Check if any go files need to be formatted
	@./scripts/check_fmt.sh
.PHONY: check-fmt

lint: ## Lint go files
	@$(SHED) run golangci-lint run ./...
.PHONY: lint

go-uninstall: ## Remove version installed with go install
	@rm $(shell go env GOPATH)/bin/shed
.PHONY: go-uninstall

# Run tests and collect coverage data
test: ## Run all tests
	@mkdir -p coverage
	@go test -coverpkg=$(COVERPKGS) -coverprofile=coverage/coverage.txt ./...
.PHONY: test

cover: test ## Run all tests and generate coverage data
	@go tool cover -html=coverage/coverage.txt -o coverage/coverage.html
.PHONY: cover

scripts/install.sh: .goreleaser.yml ## Generate install script to download binaries
	@$(SHED) run godownloader .goreleaser.yml > $@
