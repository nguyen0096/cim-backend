-- ============================================================================
-- local_seed.sql — generate VALID transactional test data for local testing.
--
-- Run AFTER `make migrate` and `make seed` (which create the base reference data:
-- units, suppliers, products, inventories, menus, sale_orders, users).
--
-- Re-runnable: it TRUNCATEs the transactional tables and rebuilds them. It never
-- touches base reference data.
--
--   docker exec -i cim_postgres psql -U postgres -d cim_db -v ON_ERROR_STOP=1 \
--     < test/data/local_seed.sql
--
-- INVARIANTS this script preserves (enforced by the app, see
-- InventoryItem.ValidateActivePurchaseTransactions):
--   * on-hand: inventory_items.quantity == SUM(lot.quantity - lot.consumed_quantity)
--     over that item's purchase/transfer_in lots.
--   * every consume (sell/disposal) sets counter_transaction_id to the source
--     purchase lot and increments that lot's consumed_quantity.
--   * a lot is never over-consumed (consumed_quantity <= quantity).
--   * purchase unit cost < selling price (positive margin).
--   * the daily "purchases in period" export columns need purchase txns DATED
--     INSIDE the window — so we scatter in-window purchase lots across recent days.
-- Transfers are intentionally omitted (real transfers need cross-inventory lot
-- pairing that should be created through the API, not raw SQL).
-- ============================================================================
BEGIN;

TRUNCATE TABLE
  purchase_order_item_selling_prices,
  payment_receipt_forms,
  inventory_transactions,
  inventory_items,
  purchase_order_items,
  purchase_orders,
  inventory_submissions,
  revenue_expense_finalizations,
  menu_inventories,
  unit_conversions
RESTART IDENTITY CASCADE;

-- 1. Purchase orders (500), spread over ~180 days, valid statuses.
INSERT INTO purchase_orders (order_number, inventory_id, status, notes, confirmed_at,
       created_by, created_at, updated_by, updated_at)
SELECT 'PO-2026-' || lpad(g::text, 5, '0'), (g % 3) + 1,
       (ARRAY['order_placed','partially_delivered','fully_delivered','completed','cancelled'])[(g % 5) + 1],
       'Seeded purchase order ' || g,
       CASE WHEN g % 5 IN (2,3,4) THEN now() - ((random()*150)::int || ' days')::interval END,
       'seeder@test.com', now() - ((random()*180)::int || ' days')::interval, 'seeder@test.com', now()
FROM generate_series(1, 500) g;

-- 2. PO items (2 per PO; distinct product per item keeps the unique index happy).
--    Realistic unit cost 10..200.
INSERT INTO purchase_order_items (purchase_order_id, product_id, supplier_id, unit_id,
       unit_price, quantity, received_quantity, status,
       created_by, created_at, updated_by, updated_at)
SELECT po.id, p.id, ((po.id + k) % 10) + 1, p.unit_id,
       round((10 + random()*190)::numeric, 2),
       q.quantity,
       round((q.quantity * CASE po.status WHEN 'fully_delivered' THEN 1
                                          WHEN 'partially_delivered' THEN 0.5 ELSE 0 END)::numeric, 2),
       (ARRAY['awaiting_delivery','partially_delivered','delivered','over_delivered','cancelled'])[((po.id + k) % 5) + 1],
       'seeder@test.com', po.created_at, 'seeder@test.com', now()
FROM purchase_orders po
CROSS JOIN generate_series(0, 1) k
JOIN LATERAL (SELECT id, unit_id FROM products WHERE id = ((po.id*2 + k) % 108) + 1) p ON true
CROSS JOIN LATERAL (SELECT round((random()*100 + 1)::numeric, 2) AS quantity) q;

-- 3. Selling price per PO item = cost * 1.15..1.50 (positive margin).
INSERT INTO purchase_order_item_selling_prices (purchase_order_item_id, selling_price,
       created_by, created_at, updated_by, updated_at)
SELECT id, round((unit_price * (1.15 + random()*0.35))::numeric, 2),
       'seeder@test.com', created_at, 'seeder@test.com', now()
FROM purchase_order_items;

-- 4. Payment receipt per PO; total = actual line totals.
INSERT INTO payment_receipt_forms (form_number, purchase_order_id, date, full_name,
       department, details, total_amount, status, created_by, created_at, updated_by, updated_at)
SELECT 'PRF-' || lpad(po.id::text, 6, '0'), po.id, po.created_at, 'Nguyen Van ' || po.id,
       (ARRAY['Kitchen','Procurement','Finance','Warehouse'])[(po.id % 4) + 1],
       'Payment for ' || po.order_number,
       COALESCE((SELECT sum(poi.quantity * poi.unit_price) FROM purchase_order_items poi
                 WHERE poi.purchase_order_id = po.id), 0)::double precision,
       (ARRAY['pending','submitted','approved','rejected'])[(po.id % 4) + 1],
       'seeder@test.com', po.created_at, 'seeder@test.com', now()
FROM purchase_orders po;

-- 5. Inventory items: every product in every inventory (quantity reconciled in step 11).
INSERT INTO inventory_items (inventory_id, product_id, quantity, status, unit_id,
       created_by, created_at, updated_by, updated_at)
SELECT inv.n, p.id, 0, 'active', p.unit_id, 'seeder@test.com',
       now() - '180 days'::interval, 'seeder@test.com', now()
FROM products p CROSS JOIN (VALUES (1),(2),(3)) AS inv(n);

-- 6. Misc transactional tables.
INSERT INTO inventory_submissions (inventory_id, submission_type, processing_status,
       approval_status, payload, reason, created_by, created_at, updated_by, updated_at)
SELECT (g % 3) + 1, (ARRAY['stock_in','stock_out','adjustment','count'])[(g % 4) + 1],
       (ARRAY['pending','processing','completed','failed'])[(g % 4) + 1],
       (ARRAY['pending','approved','rejected'])[(g % 3) + 1],
       jsonb_build_object('items', g, 'note', 'seeded'), 'Seeded submission ' || g,
       'seeder@test.com', now() - ((random()*180)::int || ' days')::interval, 'seeder@test.com', now()
FROM generate_series(1, 500) g;

INSERT INTO revenue_expense_finalizations (finalized_date, status, reason,
       created_by, created_at, updated_by, updated_at)
SELECT (current_date - g)::timestamptz, (ARRAY['finalized','pending','reopened'])[(g % 3) + 1],
       'Daily finalization ' || g, 'seeder@test.com', now() - (g || ' days')::interval, 'seeder@test.com', now()
FROM generate_series(1, 200) g;

INSERT INTO menu_inventories (menu_id, inventory_id)
SELECT m.id, i.id FROM menus m CROSS JOIN inventories i ON CONFLICT DO NOTHING;

INSERT INTO unit_conversions (from_unit_id, to_unit_id, conversion_factor)
VALUES (4,5,2.0),(6,7,1000.0),(8,9,0.5),(10,11,12.0),(12,13,24.0),(14,15,100.0)
ON CONFLICT DO NOTHING;

-- ----------------------------------------------------------------------------
-- 7. LEDGER. Opening lot per item (before window) gives beginning stock.
--    POI preferred from a PO of the same inventory; else any PO for the product.
INSERT INTO inventory_transactions (inventory_item_id, supplier_id, transaction_type,
       price, quantity, consumed_quantity, counter_transaction_id, purchase_order_item_id,
       created_by, created_at, updated_by, updated_at)
SELECT it.id, poi.supplier_id, 'purchase', poi.unit_price::double precision,
       round((800 + random()*1200)::numeric, 2), 0, NULL, poi.id,
       'seeder@test.com', now() - '180 days'::interval - ((random()*12)::int || ' hours')::interval,
       'seeder@test.com', now()
FROM inventory_items it
JOIN LATERAL (
    SELECT poi.id, poi.supplier_id, poi.unit_price
    FROM purchase_order_items poi JOIN purchase_orders po ON po.id = poi.purchase_order_id
    WHERE poi.product_id = it.product_id
    ORDER BY (po.inventory_id = it.inventory_id) DESC, poi.id LIMIT 1
) poi ON true;

-- 8. Consumes (sell/disposal, 600), FIFO-linked to the item's opening lot.
--    At this point the only purchase txns are the openings, so the offset picks one.
INSERT INTO inventory_transactions (inventory_item_id, supplier_id, transaction_type,
       price, quantity, consumed_quantity, counter_transaction_id, purchase_order_item_id,
       created_by, created_at, updated_by, updated_at)
SELECT lot.inventory_item_id, NULL, tt.t,
       round((CASE tt.t WHEN 'sell' THEN COALESCE(sp.selling_price, poi.unit_price * 1.3)
                        ELSE poi.unit_price END)::numeric, 2)::double precision,
       round((random()*30 + 1)::numeric, 2), 0, lot.id, NULL,
       'seeder@test.com', now() - ((g % 165) || ' days')::interval - ((g * 13 % 24) || ' hours')::interval,
       'seeder@test.com', now()
FROM generate_series(1, 600) g
JOIN LATERAL (
    SELECT t.id, t.inventory_item_id, t.purchase_order_item_id
    FROM inventory_transactions t WHERE t.transaction_type = 'purchase'
    ORDER BY t.id OFFSET ((g * 7) % 324) LIMIT 1
) lot ON true
CROSS JOIN LATERAL (SELECT (ARRAY['sell','disposal'])[(g % 2) + 1] AS t) tt
JOIN purchase_order_items poi ON poi.id = lot.purchase_order_item_id
LEFT JOIN purchase_order_item_selling_prices sp ON sp.purchase_order_item_id = lot.purchase_order_item_id;

-- 9. In-window purchase lots (400) scattered across the last ~150 days. These are
--    NEW lots (consumed_quantity 0) sharing the product's POI so each product's
--    row shows beginning stock AND daily purchases. This is what fills the day cells.
INSERT INTO inventory_transactions (inventory_item_id, supplier_id, transaction_type,
       price, quantity, consumed_quantity, counter_transaction_id, purchase_order_item_id,
       created_by, created_at, updated_by, updated_at)
SELECT it.id, poi.supplier_id, 'purchase', poi.unit_price::double precision,
       round((20 + random()*80)::numeric, 2), 0, NULL, poi.id,
       'seeder@test.com', now() - ((g % 150) || ' days')::interval - ((g * 7 % 24) || ' hours')::interval,
       'seeder@test.com', now()
FROM generate_series(1, 1200) g
JOIN inventory_items it ON it.id = ((g * 7) % 324) + (SELECT min(id) FROM inventory_items)
                       AND it.product_id NOT IN (1,2,3,4,5)   -- reserved for the price-change demo (step 9b)
JOIN LATERAL (
    SELECT poi.id, poi.supplier_id, poi.unit_price
    FROM purchase_order_items poi JOIN purchase_orders po ON po.id = poi.purchase_order_id
    WHERE poi.product_id = it.product_id
    ORDER BY (po.inventory_id = it.inventory_id) DESC, poi.id LIMIT 1
) poi ON true;

-- 9b. SELLING-PRICE-CHANGE demo. For products 1..5 in each inventory:
--     PO item "A" (the item's existing lot POI) gets an APRIL purchase at a lower
--     selling price; a different PO item "B" gets a MAY purchase at a higher
--     selling price. In an Apr..May export the product shows two rows at two
--     different selling prices => a price change within the report duration.
CREATE TEMP TABLE demo_price_change ON COMMIT DROP AS
SELECT it.id AS item_id, it.product_id, it.inventory_id,
       a.id AS poi_a, a.supplier_id AS sup_a, a.unit_price AS up_a,
       b.id AS poi_b, b.supplier_id AS sup_b, b.unit_price AS up_b
FROM inventory_items it
JOIN LATERAL (   -- A = same POI the opening lot uses (so April purchase shares that row)
    SELECT poi.id, poi.supplier_id, poi.unit_price
    FROM purchase_order_items poi JOIN purchase_orders po ON po.id = poi.purchase_order_id
    WHERE poi.product_id = it.product_id
    ORDER BY (po.inventory_id = it.inventory_id) DESC, poi.id LIMIT 1
) a ON true
JOIN LATERAL (   -- B = a different PO item for the same product
    SELECT poi.id, poi.supplier_id, poi.unit_price
    FROM purchase_order_items poi
    WHERE poi.product_id = it.product_id AND poi.id <> a.id
    ORDER BY poi.id LIMIT 1
) b ON true
WHERE it.product_id IN (1,2,3,4,5);

-- A = the early (April) price; B = a clear 40% increase, so every demo product
-- shows the selling price RISING within the window (B both relative to A).
UPDATE purchase_order_item_selling_prices sp SET selling_price = round((d.up_a*1.20)::numeric,2)
FROM demo_price_change d WHERE sp.purchase_order_item_id = d.poi_a;
UPDATE purchase_order_item_selling_prices sp SET selling_price = round((d.up_a*1.20*1.40)::numeric,2)
FROM demo_price_change d WHERE sp.purchase_order_item_id = d.poi_b;

INSERT INTO inventory_transactions (inventory_item_id, supplier_id, transaction_type,
       price, quantity, consumed_quantity, counter_transaction_id, purchase_order_item_id,
       created_by, created_at, updated_by, updated_at)
SELECT item_id, sup_a, 'purchase', up_a::double precision, 40, 0, NULL::int, poi_a,
       'seeder@test.com', timestamptz '2026-04-10 09:00:00+07', 'seeder@test.com', now()
FROM demo_price_change
UNION ALL
SELECT item_id, sup_b, 'purchase', up_b::double precision, 50, 0, NULL::int, poi_b,
       'seeder@test.com', timestamptz '2026-05-15 09:00:00+07', 'seeder@test.com', now()
FROM demo_price_change;

-- 10. Roll consumed quantities onto lots.
UPDATE inventory_transactions lot
SET consumed_quantity = COALESCE((SELECT sum(c.quantity) FROM inventory_transactions c
                                  WHERE c.counter_transaction_id = lot.id), 0)
WHERE lot.transaction_type = 'purchase';

-- 11. Reconcile on-hand to the ledger; consuming_transaction_id = oldest lot.
UPDATE inventory_items ii
SET quantity = COALESCE((SELECT sum(t.quantity - t.consumed_quantity)
                         FROM inventory_transactions t
                         WHERE t.inventory_item_id = ii.id
                           AND t.transaction_type IN ('purchase','transfer_in')), 0),
    consuming_transaction_id = (SELECT t.id FROM inventory_transactions t
                                WHERE t.inventory_item_id = ii.id AND t.transaction_type = 'purchase'
                                ORDER BY t.created_at ASC, t.id ASC LIMIT 1),
    updated_at = now();

COMMIT;
