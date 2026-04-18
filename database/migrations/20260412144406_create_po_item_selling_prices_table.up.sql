CREATE TABLE IF NOT EXISTS purchase_order_item_selling_prices (
    id SERIAL PRIMARY KEY,
    purchase_order_item_id INTEGER NOT NULL UNIQUE REFERENCES purchase_order_items(id),
    selling_price NUMERIC(13,2),
    selling_price_id INTEGER REFERENCES selling_prices(id),
    created_by VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_by VARCHAR(255),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);
