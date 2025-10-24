-- Remove unique constraint on (product_id, supplier_id, purchase_order_id) combination
DROP INDEX IF EXISTS idx_product_supplier_po;

