BINARY_NAME=ssh2rclone
BIN_DIR=bin
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE?=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
LDFLAGS=-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

.PHONY: all build test lint snapshot clean

all: test build

build:
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY_NAME) main.go

test:
	go test -v -race ./...

lint:
	@which golangci-lint > /dev/null || (echo "golangci-lint is not installed" && exit 1)
	golangci-lint run

snapshot:
	@which goreleaser > /dev/null || (echo "goreleaser is not installed" && exit 1)
	goreleaser release --snapshot --clean

clean:
	rm -rf $(BIN_DIR) dist
