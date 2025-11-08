CREATE TABLE IF NOT EXISTS units (
    id SERIAL PRIMARY KEY,
    unit_type VARCHAR(50) NOT NULL,
    name VARCHAR(100) NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    conversion_factor DOUBLE PRECISION NOT NULL DEFAULT 1,
    base_unit_id INTEGER,
    created_by VARCHAR(255) NOT NULL DEFAULT 'system@cim.local',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_by VARCHAR(255) NOT NULL DEFAULT 'system@cim.local',
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

ALTER TABLE units
    ADD CONSTRAINT fk_units_base_unit
    FOREIGN KEY (base_unit_id) REFERENCES units(id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_units_unit_type_name ON units (unit_type, name);

CREATE INDEX IF NOT EXISTS idx_units_base_unit ON units(base_unit_id);

-- Add reference from products to units
ALTER TABLE products ADD COLUMN IF NOT EXISTS unit_id INTEGER;

-- Insert a default catch-all unit to ensure reference integrity
INSERT INTO units (unit_type, name, symbol, conversion_factor)
VALUES ('general', 'unit', 'unit', 1)
ON CONFLICT (unit_type, name) DO NOTHING;

-- Ensure every product has a unit reference before enforcing constraint
WITH default_unit AS (
    SELECT id FROM units ORDER BY id LIMIT 1
)
UPDATE products
SET unit_id = (SELECT id FROM default_unit)
WHERE unit_id IS NULL;

ALTER TABLE products
    ALTER COLUMN unit_id SET NOT NULL;

ALTER TABLE products
    ADD CONSTRAINT fk_products_units FOREIGN KEY (unit_id) REFERENCES units(id);

-- Drop legacy unit column
ALTER TABLE products DROP COLUMN IF EXISTS unit;

