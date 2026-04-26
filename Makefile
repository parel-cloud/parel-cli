.PHONY: build test vet lint tidy clean install-tools snapshot

BIN     := parel
PKG     := ./cmd/parel
VERSION ?= dev
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
  -X github.com/parel-cloud/parel-cli/internal/cmd.version=$(VERSION) \
  -X github.com/parel-cloud/parel-cli/internal/cmd.commit=$(COMMIT) \
  -X github.com/parel-cloud/parel-cli/internal/cmd.date=$(DATE)

build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN) $(PKG)

test:
	go test -race -cover ./...

vet:
	go vet ./...

lint:
	golangci-lint run

tidy:
	go mod tidy

clean:
	rm -f $(BIN) $(BIN).exe coverage.txt
	rm -rf dist/ .goreleaser-snapshot/

install-tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/goreleaser/goreleaser/v2@latest

snapshot:
	goreleaser build --snapshot --clean
