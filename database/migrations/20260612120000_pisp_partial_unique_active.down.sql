-- Re-add the original NON-partial column-level UNIQUE on purchase_order_item_id.
--
-- This down runs BEFORE 20260612110000's down (which drops the partial index
-- CONCURRENTLY), so PO-item uniqueness is re-established here before the partial
-- index goes away — no window without uniqueness enforcement during rollback.
--
-- ROLLBACK CAVEAT (NOT auto-handled here, by design):
-- The NON-partial UNIQUE counts ALL rows, including soft-deleted ones, whereas
-- the partial index this PR introduced only counts live (deleted_at IS NULL)
-- rows. So this PR's runtime can legitimately produce TWO pisp rows for the same
-- purchase_order_item_id: one soft-deleted plus one live (e.g. soft-delete then
-- re-insert via apply/upsert). If any such duplicate purchase_order_item_id pair
-- exists, ADD CONSTRAINT below will FAIL with a unique violation.
--
-- Rolling back therefore requires that there be NO duplicate purchase_order_item_id
-- values across all (live + soft-deleted) pisp rows. Any such duplicates must be
-- cleaned up first — e.g. hard-deleting the redundant soft-deleted row(s) for each
-- affected PO item:
--   DELETE FROM purchase_order_item_selling_prices a
--   USING purchase_order_item_selling_prices b
--   WHERE a.purchase_order_item_id = b.purchase_order_item_id
--     AND a.deleted_at IS NOT NULL          -- only remove soft-deleted dupes
--     AND b.id <> a.id;
-- This cleanup is intentionally NOT run automatically: it is destructive and the
-- operator must decide which rows to remove. Run it manually before this down
-- migration if the ADD CONSTRAINT fails.
ALTER TABLE purchase_order_item_selling_prices
    ADD CONSTRAINT purchase_order_item_selling_prices_purchase_order_item_id_key
    UNIQUE (purchase_order_item_id);
