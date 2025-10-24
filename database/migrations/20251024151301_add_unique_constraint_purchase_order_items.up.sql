-- Add unique constraint on (product_id, supplier_id, purchase_order_id) combination
CREATE UNIQUE INDEX IF NOT EXISTS idx_product_supplier_po ON purchase_order_items(product_id, supplier_id, purchase_order_id, deleted_at);

