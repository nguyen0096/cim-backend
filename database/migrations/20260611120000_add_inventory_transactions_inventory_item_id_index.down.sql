-- Drop index on inventory_transactions(inventory_item_id).
-- DROP INDEX CONCURRENTLY also cannot run inside a transaction; this file
-- contains exactly ONE statement so the non-transactional driver path applies.
DROP INDEX CONCURRENTLY IF EXISTS idx_inventory_transactions_inventory_item_id;
