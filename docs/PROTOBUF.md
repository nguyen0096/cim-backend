# Protocol Buffers (Protobuf) API Contract

This document describes the Protocol Buffers definitions for the Import-Export Backend API.

## Table of Contents

- [Overview](#overview)
- [Directory Structure](#directory-structure)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Compilation](#compilation)
- [Usage](#usage)
- [API Services](#api-services)
- [Message Definitions](#message-definitions)

## Overview

Protocol Buffers (protobuf) is a language-neutral, platform-neutral extensible mechanism for serializing structured data. This project uses protobuf to define the API contract for all services.

## Directory Structure

```
proto/
├── common/              # Common message definitions
│   └── common.proto     # Base, pagination, and error messages
├── models/              # Data model definitions
│   ├── product.proto    # Product, Supplier, Inventory models
│   ├── purchase_order.proto
│   ├── user.proto
│   ├── settings.proto
│   └── dto.proto        # Data Transfer Objects
└── services/            # Service definitions
    ├── product_service.proto
    ├── purchase_order_service.proto
    ├── inventory_service.proto
    ├── inventory_item_service.proto
    ├── supplier_service.proto
    ├── settings_service.proto
    ├── user_service.proto
    └── excel_service.proto
```

## Prerequisites

Before working with protobuf files, ensure you have the following installed:

### Option 1: Using Buf (Recommended)

**Buf CLI** - Modern protobuf tooling with built-in linting and breaking change detection

```bash
make buf-install
```

Or manually:

```bash
go install github.com/bufbuild/buf/cmd/buf@latest
```

### Option 2: Using protoc (Legacy)

1. **Protocol Buffers Compiler (protoc)**

   - macOS: `brew install protobuf`
   - Linux: `apt-get install protobuf-compiler`
   - Or download from: https://github.com/protocolbuffers/protobuf/releases

2. **Go Protobuf Plugins**

   ```bash
   make proto-install
   ```

   Or manually:

   ```bash
   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
   ```

## Installation

### Using Buf (Recommended)

1. Install Buf CLI:

   ```bash
   make buf-install
   ```

2. Verify installation:
   ```bash
   buf --version
   ```

### Using protoc

1. Install protobuf tools:

   ```bash
   make proto-install
   ```

2. Verify installation:
   ```bash
   protoc --version
   protoc-gen-go --version
   protoc-gen-go-grpc --version
   ```

## Compilation

### Using Make (Recommended)

The `make proto` command uses Buf for code generation:

```bash
# Compile all protobuf files
make proto

# Lint protobuf files
make buf-lint

# Format protobuf files
make buf-format

# Clean generated files
make proto-clean
```

### Using Buf Directly

```bash
# Generate code from buf.gen.yaml
buf generate

# Lint proto files
buf lint

# Format proto files
buf format -w

# Check for breaking changes
buf breaking --against '.git#branch=main'
```

### Manual Compilation with protoc

For individual files:

```bash
# Compile common messages
protoc --proto_path=. \
  --go_out=. \
  --go_opt=paths=source_relative \
  proto/common/common.proto

# Compile service with gRPC
protoc --proto_path=. \
  --go_out=. \
  --go_opt=paths=source_relative \
  --go-grpc_out=. \
  --go-grpc_opt=paths=source_relative \
  proto/services/product_service.proto
```

## Usage

### In Go Code

Import the generated protobuf packages:

```go
import (
    "cim-backend/proto/common"
    "cim-backend/proto/models"
    "cim-backend/proto/services"
)
```

### Example: Creating a Product

```go
product := &models.Product{
    Base: &common.Base{
        Id: 1,
        CreatedBy: "user@example.com",
    },
    Name: "Sample Product",
    Description: "Product description",
    ProductType: "electronics",
    Unit: "piece",
    Status: "active",
}
```

### Example: Using Pagination

```go
params := &common.ListParams{
    Page: 1,
    Limit: 20,
    Search: "product",
    Sort: "created_at",
    Order: "desc",
    Status: "active",
}

request := &services.ListProductsRequest{
    Params: params,
}
```

## API Services

### Product Service

Operations:

- `ListProducts` - List all products with pagination
- `CreateProduct` - Create a new product
- `SearchProducts` - Search products
- `UpdateProductStatus` - Update product status
- `ImportProductsFromCSV` - Import products from CSV

### Purchase Order Service

Operations:

- `ListPurchaseOrders` - List purchase orders
- `CreatePurchaseOrder` - Create purchase order
- `UpdatePurchaseOrderItemStatus` - Update item status
- `UpdatePurchaseOrderDeliveryStatus` - Update delivery status

### Inventory Service

Operations:

- `ListInventories` - List inventories
- `GetInventory` - Get inventory by ID
- `CreateInventory` - Create inventory
- `UpdateInventory` - Update inventory
- `DeleteInventory` - Delete inventory
- `GetLastPurchasePrices` - Get last purchase prices
- `ConfirmInventory` - Confirm inventory items
- `DisposeInventoryItems` - Dispose items

### Inventory Item Service

Operations:

- `ListInventoryItems` - List all items
- `GetInventoryItemsByInventoryID` - Get items by inventory
- `GetInventoryItem` - Get item by ID
- `GetInventoryItemByProductID` - Get item by product
- `CreateInventoryItem` - Create item
- `UpdateInventoryItem` - Update item
- `DeleteInventoryItem` - Delete item
- `AdjustInventoryItemQuantity` - Adjust quantity
- `GetLowStockItems` - Get low stock items

### Settings Service

Operations:

- `GetSettings` - Get all settings
- `GetSetting` - Get setting by key
- `SetSetting` - Set setting value
- `DeleteSetting` - Delete setting

### User Service

Operations:

- `GetUserPermissions` - Get user permissions

### Supplier Service

Operations:

- `UpdateSupplierStatus` - Update supplier status

### Excel Service

Operations:

- `VerifyExcelFile` - Verify Excel file and sheet

## Message Definitions

### Common Messages

#### Base

```protobuf
message Base {
  uint32 id = 1;
  string created_by = 2;
  google.protobuf.Timestamp created_at = 3;
  google.protobuf.Timestamp updated_at = 4;
  google.protobuf.Timestamp deleted_at = 5;
}
```

#### ListParams

```protobuf
message ListParams {
  int32 page = 1;
  int32 limit = 2;
  string search = 3;
  string sort = 4;
  string order = 5;
  string status = 6;
  string start_date = 7;
  string end_date = 8;
}
```

#### PaginationResult

```protobuf
message PaginationResult {
  int64 total = 1;
  int32 page = 2;
  int32 limit = 3;
  int32 total_pages = 4;
}
```

### Model Messages

Refer to individual proto files in `proto/models/` for detailed message definitions:

- **Product** - Product information
- **Supplier** - Supplier information
- **Inventory** - Inventory/warehouse information
- **InventoryItem** - Inventory item details
- **InventoryTransaction** - Transaction records
- **PurchaseOrder** - Purchase order
- **PurchaseOrderItem** - Purchase order items
- **User** - User information
- **Settings** - Application settings

### Request/Response Messages

DTOs (Data Transfer Objects) are defined in `proto/models/dto.proto`:

- `UpdatePurchaseOrderItemStatusResponse`
- `UpdatePurchaseOrderDeliveryStatusRequest`
- `ConfirmInventoryRequest`
- `DisposeItemsRequest`
- `LastPurchasePriceMap`
- And more...

## Configuration Files

### buf.yaml

Defines lint and breaking change detection rules:

```yaml
version: v1
name: buf.build/cim-backend
breaking:
  use:
    - FILE
lint:
  use:
    - DEFAULT
```

### buf.gen.yaml

Configures code generation with managed mode for consistent package naming:

```yaml
version: v1
managed:
  enabled: true
  go_package_prefix:
    default: cim-backend
    except:
      - buf.build/googleapis/googleapis
plugins:
  - plugin: buf.build/protocolbuffers/go
    out: .
    opt:
      - paths=source_relative
  - plugin: buf.build/grpc/go
    out: .
    opt:
      - paths=source_relative
```

The `managed` mode automatically handles Go package naming, and `paths=source_relative` ensures generated files are placed alongside proto files.

## Best Practices

1. **Versioning**: When making breaking changes, create a new version of the proto file
2. **Field Numbers**: Never reuse field numbers when removing fields
3. **Required Fields**: Avoid using required fields; use validation in application code
4. **Enums**: Always include a default/unknown value (0) for enums
5. **Documentation**: Add comments to proto files for better code generation
6. **Backward Compatibility**: Add new fields instead of modifying existing ones

## Troubleshooting

### Compilation Errors

1. **Import not found**: Ensure you're running protoc from project root

   ```bash
   cd /path/to/cim-backend
   make proto
   ```

2. **Plugin not found**: Reinstall protoc plugins

   ```bash
   make proto-install
   ```

3. **Permission denied**: Make script executable
   ```bash
   chmod +x scripts/compile-proto.sh
   ```

### Generated Code Issues

1. **Package conflicts**: Ensure `go_package` option is correctly set
2. **Import errors**: Run `go mod tidy` after compilation
3. **Type conflicts**: Check for circular dependencies in proto files

## References

- [Protocol Buffers Documentation](https://protobuf.dev/)
- [Go Protobuf Tutorial](https://protobuf.dev/getting-started/gotutorial/)
- [gRPC Go Quick Start](https://grpc.io/docs/languages/go/quickstart/)
- [Style Guide](https://protobuf.dev/programming-guides/style/)

## Contributing

When adding new API endpoints or models:

1. Define the proto messages in appropriate files
2. Run `make proto` to compile
3. Update this documentation
4. Ensure backward compatibility
5. Add tests for new functionality

## Support

For issues or questions about protobuf definitions, please contact the development team or create an issue in the project repository.
