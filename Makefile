BINARY := ctx
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X main.Version=$(VERSION)"

.PHONY: build test lint clean install fmt-check vet check release-check

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/ctx

install: build
	mv $(BINARY) $(GOPATH)/bin/$(BINARY)

test:
	go test ./... -v -count=1

test-race:
	go test ./... -v -count=1 -race

test-coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out

fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "Files need gofmt:"; gofmt -l .; exit 1)

vet:
	go vet ./...

lint:
	golangci-lint run ./...

check: vet lint test

release-check: check
	@grep -q '^replace' go.mod 2>/dev/null && { echo "Error: go.mod contains replace directives"; exit 1; } || true
	@echo "All release checks passed"

clean:
	rm -f $(BINARY) coverage.out

# Cross-compilation
build-all:
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-arm64 ./cmd/ctx
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-amd64 ./cmd/ctx
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-amd64 ./cmd/ctx
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-arm64 ./cmd/ctx
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-windows-amd64.exe ./cmd/ctx
