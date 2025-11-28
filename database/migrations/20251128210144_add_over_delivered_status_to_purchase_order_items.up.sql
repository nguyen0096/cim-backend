-- Add over_delivered status to purchase_order_items status constraint
ALTER TABLE purchase_order_items
DROP CONSTRAINT IF EXISTS chk_purchase_order_items_status;

ALTER TABLE purchase_order_items
ADD CONSTRAINT chk_purchase_order_items_status 
CHECK (status IN ('awaiting_delivery', 'partially_delivered', 'delivered', 'over_delivered', 'cancelled'));

