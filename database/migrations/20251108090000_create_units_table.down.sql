-- Restore legacy product unit column and remove units table
ALTER TABLE products ADD COLUMN IF NOT EXISTS unit VARCHAR(20) NOT NULL DEFAULT '';

UPDATE products p
SET unit = COALESCE(u.symbol, u.name, '')
FROM units u
WHERE p.unit_id = u.id;

ALTER TABLE products DROP CONSTRAINT IF EXISTS fk_products_units;
ALTER TABLE products DROP COLUMN IF EXISTS unit_id;

DROP INDEX IF EXISTS idx_units_base_unit;

ALTER TABLE units
    DROP CONSTRAINT IF EXISTS fk_units_base_unit;

DROP INDEX IF EXISTS idx_units_unit_type_name;
DROP TABLE IF EXISTS units;

