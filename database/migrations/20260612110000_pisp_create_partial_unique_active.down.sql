-- Drop the partial unique index. CONCURRENTLY so the drop does not block concurrent
-- writes on a populated pisp table; like the build, it cannot run inside a
-- transaction, so this file MUST contain exactly ONE statement.
--
-- The matching down migration for 20260612120000 re-adds the original non-partial
-- UNIQUE constraint and runs BEFORE this drop, so uniqueness on PO items is never
-- left unenforced during rollback.
DROP INDEX CONCURRENTLY IF EXISTS uq_pisp_po_item_id_active;
