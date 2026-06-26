-- Reconciliation submission lifecycle + processed-at marker (epic #38, Part 6
-- redesign — locked decisions Q3 + Q6).
--
-- Q3: `item_submission_closed` is modeled as a STATUS of inventory_submissions
-- (not a separate boolean, not an approval status). reconcile_status carries the
-- reconciliation lifecycle: open -> closed -> processing -> processed. It is set
-- only for initiated reconciles; every other submission (dispose / transfer /
-- legacy reconcile) leaves it unset, so the legacy approval/processing flow is
-- completely unaffected. The CHECK admits NULL or '' (the GORM zero value written
-- for a non-reconcile row's unset string column) or one of the four lifecycle
-- values — so a dispose/transfer Create with no reconcile_status never trips it.
--
-- Q6: processed_at is the precise instant a CONSUMING submission's processing
-- completed. It is the authoritative window bound for the Start-Processing drift
-- re-check (a sibling consuming submission whose processed_at falls inside the
-- reconcile window is drift). NULL until processing completes.
--
-- It MUST be backfilled for pre-existing COMPLETED consuming submissions (see the
-- backfill below). The initiate/snapshot flow (Part 2) is already shipped on main,
-- so a reconcile may be IN FLIGHT across this deploy: it captured its snapshot
-- baseline before the migration, and a dispose/transfer/reconcile that COMPLETED
-- between that capture and this deploy left processed_at NULL (the column did not
-- exist yet). The drift re-check (ListConsumingProcessedSince) excludes NULL
-- processed_at rows, so without a backfill StartProcessing would MISS that
-- already-applied consuming sibling and apply from a stale baseline — a silent
-- data-correctness defect. Backfilling a completion-time proxy makes those
-- pre-deploy completions visible to the re-check (whether they land inside or
-- before a given reconcile's window is then decided correctly by the timestamp).
ALTER TABLE inventory_submissions
    ADD COLUMN IF NOT EXISTS reconcile_status VARCHAR(20);

ALTER TABLE inventory_submissions
    ADD COLUMN IF NOT EXISTS processed_at TIMESTAMP WITH TIME ZONE;

ALTER TABLE inventory_submissions
    DROP CONSTRAINT IF EXISTS chk_inventory_submissions_reconcile_status;

ALTER TABLE inventory_submissions
    ADD CONSTRAINT chk_inventory_submissions_reconcile_status
        CHECK (reconcile_status IS NULL
            OR reconcile_status = ''
            OR reconcile_status IN ('open', 'closed', 'processing', 'processed'));

-- Backfill: stamp processed_at on pre-existing COMPLETED CONSUMING submissions.
--
-- These rows consumed source stock before this deploy but carry processed_at NULL
-- (the column is new). A reconcile in flight across the deploy needs the drift
-- re-check (ListConsumingProcessedSince) to SEE such a sibling that completed after
-- its snapshot capture; with NULL processed_at the re-check would skip it and apply
-- a stale baseline. We use COALESCE(updated_at, created_at) as the truest available
-- completion-time proxy: updated_at is rewritten on the status flip to 'completed'
-- (the apply commit), so for a completed consuming submission it tracks the
-- completion instant; created_at is the conservative fallback if updated_at is null.
-- This populates ALL pre-deploy completed consuming rows; the per-reconcile window
-- bound then decides correctly (timestamp >= snapshot-capture => drift, else not).
--
-- Predicate precision (do NOT touch anything else):
--   * submission_type IN ('dispose','transfer','reconcile') — the CONSUMING types
--                                             the drift re-check inspects; an
--                                             import/other type is never consuming.
--   * processing_status = 'completed'        — only rows whose apply actually ran
--                                             and created consuming transactions;
--                                             pending/failed/canceled created none.
--   * approval_status = 'approved'           — completed implies approved; assert it
--                                             so a stray completed-but-not-approved
--                                             row is not stamped.
--   * processed_at IS NULL                   — idempotent / re-run safe; never
--                                             overwrite a value the new flow stamped.
--   * deleted_at IS NULL                     — skip soft-deleted submissions.
--
-- Ordering vs the `open` backfill below: the two predicates are DISJOINT
-- (processing_status='completed' here vs 'pending' there), so they never touch the
-- same row and the order between them does not matter.
UPDATE inventory_submissions
SET processed_at = COALESCE(updated_at, created_at)
WHERE submission_type IN ('dispose', 'transfer', 'reconcile')
    AND processing_status = 'completed'
    AND approval_status = 'approved'
    AND processed_at IS NULL
    AND deleted_at IS NULL;

-- Backfill: existing IN-FLIGHT INITIATED reconciles must start at `open`.
--
-- The initiate/snapshot flow (Part 2) is already shipped on main, so when this
-- migration runs there may already be reconcile submissions that are still in
-- flight and carry a captured snapshot baseline. Those rows get `reconcile_status`
-- NULL from the column add above, but the new lifecycle guards treat ONLY `open`
-- as staff-editable, and close/start-processing require the source status to be
-- `open`/`closed`. Left NULL, such a reconcile would be stranded: neither editable
-- by staff nor closable/startable by an admin. Set it to `open` — the exact state
-- a fresh initiate writes — so it rejoins the lifecycle seamlessly.
--
-- Predicate precision (do NOT touch anything else):
--   * submission_type = 'reconcile'        — reconcile-only; dispose/transfer never
--                                             carry a reconcile_status.
--   * processing_status = 'pending'         — still in flight; excludes terminal
--                                             rows (completed/failed/canceled), so a
--                                             processed/rejected reconcile stays put.
--   * approval_status = 'pending'           — not yet approved/rejected; an
--                                             already-rejected (canceled) reconcile
--                                             is terminal and must NOT be reopened.
--   * reconcile_status IS NULL OR = ''      — only rows the new flow has not already
--                                             stamped (idempotent / re-run safe).
--   * deleted_at IS NULL                    — skip soft-deleted submissions.
--   * EXISTS live reconciliation_snapshots  — the INITIATED marker. A LEGACY
--                                             reconcile (created via the old
--                                             create-submission path, no snapshot)
--                                             has no lifecycle and is left untouched.
UPDATE inventory_submissions s
SET reconcile_status = 'open'
WHERE s.submission_type = 'reconcile'
    AND s.processing_status = 'pending'
    AND s.approval_status = 'pending'
    AND (s.reconcile_status IS NULL OR s.reconcile_status = '')
    AND s.deleted_at IS NULL
    AND EXISTS (
        SELECT 1
        FROM reconciliation_snapshots rs
        WHERE rs.submission_id = s.id
            AND rs.deleted_at IS NULL
    );
