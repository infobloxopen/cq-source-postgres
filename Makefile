.PHONY: test lint build gen-docs docker-build clean

BINARY_NAME=cq-source-postgres

## test: Run all tests
test:
	go test -race -count=1 ./...

## lint: Run golangci-lint
lint:
	golangci-lint run --timeout 10m

## build: Build the binary
build:
	CGO_ENABLED=0 go build -o $(BINARY_NAME) .

## gen-docs: Generate table documentation
gen-docs:
	go run main.go doc docs/tables

## docker-build: Build Docker image
docker-build:
	docker build -t $(BINARY_NAME):dev .

## clean: Remove build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -rf dist/

## help: Show this help
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //' | column -t -s ':'
