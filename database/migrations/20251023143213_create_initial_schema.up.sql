-- Create initial database schema
-- This migration creates all tables that were previously created by GORM AutoMigrate
-- Uses IF NOT EXISTS to be safe for existing databases

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    uid VARCHAR(255),
    email VARCHAR(255) NOT NULL,
    name VARCHAR(255),
    role VARCHAR(50) DEFAULT 'staff',
    type VARCHAR(50) DEFAULT 'user',
    status VARCHAR(50) DEFAULT 'active',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    CONSTRAINT chk_users_status CHECK (status IN ('active', 'pending', 'inactive'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_uid ON users(uid);
CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users(deleted_at);

-- Suppliers table
CREATE TABLE IF NOT EXISTS suppliers (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    contact_email VARCHAR(255),
    contact_phone VARCHAR(255),
    address TEXT,
    status VARCHAR(50) DEFAULT 'active',
    created_by VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_by VARCHAR(255),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    CONSTRAINT chk_suppliers_status CHECK (status IN ('active', 'inactive'))
);

CREATE INDEX IF NOT EXISTS idx_suppliers_deleted_at ON suppliers(deleted_at);

-- Units table
CREATE TABLE IF NOT EXISTS units (
    id SERIAL PRIMARY KEY,
    unit_type VARCHAR(50) NOT NULL,
    name VARCHAR(100) NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    conversion_factor DOUBLE PRECISION NOT NULL DEFAULT 1,
    level SMALLINT NOT NULL DEFAULT 1,
    decimal_places SMALLINT NOT NULL DEFAULT 0,
    base_unit_id INTEGER,
    created_by VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_by VARCHAR(255),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    CONSTRAINT chk_units_level CHECK (level >= 1 AND level <= 4),
    CONSTRAINT chk_units_decimal_places CHECK (decimal_places >= 0 AND decimal_places <= 10),
    CONSTRAINT fk_units_base_unit FOREIGN KEY (base_unit_id) REFERENCES units(id)
);

CREATE INDEX IF NOT EXISTS idx_units_deleted_at ON units(deleted_at);
CREATE INDEX IF NOT EXISTS idx_units_base_unit ON units(base_unit_id);

-- Products table
CREATE TABLE IF NOT EXISTS products (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    product_type VARCHAR(20),
    unit_id INTEGER NOT NULL,
    status VARCHAR(50) DEFAULT 'active',
    created_by VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_by VARCHAR(255),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    CONSTRAINT chk_products_status CHECK (status IN ('active', 'inactive')),
    CONSTRAINT fk_products_units FOREIGN KEY (unit_id) REFERENCES units(id) ON UPDATE CASCADE ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_products_deleted_at ON products(deleted_at);

-- Product-Supplier join table (many-to-many)
CREATE TABLE IF NOT EXISTS product_suppliers (
    product_id INTEGER NOT NULL,
    supplier_id INTEGER NOT NULL,
    PRIMARY KEY (product_id, supplier_id),
    CONSTRAINT fk_product_suppliers_product FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE,
    CONSTRAINT fk_product_suppliers_supplier FOREIGN KEY (supplier_id) REFERENCES suppliers(id) ON DELETE CASCADE
);

-- Inventories table
CREATE TABLE IF NOT EXISTS inventories (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    location VARCHAR(255) NOT NULL,
    status VARCHAR(50) DEFAULT 'active',
    restricted_delivery_hours VARCHAR(255),
    created_by VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_by VARCHAR(255),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_inventories_deleted_at ON inventories(deleted_at);

-- Inventory items table
CREATE TABLE IF NOT EXISTS inventory_items (
    id SERIAL PRIMARY KEY,
    inventory_id INTEGER NOT NULL,
    product_id INTEGER NOT NULL,
    quantity DECIMAL(10,2) DEFAULT 0,
    status VARCHAR(50) DEFAULT 'active',
    consuming_transaction_id INTEGER,
    created_by VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_by VARCHAR(255),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    CONSTRAINT fk_inventory_items_inventory FOREIGN KEY (inventory_id) REFERENCES inventories(id),
    CONSTRAINT fk_inventory_items_product FOREIGN KEY (product_id) REFERENCES products(id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_inventory_items_unique ON inventory_items(inventory_id, product_id);
CREATE INDEX IF NOT EXISTS idx_inventory_items_deleted_at ON inventory_items(deleted_at);

-- Inventory transactions table
CREATE TABLE IF NOT EXISTS inventory_transactions (
    id SERIAL PRIMARY KEY,
    inventory_item_id INTEGER NOT NULL,
    supplier_id INTEGER,
    transaction_type VARCHAR(50) NOT NULL,
    price DOUBLE PRECISION NOT NULL,
    quantity DECIMAL(10,2) NOT NULL,
    consumed_quantity DECIMAL(10,2) DEFAULT 0,
    counter_transaction_id INTEGER,
    purchase_order_item_id INTEGER,
    created_by VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_by VARCHAR(255),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    CONSTRAINT chk_inventory_transactions_type CHECK (transaction_type IN ('purchase', 'disposal', 'sell', 'transfer_out', 'transfer_in')),
    CONSTRAINT fk_inventory_transactions_inventory_item FOREIGN KEY (inventory_item_id) REFERENCES inventory_items(id),
    CONSTRAINT fk_inventory_transactions_supplier FOREIGN KEY (supplier_id) REFERENCES suppliers(id)
);

CREATE INDEX IF NOT EXISTS idx_inventory_transactions_deleted_at ON inventory_transactions(deleted_at);

-- Purchase orders table
CREATE TABLE IF NOT EXISTS purchase_orders (
    id SERIAL PRIMARY KEY,
    order_number VARCHAR(255) NOT NULL UNIQUE,
    inventory_id INTEGER NOT NULL,
    status VARCHAR(50) DEFAULT 'order_placed',
    notes TEXT,
    confirmed_at TIMESTAMP WITH TIME ZONE,
    confirmation_notes TEXT,
    created_by VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_by VARCHAR(255),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    CONSTRAINT chk_purchase_orders_status CHECK (status IN ('order_placed', 'partially_delivered', 'fully_delivered', 'completed', 'cancelled')),
    CONSTRAINT fk_purchase_orders_inventory FOREIGN KEY (inventory_id) REFERENCES inventories(id)
);

CREATE INDEX IF NOT EXISTS idx_purchase_orders_deleted_at ON purchase_orders(deleted_at);

-- Purchase order items table
CREATE TABLE IF NOT EXISTS purchase_order_items (
    id SERIAL PRIMARY KEY,
    purchase_order_id INTEGER,
    product_id INTEGER NOT NULL,
    supplier_id INTEGER NOT NULL,
    unit_id INTEGER NOT NULL,
    unit_price DECIMAL(13,2),
    quantity DECIMAL(10,2) NOT NULL,
    received_quantity DECIMAL(10,2) DEFAULT 0,
    status VARCHAR(50) DEFAULT 'awaiting_delivery',
    created_by VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_by VARCHAR(255),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    CONSTRAINT chk_purchase_order_items_status CHECK (status IN ('awaiting_delivery', 'partially_delivered', 'delivered', 'cancelled')),
    CONSTRAINT fk_purchase_order_items_purchase_order FOREIGN KEY (purchase_order_id) REFERENCES purchase_orders(id),
    CONSTRAINT fk_purchase_order_items_product FOREIGN KEY (product_id) REFERENCES products(id),
    CONSTRAINT fk_purchase_order_items_supplier FOREIGN KEY (supplier_id) REFERENCES suppliers(id),
    CONSTRAINT fk_purchase_order_items_unit FOREIGN KEY (unit_id) REFERENCES units(id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_product_supplier_po ON purchase_order_items(product_id, supplier_id, purchase_order_id);
CREATE INDEX IF NOT EXISTS idx_purchase_order_items_deleted_at ON purchase_order_items(deleted_at);

-- Settings table
CREATE TABLE IF NOT EXISTS settings (
    key VARCHAR(255) PRIMARY KEY,
    value JSONB
);

-- Payment receipt forms table
CREATE TABLE IF NOT EXISTS payment_receipt_forms (
    id SERIAL PRIMARY KEY,
    form_number VARCHAR(255) UNIQUE,
    purchase_order_id INTEGER NOT NULL,
    date TIMESTAMP WITH TIME ZONE NOT NULL,
    full_name VARCHAR(255),
    department VARCHAR(255),
    details TEXT,
    total_amount DOUBLE PRECISION,
    status VARCHAR(50) DEFAULT 'pending',
    created_by VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_by VARCHAR(255),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    CONSTRAINT chk_payment_receipt_forms_status CHECK (status IN ('pending', 'submitted', 'approved', 'rejected')),
    CONSTRAINT fk_payment_receipt_forms_purchase_order FOREIGN KEY (purchase_order_id) REFERENCES purchase_orders(id)
);

CREATE INDEX IF NOT EXISTS idx_payment_receipt_forms_deleted_at ON payment_receipt_forms(deleted_at);

-- Inventory submissions table
CREATE TABLE IF NOT EXISTS inventory_submissions (
    id SERIAL PRIMARY KEY,
    inventory_id INTEGER,
    submission_type VARCHAR(50) NOT NULL,
    processing_status VARCHAR(50) DEFAULT 'pending',
    approval_status VARCHAR(50) DEFAULT 'pending',
    payload JSONB,
    reason TEXT,
    error JSONB,
    created_by VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_by VARCHAR(255),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    CONSTRAINT fk_inventory_submissions_inventory FOREIGN KEY (inventory_id) REFERENCES inventories(id)
);

CREATE INDEX IF NOT EXISTS idx_inventory_submissions_deleted_at ON inventory_submissions(deleted_at);

