-- Add index on inventory_transactions(inventory_item_id).
-- Prerequisite for the #46 reconcile tool's prod queries, which filter/join
-- inventory_transactions by inventory_item_id (flagged as missing in review).
--
-- IMPORTANT: CREATE INDEX CONCURRENTLY cannot run inside a transaction block.
-- This repo's golang-migrate v4 postgres driver (database/database.go ->
-- runStatement -> conn.ExecContext) does NOT wrap a single-statement migration
-- in a transaction, so CONCURRENTLY works here. This file therefore contains
-- exactly ONE statement on purpose -- do not add more statements to it, and do
-- not enable x-multi-statement, or the implicit transaction will break it.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_inventory_transactions_inventory_item_id
ON inventory_transactions (inventory_item_id);
