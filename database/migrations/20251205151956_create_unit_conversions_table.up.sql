-- Create unit_conversions table
-- This table stores conversion factors between different units

CREATE TABLE IF NOT EXISTS unit_conversions (
    id SERIAL PRIMARY KEY,
    from_unit_id INTEGER NOT NULL,
    to_unit_id INTEGER NOT NULL,
    conversion_factor NUMERIC(20,10) NOT NULL,
    created_by VARCHAR(255) NOT NULL DEFAULT 'system@cim.local',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_by VARCHAR(255) NOT NULL DEFAULT 'system@cim.local',
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Add foreign key constraints
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'fk_unit_conversions_from_unit'
    ) THEN
        ALTER TABLE unit_conversions
            ADD CONSTRAINT fk_unit_conversions_from_unit
            FOREIGN KEY (from_unit_id) REFERENCES units(id);
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'fk_unit_conversions_to_unit'
    ) THEN
        ALTER TABLE unit_conversions
            ADD CONSTRAINT fk_unit_conversions_to_unit
            FOREIGN KEY (to_unit_id) REFERENCES units(id);
    END IF;
END $$;

-- Create indexes for foreign keys and soft delete
CREATE INDEX IF NOT EXISTS idx_unit_conversions_from_unit ON unit_conversions(from_unit_id);
CREATE INDEX IF NOT EXISTS idx_unit_conversions_to_unit ON unit_conversions(to_unit_id);
CREATE INDEX IF NOT EXISTS idx_unit_conversions_deleted_at ON unit_conversions(deleted_at);

-- Create unique constraint to prevent duplicate conversion entries
-- Includes deleted_at to allow soft-deleted entries to be re-created
CREATE UNIQUE INDEX IF NOT EXISTS idx_unit_conversions_from_to_unique
    ON unit_conversions(from_unit_id, to_unit_id, deleted_at);
