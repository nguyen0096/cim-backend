-- Remove indexes added for supplier name search optimization

-- Drop trigram index on suppliers.name
DROP INDEX IF EXISTS idx_suppliers_name_trgm;

-- Drop index on purchase_order_items.purchase_order_id
DROP INDEX IF EXISTS idx_purchase_order_items_purchase_order_id;

-- Note: We don't drop the pg_trgm extension as it might be used by other parts of the system

