.PHONY: build test test-unit test-integration lint clean

NAME=openbao-teradata-secret-plugin
VERSION=0.1.0

build:
	go build -o bin/$(NAME) ./cmd/teradata/

test:
	go test -v -count=1 ./...

test-unit:
	go test -v -count=1 ./...

test-integration:
	@echo "Requires Teradata database running"
	export RUN_INTEGRATION_TESTS=true
	go test -tags=integration -v -count=1 ./...

lint:
	golangci-lint run

clean:
	rm -rf bin/

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
