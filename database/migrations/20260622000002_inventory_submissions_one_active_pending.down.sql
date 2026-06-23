-- Drop the one-active-pending index. CONCURRENTLY (no write lock) cannot run in a
-- transaction, so this file holds exactly ONE statement.
DROP INDEX CONCURRENTLY IF EXISTS uq_inventory_submissions_one_active_pending;
