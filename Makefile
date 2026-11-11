.PHONY: build test clean

VERSION ?= v1.0.0
LDFLAGS := -X main.version=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o ./bin/forage-mcp ./cmd/forage-mcp

test:
	go test ./...

clean:
	rm -rf bin/
