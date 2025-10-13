# Protocol Buffers API Contract

This directory contains all Protocol Buffer definitions for the Import-Export Backend API.

## Directory Structure

```
proto/
├── common/              # Shared message definitions
│   └── common.proto     # Base, pagination, error messages
│
├── models/              # Data model definitions
│   ├── product.proto    # Product, Supplier, Inventory, InventoryItem, InventoryTransaction
│   ├── purchase_order.proto  # PurchaseOrder, PurchaseOrderItem
│   ├── user.proto       # User
│   ├── settings.proto   # Settings
│   └── dto.proto        # Data Transfer Objects
│
└── services/            # gRPC service definitions
    ├── product_service.proto
    ├── purchase_order_service.proto
    ├── inventory_service.proto
    ├── inventory_item_service.proto
    ├── supplier_service.proto
    ├── settings_service.proto
    ├── user_service.proto
    └── excel_service.proto
```

## Quick Start

1. **Install Buf CLI (Recommended):**

   ```bash
   make buf-install
   ```

2. **Compile proto files:**

   ```bash
   make proto
   ```

3. **Lint and format:**

   ```bash
   make buf-lint
   make buf-format
   ```

4. **Clean generated files:**
   ```bash
   make proto-clean
   ```

## Configuration

- **buf.yaml** - Defines linting rules and breaking change detection
- **buf.gen.yaml** - Configures code generation with managed mode for Go packages

## Available Services

### ProductService

- List, create, search products
- Update product status
- Import from CSV

### PurchaseOrderService

- List, create purchase orders
- Update item and delivery status

### InventoryService

- CRUD operations for inventories
- Get last purchase prices
- Confirm and dispose items

### InventoryItemService

- CRUD operations for inventory items
- Adjust quantities
- Get low stock items

### SettingsService

- Manage application settings

### UserService

- Get user permissions

### SupplierService

- Update supplier status

### ExcelService

- Verify Excel files

## Usage Example

```go
import (
    "cim-backend/proto/common"
    "cim-backend/proto/models"
    "cim-backend/proto/services"
)

// Create a product
product := &models.Product{
    Base: &common.Base{Id: 1},
    Name: "Sample Product",
    Status: "active",
}

// Use pagination
params := &common.ListParams{
    Page: 1,
    Limit: 20,
    Sort: "created_at",
    Order: "desc",
}
```

## Documentation

For detailed documentation, see [/docs/PROTOBUF.md](../docs/PROTOBUF.md)

## Notes

- Generated `.pb.go` and `*_grpc.pb.go` files are gitignored
- All models include common `Base` message with id, timestamps, and created_by
- Services support pagination, filtering, and sorting where applicable
- Use `google.protobuf.Timestamp` for dates
- Use `google.protobuf.Value` for dynamic JSON values
