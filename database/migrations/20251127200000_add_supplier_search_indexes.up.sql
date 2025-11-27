-- Add indexes for supplier name search optimization
-- This improves performance of searching purchase orders by supplier name

-- Index for purchase_order_id lookups in EXISTS subquery
-- This allows fast lookups when checking if a purchase order has items with matching suppliers
CREATE INDEX IF NOT EXISTS idx_purchase_order_items_purchase_order_id 
ON purchase_order_items(purchase_order_id);

-- Enable pg_trgm extension for trigram text search (if not already enabled)
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Trigram index for ILIKE text search on supplier names
-- This enables efficient pattern matching even with leading wildcards
-- Using GIN instead of GiST because:
-- 1. GIN is faster for LIKE/ILIKE pattern matching queries (our use case)
-- 2. Suppliers table is relatively static (infrequent updates)
-- 3. GIN provides better lookup performance for EXISTS subqueries
-- 4. GIN indexes are more efficient for exact and pattern matches
-- Note: GiST would be better if we needed distance-based queries (<-> operators)
-- Reference: https://www.postgresql.org/docs/current/pgtrgm.html
CREATE INDEX IF NOT EXISTS idx_suppliers_name_trgm 
ON suppliers USING gin(name gin_trgm_ops);


