.PHONY: build run test clean docker-build docker-run docker-stop migrate-up migrate-down

# Build the application
build:
	go build -o bin/main .

# Run the application
run:
	go run main.go

# Run tests
test:
	go test -v ./...

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

# Docker commands
docker-build:
	docker-compose build

docker-run:
	docker-compose up -d

docker-stop:
	docker-compose down

docker-logs:
	docker-compose logs -f

# Database migrations
migrate-up:
	docker-compose exec app go run main.go migrate up

migrate-down:
	docker-compose exec app go run main.go migrate down

# Development setup
dev-setup:
	docker-compose up -d postgres
	sleep 10
	go run main.go

# Production setup
prod-setup:
	docker-compose -f docker-compose.yml -f docker-compose.prod.yml up -d

# Generate API documentation
docs:
	swag init -g main.go

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
