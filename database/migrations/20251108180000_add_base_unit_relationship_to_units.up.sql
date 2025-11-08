ALTER TABLE units
    ADD COLUMN IF NOT EXISTS base_unit_id INTEGER;

ALTER TABLE units
    ADD CONSTRAINT fk_units_base_unit
    FOREIGN KEY (base_unit_id) REFERENCES units(id);

CREATE INDEX IF NOT EXISTS idx_units_base_unit ON units(base_unit_id);

ALTER TABLE units
    DROP COLUMN IF EXISTS is_base;
