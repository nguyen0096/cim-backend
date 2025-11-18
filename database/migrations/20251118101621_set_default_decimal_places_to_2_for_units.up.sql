-- Update decimal_places to 2 for all existing units
UPDATE units
SET decimal_places = 2
WHERE decimal_places IS NULL OR decimal_places == 0;

-- Change the default value for decimal_places from 0 to 2
ALTER TABLE units
ALTER COLUMN decimal_places SET DEFAULT 2;
