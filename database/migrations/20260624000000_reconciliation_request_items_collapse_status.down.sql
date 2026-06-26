-- Restore the original permissive Part-1 status CHECK (in_progress / ready /
-- approved / applied). Down does not (and cannot) recreate the relabeled row
-- statuses — the up migration's normalization is lossless on the data (counts
-- unchanged), only the status label was collapsed.
ALTER TABLE reconciliation_request_items
    DROP CONSTRAINT IF EXISTS chk_reconciliation_request_items_status;

ALTER TABLE reconciliation_request_items
    ADD CONSTRAINT chk_reconciliation_request_items_status
        CHECK (status IN ('in_progress', 'ready', 'approved', 'applied'));
