-- Drop index on inventory_submissions(inventory_id).
-- DROP INDEX CONCURRENTLY also cannot run inside a transaction; this file
-- contains exactly ONE statement so the non-transactional driver path applies.
DROP INDEX CONCURRENTLY IF EXISTS idx_inventory_submissions_inventory_id;
