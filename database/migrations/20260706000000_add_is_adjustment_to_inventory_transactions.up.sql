-- Flags a zero-cost reconcile-correction stock layer (reconcile_stock_up, or a
-- transfer_in that consumed such a layer) so the in/out export can attribute
-- found stock across any number of transfer hops without a cross-inventory
-- counter-chain walk. A constant DEFAULT is a metadata-only add in Postgres
-- (no table rewrite); the brief ACCESS EXCLUSIVE lock only updates the catalog.
ALTER TABLE inventory_transactions
    ADD COLUMN IF NOT EXISTS is_adjustment boolean NOT NULL DEFAULT false;
