-- Add index on inventory_submissions(inventory_id).
-- Prerequisite for the #46 reconcile tool's prod queries, which filter/join
-- inventory_submissions by inventory_id (flagged as missing in review).
--
-- IMPORTANT: CREATE INDEX CONCURRENTLY cannot run inside a transaction block.
-- This repo's golang-migrate v4 postgres driver (database/database.go ->
-- runStatement -> conn.ExecContext) does NOT wrap a single-statement migration
-- in a transaction, so CONCURRENTLY works here. This file therefore contains
-- exactly ONE statement on purpose -- do not add more statements to it, and do
-- not enable x-multi-statement, or the implicit transaction will break it.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_inventory_submissions_inventory_id
ON inventory_submissions (inventory_id);
