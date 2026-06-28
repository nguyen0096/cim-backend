-- Drop the ROW-level count-session label column (issue #73 rollback).
ALTER TABLE reconciliation_request_items
    DROP COLUMN IF EXISTS label;
