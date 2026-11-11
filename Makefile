.PHONY: build test clean

build:
	go build -o ./bin/forage-mcp ./cmd/forage-mcp

test:
	go test ./...

clean:
	rm -rf bin/
