-- Create sale_orders table
CREATE TABLE IF NOT EXISTS sale_orders (
    id SERIAL PRIMARY KEY,
    previous_order_id INTEGER,
    is_latest BOOLEAN DEFAULT true,
    customer_id VARCHAR(26) NOT NULL,
    tag INTEGER DEFAULT 0,
    order_number VARCHAR(255) NOT NULL,
    inventory_id INTEGER NOT NULL,
    status VARCHAR(50) DEFAULT 'ordered',
    notes TEXT,
    created_by VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_by VARCHAR(255),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    CONSTRAINT chk_sale_orders_status CHECK (status IN ('ordered', 'served', 'completed', 'cancelled')),
    CONSTRAINT fk_sale_orders_previous_order FOREIGN KEY (previous_order_id) REFERENCES sale_orders(id),
    CONSTRAINT fk_sale_orders_inventory FOREIGN KEY (inventory_id) REFERENCES inventories(id)
);

CREATE INDEX IF NOT EXISTS idx_sale_orders_deleted_at ON sale_orders(deleted_at);
CREATE INDEX IF NOT EXISTS idx_sale_orders_customer_id ON sale_orders(customer_id);
CREATE INDEX IF NOT EXISTS idx_sale_orders_tag ON sale_orders(tag);
CREATE INDEX IF NOT EXISTS idx_sale_orders_order_number ON sale_orders(order_number);
CREATE INDEX IF NOT EXISTS idx_sale_orders_previous_order_id ON sale_orders(previous_order_id);
CREATE INDEX IF NOT EXISTS idx_sale_orders_inventory_id ON sale_orders(inventory_id);

-- Create sale_order_items table
CREATE TABLE IF NOT EXISTS sale_order_items (
    id SERIAL PRIMARY KEY,
    sale_order_id INTEGER NOT NULL,
    created_by VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_by VARCHAR(255),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    CONSTRAINT fk_sale_order_items_sale_order FOREIGN KEY (sale_order_id) REFERENCES sale_orders(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_sale_order_items_deleted_at ON sale_order_items(deleted_at);
CREATE INDEX IF NOT EXISTS idx_sale_order_items_sale_order_id ON sale_order_items(sale_order_id);

-- Create sale_order_item_menu_items join table (many-to-many between SaleOrderItem and MenuItem)
CREATE TABLE IF NOT EXISTS sale_order_item_menu_items (
    sale_order_item_id INTEGER NOT NULL,
    menu_item_id INTEGER NOT NULL,
    PRIMARY KEY (sale_order_item_id, menu_item_id),
    CONSTRAINT fk_sale_order_item_menu_items_sale_order_item FOREIGN KEY (sale_order_item_id) REFERENCES sale_order_items(id) ON DELETE CASCADE,
    CONSTRAINT fk_sale_order_item_menu_items_menu_item FOREIGN KEY (menu_item_id) REFERENCES menu_items(id) ON DELETE CASCADE
);
