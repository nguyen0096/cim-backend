CREATE TABLE IF NOT EXISTS selling_prices (
    id SERIAL PRIMARY KEY,
    product_id INTEGER NOT NULL REFERENCES products(id),
    inventory_id INTEGER REFERENCES inventories(id),
    price NUMERIC(13,2) NOT NULL,
    effective_from DATE NOT NULL,
    notes TEXT,
    created_by VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_by VARCHAR(255),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_selling_prices_product_effective
    ON selling_prices(product_id, effective_from DESC) WHERE deleted_at IS NULL;
