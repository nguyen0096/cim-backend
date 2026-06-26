-- Collapse the reconciliation_request_items status enum (epic #38, Part 6
-- redesign — locked decisions Q1 + Q2).
--
-- The per-row review/apply states (ready / approved / applied) are removed: the
-- staff child row now has a single editable state, `in_progress`. Immutability
-- and the apply lifecycle move to the SUBMISSION level (inventory_submissions
-- reconcile_status: open -> closed -> processing -> processed), so per-row gates
-- are no longer needed.
--
-- The original Part-1 CHECK allowed ('in_progress','ready','approved','applied').
-- This migration narrows it to ('in_progress') only. Existing rows on the merged
-- branch are all in_progress/ready (no approved/applied ever shipped to prod —
-- the approve gate was never merged), so any leftover non-in_progress rows are
-- normalized down to in_progress before the constraint is re-applied; this is a
-- pure relabel (the row's counts are unchanged) and keeps the down migration
-- able to restore the permissive constraint without data loss.
UPDATE reconciliation_request_items
    SET status = 'in_progress'
    WHERE status <> 'in_progress';

ALTER TABLE reconciliation_request_items
    DROP CONSTRAINT IF EXISTS chk_reconciliation_request_items_status;

ALTER TABLE reconciliation_request_items
    ADD CONSTRAINT chk_reconciliation_request_items_status
        CHECK (status IN ('in_progress'));
