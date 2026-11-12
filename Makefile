GOLANGCI_LINT_VERSION ?= v2.15.0

.PHONY: all build vet lint test tidy-check

all: tidy-check build vet lint test

build:
	go build ./...

vet:
	go vet ./...

lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run ./...

test:
	go test -race -timeout 30m ./...

tidy-check:
	go mod tidy -diff
