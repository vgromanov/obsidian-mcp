.PHONY: all build test test-integration cover fmt vet tidy clean install

BINARY_NAME := obsidian-mcp
CMD := ./cmd/obsidian-mcp

all: build test

build:
	CGO_ENABLED=0 go build -o $(BINARY_NAME) $(CMD)

install:
	CGO_ENABLED=0 go install $(CMD)

test:
	go test ./...

test-integration:
	go test -tags=integration ./internal/integration/...

cover:
	go test ./internal/fetch ./internal/mcpapp ./internal/templater ./internal/obsidian -coverprofile=coverage.out -covermode=atomic
	go tool cover -func=coverage.out | tail -8

fmt:
	go fmt ./...

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -f $(BINARY_NAME) coverage.out

dist:
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -o dist/$(BINARY_NAME)-darwin-amd64 $(CMD)
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o dist/$(BINARY_NAME)-darwin-arm64 $(CMD)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o dist/$(BINARY_NAME)-linux-amd64 $(CMD)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o dist/$(BINARY_NAME)-linux-arm64 $(CMD)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o dist/$(BINARY_NAME)-windows-amd64.exe $(CMD)
