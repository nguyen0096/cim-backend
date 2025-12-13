.PHONY: build build-util run test clean docker-build docker-run docker-stop migrate-up migrate-down docs mock proto proto-install proto-clean buf-install buf-lint buf-format

# BUILD COMMANDS

build:
	go build -o bin/main .

# Build the utility binary
build-util:
	go build -o bin/util ./cmd/util

# DEVELOPMENT COMMANDS
generate:
	go generate ./...

run:
	go run main.go

test: # AGENTS MUST NOT CHANGE THIS COMMAND.
	go test ./...

test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

deps:
	go mod download
	go mod tidy

lint:
	golangci-lint run

fmt:
	go fmt ./...

migrate:
	go run ./cmd/util migrate

seed:
	go run ./cmd/util seed

token:
	go run ./cmd/auth

# Development setup
dev-setup:
	docker-compose up -d postgres pgweb
	sleep 10
	go run main.go

# Generate API documentation
docs:
	swag init -g main.go

# Test API with sample data
test-api:
	./test-api.sh

migrate-file:
	migrate create -ext sql -dir database/migrations ${NAME}
