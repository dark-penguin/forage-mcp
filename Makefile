BINARY := bin/forage-mcp

.PHONY: build test clean

build:
	go build -o $(BINARY) ./cmd/forage-mcp

test:
	go test ./...

clean:
	rm -rf bin/
