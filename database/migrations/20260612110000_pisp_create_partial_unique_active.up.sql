-- Create the PARTIAL unique index on purchase_order_item_selling_prices (pisp):
-- a PO item may have at most one LIVE (deleted_at IS NULL) pisp row. Soft-deleted
-- rows are ignored, so re-inserting after a soft delete is allowed and the
-- apply/upsert "deleted_at IS NULL"-scoped logic stays consistent.
--
-- WHY CONCURRENTLY / WHY A SEPARATE MIGRATION:
-- The selling-price write paths (UpsertPOItemSellingPrice, BackfillPOItems,
-- createPOItemSellingPrices) are already live on main, so pisp may hold rows in
-- production. A plain CREATE UNIQUE INDEX takes an ACCESS EXCLUSIVE-style build
-- lock that blocks writes to the table for the duration of the build; CONCURRENTLY
-- builds without blocking concurrent writes.
--
-- CONCURRENTLY cannot run inside a transaction block. The golang-migrate postgres
-- driver runs each migration file as a single ExecContext with no surrounding
-- BEGIN/COMMIT, so this file MUST contain exactly ONE statement (no other DDL,
-- no comments-as-statements) for the CONCURRENTLY build to succeed. The DROP of
-- the old non-partial constraint lives in the next migration (20260612120000),
-- which runs AFTER this index exists — creating the new index before dropping the
-- old constraint avoids any window with no uniqueness enforcement on PO items.
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_pisp_po_item_id_active
    ON purchase_order_item_selling_prices (purchase_order_item_id)
    WHERE deleted_at IS NULL;
