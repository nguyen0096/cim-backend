-- Uppercase all existing unit names for consistency
UPDATE units SET name = UPPER(name) WHERE name != UPPER(name);

