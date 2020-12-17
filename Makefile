.DEFAULT_GOAL = build
SHED = go run main.go
COVERPKGS = ./cache,./client,./internal/spinner,./internal/util,./lockfile,./tool

# Get all dependencies
setup:
	@echo Installing dependencies
	@go mod download
# Self-hoisted!
	@echo Installing tool dependencies
	@$(SHED) install
	@$(SHED) run go-fish install
.PHONY: setup

build:
	@go build
.PHONY: build

build-snapshot:
	@$(SHED) run goreleaser build -- --snapshot --rm-dist
.PHONY: build-snapshot

# Generate shell completions for distribution
completions:
	@mkdir -p completions
	@$(SHED) completions bash > completions/shed.bash
	@$(SHED) completions zsh > completions/_shed
.PHONY: completions

# Clean all build artifacts
clean:
	@rm -rf completions
	@rm -rf coverage
	@rm -rf dist
	@rm -f shed
.PHONY: clean

fmt:
	@gofmt -w .
.PHONY: fmt

check-fmt:
	@./scripts/check_fmt.sh
.PHONY: check-fmt

lint:
	@$(SHED) run golangci-lint run ./...
.PHONY: lint

# Remove version installed with go install
go-uninstall:
	@rm $(shell go env GOPATH)/bin/shed
.PHONY: go-uninstall

# Run tests and collect coverage data
test:
	@mkdir -p coverage
	@go test -coverpkg=$(COVERPKGS) -coverprofile=coverage/coverage.txt ./...
	@go tool cover -html=coverage/coverage.txt -o coverage/coverage.html
.PHONY: test

# Run tests and print coverage data to stdout
test-ci:
	@mkdir -p coverage
	@go test -coverpkg=$(COVERPKGS) -coverprofile=coverage/coverage.txt ./...
	@go tool cover -func=coverage/coverage.txt
.PHONY: test-ci
