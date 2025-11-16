.PHONY: build run test clean docker-build docker-run docker-stop migrate-up migrate-down docs mock proto proto-install proto-clean buf-install buf-lint buf-format

# Build the application
build:
	go build -o bin/main .

# Run the application
run:
	go run main.go

# Run tests
test:
	go test -v ./internal/...

# Run tests with coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

# Install dependencies
deps:
	go mod download
	go mod tidy

# Run linter
lint:
	golangci-lint run

# Format code
fmt:
	go fmt ./...

# Database migrations
migrate-up:
	go run ./cmd/util migrate up

migrate-down:
	go run ./cmd/util migrate down

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

# Seed database with mock data
seed-db:
	go run cmd/util seed

# Help
help:
	@echo "Available commands:"
	@echo "  build          - Build the application"
	@echo "  run            - Run the application"
	@echo "  test           - Run tests"
	@echo "  test-coverage  - Run tests with coverage"
	@echo "  clean          - Clean build artifacts"
	@echo "  deps           - Install dependencies"
	@echo "  lint           - Run linter"
	@echo "  fmt            - Format code"
	@echo "  docker-build   - Build Docker images"
	@echo "  docker-run     - Run with Docker Compose"
	@echo "  docker-stop    - Stop Docker containers"
	@echo "  docker-logs    - Show Docker logs"
	@echo "  dev-setup      - Setup development environment"
	@echo "  prod-setup     - Setup production environment"
	@echo "  docs           - Generate API documentation"
	@echo "  test-api       - Test API with sample data"
	@echo "  seed-db        - Seed database with mock data"

generate:
	go generate ./...

migrate-file:
	migrate create -ext sql -dir database/migrations ${NAME}

test-components:
	go test -v ./test/components/scenarios -run TestComponentTestSuite