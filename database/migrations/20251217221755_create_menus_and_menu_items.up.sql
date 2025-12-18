-- Create menus table
CREATE TABLE IF NOT EXISTS menus (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    created_by VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_by VARCHAR(255),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_menus_deleted_at ON menus(deleted_at);

-- Create menu_items table
CREATE TABLE IF NOT EXISTS menu_items (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    created_by VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_by VARCHAR(255),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_menu_items_deleted_at ON menu_items(deleted_at);

-- Create menu_menu_items join table (many-to-many between Menu and MenuItem)
CREATE TABLE IF NOT EXISTS menu_menu_items (
    menu_id INTEGER NOT NULL,
    menu_item_id INTEGER NOT NULL,
    PRIMARY KEY (menu_id, menu_item_id),
    CONSTRAINT fk_menu_menu_items_menu FOREIGN KEY (menu_id) REFERENCES menus(id) ON DELETE CASCADE,
    CONSTRAINT fk_menu_menu_items_menu_item FOREIGN KEY (menu_item_id) REFERENCES menu_items(id) ON DELETE CASCADE
);

-- Create menu_inventories join table (many-to-many between Menu and Inventory)
CREATE TABLE IF NOT EXISTS menu_inventories (
    menu_id INTEGER NOT NULL,
    inventory_id INTEGER NOT NULL,
    PRIMARY KEY (menu_id, inventory_id),
    CONSTRAINT fk_menu_inventories_menu FOREIGN KEY (menu_id) REFERENCES menus(id) ON DELETE CASCADE,
    CONSTRAINT fk_menu_inventories_inventory FOREIGN KEY (inventory_id) REFERENCES inventories(id) ON DELETE CASCADE
);

-- Create menu_item_products join table (many-to-many between MenuItem and Product)
CREATE TABLE IF NOT EXISTS menu_item_products (
    menu_item_id INTEGER NOT NULL,
    product_id INTEGER NOT NULL,
    PRIMARY KEY (menu_item_id, product_id),
    CONSTRAINT fk_menu_item_products_menu_item FOREIGN KEY (menu_item_id) REFERENCES menu_items(id) ON DELETE CASCADE,
    CONSTRAINT fk_menu_item_products_product FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE
);
