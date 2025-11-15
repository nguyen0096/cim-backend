-- Remove check constraints
ALTER TABLE units
DROP CONSTRAINT IF EXISTS chk_units_decimal_places;

ALTER TABLE units
DROP CONSTRAINT IF EXISTS chk_units_level;

-- Remove columns
ALTER TABLE units
DROP COLUMN IF EXISTS decimal_places;

ALTER TABLE units
DROP COLUMN IF EXISTS level;

