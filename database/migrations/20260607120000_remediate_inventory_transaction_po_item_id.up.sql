-- Remediate inventory_transactions.purchase_order_item_id corrupted by the
-- pre-#7 ReceiveInventory bug (commit d2a47c1), which wrote the PURCHASE ORDER
-- id into the PURCHASE ORDER ITEM id column. Every value written before the fix
-- deployed is therefore a PO id, not a PO-item id.
--
-- Scope is bounded to the corruption window by created_at. The fix shipped to
-- prod as release v2.0.0 on 2026-04-18 ~16:34 UTC (23:34 GMT+7) via the
-- Production Release workflow; the old, buggy code ran until that deploy. The
-- cutoff below is set just after it. On prod there are zero purchase
-- transactions between the last corrupt row (2026-04-18 08:58 UTC) and the next
-- purchase (2026-04-19 01:21 UTC), so the deploy lands in an empty gap and any
-- cutoff inside it separates pre-fix from post-fix cleanly. This positive bound
-- -- not the empirical "POI ids sit above PO ids" -- is what guarantees post-fix
-- rows are never rewritten (a valid post-fix POI id could coincide with a real
-- PO id elsewhere and would otherwise be matched).
--
-- Correct PO item = the unique, non-deleted line in the PO named by the stored
-- value (reinterpreted as a PO id) matching the transaction's product AND
-- supplier. PO items are unique on (purchase_order_id, product_id, supplier_id)
-- and ReceiveInventory persisted supplier_id onto the transaction, so matching
-- all three is exactly the original key -- and stays correct if the original
-- line was soft-deleted and the PO has a different-supplier same-product line.
--
-- We deliberately do NOT gate on "the current POI has a different product":
-- when the corrupt PO id coincidentally equals a POI id of the SAME product
-- (40 rows on prod) the product check looks satisfied yet the row still points
-- at the wrong PO/line; reading the stored value as a PO id catches those too.
--
-- Of ~9,957 pre-fix purchase transactions, 9,955 resolve to a unique POI (9,952
-- change value; 3 already hold it). The other 2 are zero-quantity no-ops whose
-- line was soft-deleted (txn 77, 78) and are left untouched.
--
-- One-way data fix (the .down migration is intentionally a no-op).

WITH resolved AS (
    SELECT b.txn_id, (array_agg(poi.id))[1] AS new_poi_id
    FROM (
        SELECT it.id                    AS txn_id,
               it.purchase_order_item_id AS stored_value,    -- actually a PO id
               ii.product_id             AS item_product,
               it.supplier_id            AS item_supplier
        FROM inventory_transactions it
        JOIN inventory_items ii ON ii.id = it.inventory_item_id
        WHERE it.transaction_type = 'purchase'
          AND it.deleted_at IS NULL
          AND it.purchase_order_item_id IS NOT NULL
          AND it.created_at < TIMESTAMPTZ '2026-04-18 17:00:00+00'   -- just after v2.0.0 prod release (~16:34 UTC 2026-04-18)
    ) b
    JOIN purchase_order_items poi
          ON poi.purchase_order_id = b.stored_value          -- reinterpret stored value as a PO id
         AND poi.product_id        = b.item_product
         AND poi.supplier_id       = b.item_supplier         -- match original supplier (PO items are supplier-scoped)
         AND poi.deleted_at IS NULL
    GROUP BY b.txn_id
    HAVING count(*) = 1                                        -- unique match only (no ambiguity)
)
UPDATE inventory_transactions it
SET purchase_order_item_id = r.new_poi_id,
    updated_at = now(),
    updated_by = 'migration 20260607: remediate purchase order id stored as purchase order item id'
FROM resolved r
WHERE it.id = r.txn_id
  AND it.purchase_order_item_id <> r.new_poi_id;             -- skip no-op rows; idempotent
