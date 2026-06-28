-- Add the ROW-level count-session label to reconciliation_request_items (issue #73).
--
-- This is a real, queryable column (NOT JSONB): the label is row identity — it
-- distinguishes a staff user's separate count sessions for one reconciliation so
-- the staff user and the admin/accountant on review can tell rows apart, and so
-- the new List-rows endpoint can return it directly.
--
-- NOT NULL DEFAULT '': blank ("no label") is already a valid state per the
-- row-label rule, so existing rows (added before this migration) backfill to ''
-- rather than NULL. This keeps the model's non-nullable `Label string` safe to scan
-- on the changed read paths (ListBySubmission / GetByID) for pre-deploy rows — a
-- NULL would otherwise fail the scan. On PG11+ a non-volatile (constant) default is
-- metadata-only (no table rewrite / no long lock). The app-side rule (label
-- required once the user has a 2nd live row; distinct per (submission, created_by))
-- is still enforced in the service under the parent FOR UPDATE lock, not by a DB
-- constraint (per-user distinctness tolerating one blank is not expressible as a
-- plain UNIQUE).
ALTER TABLE reconciliation_request_items
    ADD COLUMN IF NOT EXISTS label VARCHAR(255) NOT NULL DEFAULT '';
