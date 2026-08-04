# Import-Export Backend

A Go-based backend system for order management and warehouse operations with Excel integration.

## About this snapshot

This is a public, read-only snapshot of a private project, published as a code sample.
It is not maintained here and is not expected to run as-is.

Before publication the history was rewritten to remove operational data: a real
accounting spreadsheet used as a test fixture, committed build artifacts, and the
identifiers of live user accounts. Seed emails and UIDs throughout the repository are
synthetic placeholders. One test fixture is therefore missing, so the revenue/expense
tests will not run; see `test/data/excel/README.md`.

## Quick Start

### Docker Compose (Recommended)

```bash
git clone <repository-url>
cd cim-backend
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
