# Binaries will be built into the ./bin directory
all:: bin/openapi-mcp

bin/openapi-mcp:: $(shell find pkg -type f -name '*.go') $(shell find cmd/openapi-mcp -type f -name '*.go')
	@mkdir -p bin
	go build -o bin/openapi-mcp ./cmd/openapi-mcp

test::
	go test ./...

clean::
	rm -f bin/openapi-mcp
