-- Drop the original NON-partial uniqueness on purchase_order_item_selling_prices
-- (pisp).purchase_order_item_id now that the PARTIAL unique index
-- (uq_pisp_po_item_id_active, created in 20260612110000) enforces live-row
-- uniqueness.
--
-- The original create migration (20260412144406) declared an inline column-level
-- UNIQUE on purchase_order_item_id, which is NOT partial: it counts soft-deleted
-- rows. That conflicts with the selling-price apply/upsert logic, which scopes
-- counting and writing to live rows (deleted_at IS NULL). A non-partial unique
-- (a) lets a soft-deleted pisp row block re-inserting a live row for the same PO
-- item (hard unique violation), and (b) makes the massive-apply counted set drift
-- from the rows actually written.
--
-- ORDERING: the partial index is created CONCURRENTLY in the prior migration
-- (20260612110000) and exists by the time this runs, so dropping the old
-- constraint here never leaves PO-item uniqueness unenforced.
--
-- That create migration has already reached main / been applied, so it is NOT
-- edited in place; this forward migration only drops the non-partial constraint.

-- Drop the inline column-level UNIQUE (Postgres auto-names it <table>_<col>_key).
ALTER TABLE purchase_order_item_selling_prices
    DROP CONSTRAINT IF EXISTS purchase_order_item_selling_prices_purchase_order_item_id_key;

-- Defensive: if a same-named plain unique index exists (e.g. created by a GORM
-- AutoMigrate in a test/dev environment) rather than a table constraint, drop it
-- too so the partial index can take over cleanly.
DROP INDEX IF EXISTS purchase_order_item_selling_prices_purchase_order_item_id_key;
