# Import-Export Backend

A Go-based backend system for order management and warehouse operations with Excel integration.

## Features

- **Authentication & Authorization**: Firebase Auth integration with Admin/Staff roles
- **Product Management**: CRUD operations for products and suppliers
- **Inventory Management**: Real-time stock tracking and low-stock alerts
- **Order Management**: Order creation, status updates, and automatic inventory deduction
- **Excel Integration**: Import/export functionality with 10-column format
- **Purchase Orders**: Supplier management and purchase order processing
- **Reports**: Inventory summaries, low-stock reports, and order analytics

## Tech Stack

- **Backend**: Go 1.25 with Echo framework
- **Database**: PostgreSQL with GORM ORM
- **Authentication**: Firebase Auth
- **Excel Processing**: excelize library
- **Testing**: GitHub Actions CI/CD
- **Containerization**: Docker & Docker Compose

## Quick Start

### Prerequisites

- Go 1.25+
- Docker & Docker Compose
- PostgreSQL (if running locally)

### Using Docker Compose (Recommended)

1. **Clone the repository**
   ```bash
   git clone <repository-url>
   cd import-export-backend
   ```

2. **Start the services**
   ```bash
   docker-compose up -d
   ```

3. **Access the application**
   - API: http://localhost:8080
   - Health check: http://localhost:8080/health
   - Database Web UI: http://localhost:8081

### Local Development

1. **Install dependencies**
   ```bash
   go mod download
   ```

2. **Set up environment variables**
   ```bash
   cp env.example .env
   # Edit .env with your configuration
   ```

3. **Start PostgreSQL**
   ```bash
   docker-compose up -d postgres
   ```

4. **Run the application**
   ```bash
   go run main.go
   ```

## API Endpoints

### Authentication
- `POST /api/v1/auth/verify-token` - Verify Firebase token
- `GET /api/v1/auth/profile` - Get user profile

### Products
- `GET /api/v1/products` - List products
- `POST /api/v1/products` - Create product
- `GET /api/v1/products/{id}` - Get product details
- `PUT /api/v1/products/{id}` - Update product
- `DELETE /api/v1/products/{id}` - Delete product

### Inventory
- `GET /api/v1/inventory` - List inventory
- `PUT /api/v1/inventory/{id}` - Update inventory
- `POST /api/v1/inventory/adjust` - Manual inventory adjustment
- `GET /api/v1/inventory/transactions` - List transactions
- `GET /api/v1/inventory/low-stock` - Get low stock items

### Orders
- `GET /api/v1/orders` - List orders
- `POST /api/v1/orders` - Create order
- `GET /api/v1/orders/{id}` - Get order details
- `PUT /api/v1/orders/{id}/status` - Update order status
- `POST /api/v1/orders/{id}/complete` - Complete order (auto-deduct inventory)

### Excel Integration
- `POST /api/v1/excel/import-products` - Import products from Excel
- `GET /api/v1/excel/export-products` - Export products to Excel
- `POST /api/v1/excel/import-inventory` - Import inventory from Excel
- `GET /api/v1/excel/export-inventory` - Export inventory to Excel
- `GET /api/v1/excel/template-products` - Download product template
- `GET /api/v1/excel/template-inventory` - Download inventory template

## Database Schema

The system uses the following core entities:
- **Users**: Firebase Auth integration
- **Suppliers**: Vendor management
- **Products**: Product catalog with SKU tracking
- **Inventory**: Stock levels and locations
- **Purchase Orders**: Supplier orders
- **Orders**: Customer orders
- **Inventory Transactions**: Audit trail for all stock movements

## Development

### Running Tests
```bash
make test
make test-api
```

### Code Formatting
```bash
make fmt
```

### Linting
```bash
make lint
```

### Building
```bash
make build
```

## Docker Commands

```bash
# Build and run
make docker-build
make docker-run

# View logs
make docker-logs

# Stop services
make docker-stop
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DB_HOST` | Database host | localhost |
| `DB_PORT` | Database port | 5432 |
| `DB_USER` | Database user | postgres |
| `DB_PASSWORD` | Database password | password |
| `DB_NAME` | Database name | import_export_db |
| `SERVER_HOST` | Server host | 0.0.0.0 |
| `SERVER_PORT` | Server port | 8080 |
| `FIREBASE_SERVICE_ACCOUNT_PATH` | Firebase service account json file path | ./firebase-service-account.json
| `FIREBASE_PROJECT_ID` | Firebase Project ID | your-firebase-project-id

## Excel Integration

### Export Format
Products and inventory can be exported to Excel with the following columns:

**Products Export:**
- ID, Name, SKU, Supplier, Unit Price, Status, Stock Quantity, Reorder Level, Location, Last Updated

**Inventory Export:**
- Product ID, Product Name, SKU, Current Stock, Reorder Level, Location, Last Transaction, Transaction Type, Quantity Changed, Date

### Import Process
1. Download the appropriate template
2. Fill in your data
3. Upload the file via the import endpoint
4. System validates and processes the data

## API Authentication

All API endpoints (except health check) require authentication:
1. Include `Authorization: Bearer <token>` header
2. Token should be a valid Firebase JWT token
3. User role determines access level (Admin/Staff)

## Monitoring

- Health check: `GET /health`
- Application logs via Docker Compose
- Database performance monitoring via PostgreSQL

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests: `make test`
5. Submit a pull request
