# Import-Export Backend

A Go-based backend system for order management and warehouse operations with Excel integration.

## Quick Start

### Docker Compose (Recommended)

```bash
git clone <repository-url>
cd import-export-backend
docker-compose up -d
```

**Access:**

- API: `http://localhost:8080`
- Health: `http://localhost:8080/health`
- DB UI: `http://localhost:8081`

### Local Development

```bash
go mod download
cp env.example .env
docker-compose up -d postgres
go run main.go
```

## Development Commands

```bash
# Testing
make test
make test-api

# Code quality
make fmt
make lint

# Docker
make docker-build
make docker-run
make docker-logs
```
