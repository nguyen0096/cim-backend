# Database Schema & API Design
## Import-Export Backend System

## 1. Scope of Work

**Authentication & Authorization:**
- Email/Password authentication with Firebase Auth
- 2 roles: Admin and Staff
- Role-based access control

**Product & Inventory Management:**
- CRUD operations for products and inventory
- Import/export functionality
- Stock quantity updates
- Real-time inventory tracking

**Order Management:**
- Create orders with product selection
- Update order status (pending, processing, completed)
- Confirm completion → automatically deduct from inventory
- Order history and tracking

**Excel Integration:**
- Export data to single Excel sheet (~10 columns)
- Keep other data unchanged during updates
- Use Go Excel library for processing

**Tech stacks**
- Github action for unit-tests
- PostgreSQL/MySQL as database
- Backend with Go
- Echo library as API server framework
- GORM for interacting with Postgres

## 2. Database Schema Design

### Core Tables

#### Suppliers Table
```sql
CREATE TABLE suppliers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    contact_email VARCHAR(255),
    contact_phone VARCHAR(50),
    address TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

#### Products Table
```sql
CREATE TABLE products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    sku VARCHAR(100) UNIQUE,
    supplier_id UUID REFERENCES suppliers(id),
    unit_price DECIMAL(10,2),
    status VARCHAR(20) DEFAULT 'active' CHECK (status IN ('active', 'inactive', 'discontinued')),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

#### Inventory Table
```sql
CREATE TABLE inventory (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID REFERENCES products(id) ON DELETE CASCADE,
    quantity INTEGER DEFAULT 0,
    reorder_level INTEGER DEFAULT 0,
    location VARCHAR(100),
    last_updated TIMESTAMP DEFAULT NOW(),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(product_id)
);
```

#### Purchase Orders Table
```sql
CREATE TABLE purchase_orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_number VARCHAR(50) UNIQUE NOT NULL,
    supplier_id UUID REFERENCES suppliers(id),
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'received', 'cancelled')),
    total_amount DECIMAL(10,2),
    notes TEXT,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

#### Purchase Order Items Table
```sql
CREATE TABLE purchase_order_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    purchase_order_id UUID REFERENCES purchase_orders(id) ON DELETE CASCADE,
    product_id UUID REFERENCES products(id),
    quantity INTEGER NOT NULL,
    unit_price DECIMAL(10,2),
    total_price DECIMAL(10,2),
    received_quantity INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

#### Inventory Transactions Table
```sql
CREATE TABLE inventory_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID REFERENCES products(id),
    transaction_type VARCHAR(20) NOT NULL CHECK (transaction_type IN ('purchase', 'sale', 'adjustment', 'return')),
    quantity INTEGER NOT NULL,
    reference_id UUID, -- purchase_order_id or order_id
    reference_type VARCHAR(20), -- 'purchase_order', 'order', 'manual'
    notes TEXT,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP DEFAULT NOW()
);
```

#### Orders Table
```sql
CREATE TABLE orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_number VARCHAR(50) UNIQUE NOT NULL,
    customer_name VARCHAR(255),
    customer_email VARCHAR(255),
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'completed', 'cancelled')),
    total_amount DECIMAL(10,2),
    notes TEXT,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

#### Order Items Table
```sql
CREATE TABLE order_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID REFERENCES orders(id) ON DELETE CASCADE,
    product_id UUID REFERENCES products(id),
    quantity INTEGER NOT NULL,
    unit_price DECIMAL(10,2),
    total_price DECIMAL(10,2),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

### Indexes for Performance
```sql
-- Performance indexes
CREATE INDEX idx_products_supplier ON products(supplier_id);
CREATE INDEX idx_inventory_product ON inventory(product_id);
CREATE INDEX idx_purchase_orders_supplier ON purchase_orders(supplier_id);
CREATE INDEX idx_purchase_orders_status ON purchase_orders(status);
CREATE INDEX idx_purchase_order_items_order ON purchase_order_items(purchase_order_id);
CREATE INDEX idx_purchase_order_items_product ON purchase_order_items(product_id);
CREATE INDEX idx_inventory_transactions_product ON inventory_transactions(product_id);
CREATE INDEX idx_inventory_transactions_type ON inventory_transactions(transaction_type);
CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_order_items_order ON order_items(order_id);
CREATE INDEX idx_order_items_product ON order_items(product_id);
```

## 3. API Design

### Authentication Endpoints
```
POST /api/v1/auth/verify-token
GET  /api/v1/auth/profile
```

### Product Management
```
GET    /api/v1/products                    # List products with filtering
POST   /api/v1/products                    # Create new product
GET    /api/v1/products/{id}                # Get product details
PUT    /api/v1/products/{id}                # Update product
DELETE /api/v1/products/{id}               # Delete product
GET    /api/v1/products/{id}/inventory     # Get product inventory
```

### Inventory Management
```
GET    /api/v1/inventory                   # List inventory with stock levels
PUT    /api/v1/inventory/{id}              # Update inventory quantity
POST   /api/v1/inventory/adjust            # Manual inventory adjustment
GET    /api/v1/inventory/transactions      # List inventory transactions
GET    /api/v1/inventory/low-stock         # Get low stock items
```

### Supplier Management
```
GET    /api/v1/suppliers                   # List suppliers
POST   /api/v1/suppliers                   # Create supplier
GET    /api/v1/suppliers/{id}              # Get supplier details
PUT    /api/v1/suppliers/{id}              # Update supplier
DELETE /api/v1/suppliers/{id}              # Delete supplier
```

### Purchase Order Management
```
GET    /api/v1/purchase-orders             # List purchase orders
POST   /api/v1/purchase-orders             # Create purchase order
GET    /api/v1/purchase-orders/{id}        # Get purchase order details
PUT    /api/v1/purchase-orders/{id}        # Update purchase order
PUT    /api/v1/purchase-orders/{id}/status # Update purchase order status
DELETE /api/v1/purchase-orders/{id}       # Cancel purchase order
POST   /api/v1/purchase-orders/{id}/receive # Receive purchase order items
```

### Order Management
```
GET    /api/v1/orders                      # List orders
POST   /api/v1/orders                      # Create order
GET    /api/v1/orders/{id}                 # Get order details
PUT    /api/v1/orders/{id}                 # Update order
PUT    /api/v1/orders/{id}/status          # Update order status
DELETE /api/v1/orders/{id}                 # Cancel order
POST   /api/v1/orders/{id}/complete        # Complete order (deduct inventory)
```

### Excel Integration
```
POST   /api/v1/excel/import-products       # Import products from Excel
GET    /api/v1/excel/export-products        # Export products to Excel
POST   /api/v1/excel/import-inventory      # Import inventory from Excel
GET    /api/v1/excel/export-inventory      # Export inventory to Excel
GET    /api/v1/excel/template-products     # Download product template
GET    /api/v1/excel/template-inventory    # Download inventory template
```

### Reports & Analytics
```
GET    /api/v1/reports/inventory-summary   # Inventory summary report
GET    /api/v1/reports/low-stock           # Low stock report
GET    /api/v1/reports/purchase-summary    # Purchase order summary
GET    /api/v1/reports/order-summary       # Order summary
```

## 4. API Request/Response Examples

### Create Product
```json
POST /api/v1/products
{
  "name": "Laptop Computer",
  "description": "High-performance laptop",
  "sku": "LAPTOP-001",
  "supplier_id": "uuid-here",
  "unit_price": 999.99,
  "status": "active"
}

Response:
{
  "id": "uuid-here",
  "name": "Laptop Computer",
  "description": "High-performance laptop",
  "sku": "LAPTOP-001",
  "supplier_id": "uuid-here",
  "unit_price": 999.99,
  "status": "active",
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z"
}
```

### Create Order
```json
POST /api/v1/orders
{
  "customer_name": "John Doe",
  "customer_email": "john@example.com",
  "items": [
    {
      "product_id": "uuid-here",
      "quantity": 2,
      "unit_price": 999.99
    }
  ],
  "notes": "Rush order"
}

Response:
{
  "id": "uuid-here",
  "order_number": "ORD-2024-001",
  "customer_name": "John Doe",
  "customer_email": "john@example.com",
  "status": "pending",
  "total_amount": 1999.98,
  "items": [...],
  "created_at": "2024-01-01T00:00:00Z"
}
```

### Complete Order (Auto-deduct inventory)
```json
POST /api/v1/orders/{id}/complete
{
  "notes": "Order completed successfully"
}

Response:
{
  "id": "uuid-here",
  "status": "completed",
  "completed_at": "2024-01-01T00:00:00Z",
  "inventory_updated": true
}
```

## 5. Excel Export Format

### Products Export (10 columns)
| ID | Name | SKU | Supplier | Unit Price | Status | Stock Quantity | Reorder Level | Location | Last Updated |
|----|------|-----|----------|------------|--------|----------------|---------------|----------|--------------|
| uuid | Laptop Computer | LAPTOP-001 | Tech Supplier | 999.99 | active | 50 | 10 | A1-B2 | 2024-01-01 |

### Inventory Export (10 columns)
| Product ID | Product Name | SKU | Current Stock | Reorder Level | Location | Last Transaction | Transaction Type | Quantity Changed | Date |
|------------|--------------|-----|---------------|---------------|----------|------------------|------------------|------------------|------|
| uuid | Laptop Computer | LAPTOP-001 | 50 | 10 | A1-B2 | Purchase | +20 | 2024-01-01 |

## 6. Implementation Plan

### Phase 1: Foundation
- [ ] Set up Go project with Echo framework
- [ ] Configure PostgreSQL database
- [ ] Implement Firebase Auth integration
- [ ] Create basic project structure
- [ ] Set up database migrations

### Phase 2: Core Models
- [ ] Implement User, Supplier, Product models
- [ ] Create CRUD operations for all models
- [ ] Implement inventory management
- [ ] Add validation and error handling

### Phase 3: Order Management
- [ ] Implement Purchase Order functionality
- [ ] Create Order management system
- [ ] Add inventory auto-deduction on order completion
- [ ] Implement status workflows

### Phase 4: Excel Integration
- [ ] Implement Excel import/export using Go Excel library
- [ ] Create Excel templates
- [ ] Add data validation for imports
- [ ] Implement batch processing

### Phase 5: Testing & Deployment
- [ ] Write comprehensive tests
- [ ] Performance optimization
- [ ] Security hardening
- [ ] Production deployment setup
- [ ] Documentation creation

## 7. Technology Stack

- **Backend:** Go 1.25.1 with Echo framework
- **Database:** PostgreSQL
- **Authentication:** Firebase Auth
- **Excel Processing:** go-excelize library
- **ORM:** GORM
- **Validation:** go-playground/validator
- **Testing:** testify
- **Documentation:** Swagger/OpenAPI

## 8. Security Considerations

- Firebase Auth for secure authentication
- JWT token validation
- Role-based access control (Admin/Staff)
- Input validation and sanitization
- SQL injection prevention
- Rate limiting on API endpoints
- Secure file upload handling for Excel files

## 9. Database Relationships

```
Users (1) ──→ (N) Purchase Orders
Users (1) ──→ (N) Orders
Users (1) ──→ (N) Inventory Transactions

Suppliers (1) ──→ (N) Products
Suppliers (1) ──→ (N) Purchase Orders

Products (1) ──→ (1) Inventory
Products (1) ──→ (N) Purchase Order Items
Products (1) ──→ (N) Order Items
Products (1) ──→ (N) Inventory Transactions

Purchase Orders (1) ──→ (N) Purchase Order Items
Orders (1) ──→ (N) Order Items
```

## 10. Key Business Logic

### Inventory Auto-Deduction on Order Completion
1. When order status changes to "completed"
2. For each order item, reduce inventory quantity
3. Create inventory transaction record
4. Update inventory last_updated timestamp
5. Check for low stock alerts

### Purchase Order Receiving
1. When purchase order status changes to "received"
2. For each purchase order item, increase inventory quantity
3. Create inventory transaction record
4. Update received_quantity in purchase_order_items

### Excel Import/Export Logic
1. **Import:** Validate data, create/update records, maintain referential integrity
2. **Export:** Generate Excel file with current data, preserve formatting
3. **Template:** Provide downloadable templates with proper column headers
