BINARY := ctx
VERSION ?= 0.1.0
LDFLAGS := -ldflags "-s -w -X github.com/getctx/ctx/cmd/ctx.Version=$(VERSION)"

.PHONY: build test lint clean install

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/ctx

install: build
	mv $(BINARY) $(GOPATH)/bin/$(BINARY)

test:
	go test ./... -v -count=1

test-coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out

lint:
	golangci-lint run ./...

clean:
	rm -f $(BINARY) coverage.out

# Cross-compilation
build-all:
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-arm64 ./cmd/ctx
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-amd64 ./cmd/ctx
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-amd64 ./cmd/ctx
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-arm64 ./cmd/ctx
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-windows-amd64.exe ./cmd/ctx
