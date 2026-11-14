.PHONY: build test clean

build:
	go build -ldflags "-X main.version=$$(git describe --tags --always)" -o ./bin/forage-mcp ./cmd/forage-mcp

test:
	go test ./...

clean:
	rm -rf bin/
