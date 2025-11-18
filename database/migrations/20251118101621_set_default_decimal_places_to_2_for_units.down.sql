-- Revert default value for decimal_places back to 0
ALTER TABLE units
ALTER COLUMN decimal_places SET DEFAULT 0;

-- Note: We don't revert the data changes (decimal_places values) as we don't know
-- what the original values were before this migration
