-- One active/pending RECONCILE submission per inventory (epic #38, Part 3 — S5):
-- race-safe backstop for the service pre-check. Two concurrent pending reconciles
-- could double-apply against the same baseline and corrupt stock.
--
-- Reconcile-only (dispose/transfer are ungated); "active" = processing_status =
-- 'pending' (terminal outcomes and soft-deletes leave the predicate).
--
-- CONCURRENTLY (so the build doesn't lock writes on the populated table) cannot
-- run in a transaction and golang-migrate runs each file as one Exec, so this file
-- holds exactly ONE statement. No IF NOT EXISTS: a failed build leaves an INVALID
-- index and IF NOT EXISTS would let a rerun silently no-op over it with no
-- enforcement; instead a rerun fails loudly until the operator drops the leftover
-- (see down.sql). Pre-existing duplicate live pending reconciles must be resolved
-- before the build can validate.
CREATE UNIQUE INDEX CONCURRENTLY uq_inventory_submissions_one_active_pending
    ON inventory_submissions (inventory_id)
    WHERE deleted_at IS NULL AND processing_status = 'pending' AND submission_type = 'reconcile';
