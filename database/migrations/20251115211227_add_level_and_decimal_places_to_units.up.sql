-- Add level column to units table
ALTER TABLE units
ADD COLUMN IF NOT EXISTS level SMALLINT;

-- Add decimal_places column to units table
ALTER TABLE units
ADD COLUMN IF NOT EXISTS decimal_places SMALLINT;

-- Calculate and set level for existing units based on hierarchy
-- Base units (no base_unit_id) get level 1
UPDATE units
SET level = 1
WHERE base_unit_id IS NULL;

-- Calculate levels for derived units using iterative approach
-- This handles nested hierarchies of any depth up to 4 levels
DO $$
DECLARE
    updated_count INTEGER;
    iteration INTEGER := 0;
BEGIN
    LOOP
        -- Update units whose base unit has a level set
        UPDATE units u1
        SET level = u2.level + 1
        FROM units u2
        WHERE u1.base_unit_id = u2.id 
          AND u2.level IS NOT NULL
          AND u1.level IS NULL
          AND u2.level < 4; -- Prevent exceeding max level
        
        GET DIAGNOSTICS updated_count = ROW_COUNT;
        iteration := iteration + 1;
        
        -- Exit if no more updates or too many iterations (safety check)
        EXIT WHEN updated_count = 0 OR iteration > 10;
    END LOOP;
END $$;

-- Set default value for any remaining NULL levels (shouldn't happen, but safety check)
-- This handles any edge cases or circular references
UPDATE units
SET level = 1
WHERE level IS NULL;

-- Set default value for decimal_places (0 for existing rows)
UPDATE units
SET decimal_places = 0
WHERE decimal_places IS NULL;

-- Make level NOT NULL and add check constraint
ALTER TABLE units
ALTER COLUMN level SET NOT NULL,
ALTER COLUMN level SET DEFAULT 1;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chk_units_level'
    ) THEN
        ALTER TABLE units
        ADD CONSTRAINT chk_units_level CHECK (level >= 1 AND level <= 4);
    END IF;
END $$;

-- Make decimal_places NOT NULL and add check constraint
ALTER TABLE units
ALTER COLUMN decimal_places SET NOT NULL,
ALTER COLUMN decimal_places SET DEFAULT 0;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chk_units_decimal_places'
    ) THEN
        ALTER TABLE units
        ADD CONSTRAINT chk_units_decimal_places CHECK (decimal_places >= 0 AND decimal_places <= 10);
    END IF;
END $$;

