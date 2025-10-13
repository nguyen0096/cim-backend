# Import-Export Backend

A Go-based backend system for order management and warehouse operations with Excel integration.

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

# Protobuf
make proto-install  # Install protobuf tools
buf lint # Lint protobuf
buf generate # Generate protobuf
```

## API Contract (Protobuf)

This project uses Protocol Buffers with **Buf** to define the API contract. All service definitions and data models are available in the `proto/` directory.

**Quick Start:**

1. Install Buf CLI:

   ```bash
   make buf-install
   ```

2. Compile protobuf files:

   ```bash
   make proto
   ```

3. Lint and format:
   ```bash
   make buf-lint
   make buf-format
   ```

**Configuration Files:**

- `buf.yaml` - Lint and breaking change detection
- `buf.gen.yaml` - Code generation settings

For detailed documentation, see [docs/PROTOBUF.md](docs/PROTOBUF.md)
