# Reconcile resolver V2 (#46) — runbook

One-time data-fix that corrects the *earlier* reconcile count to match the
later one (accounting for in-window purchases/disposes), synthesizes backdated
FIFO sell/disposal transactions, and recreates the corrected submissions.

**Rule (per item, per adjacent reconcile pair sub1→sub2):**
keep if `sub2_qty <= sub1_qty + range_purchases − range_disposes`; otherwise
correct `sub1_qty := sub2_qty − range_purchases + range_disposes`. Corrections
propagate backward (latest→earliest). Net shrinkage is dated at the **earlier**
sub. Disposes are applied as real backdated Disposal txns at their own dates and
also counted in `− range_disposes` (not double-counted).

## Order of operations (do not skip)

1. **Apply the index migration PR first.** A separate PR adds indexes on
   `inventory_submissions.inventory_id` and
   `inventory_transactions.inventory_item_id`. Merge and run it on prod before
   this tool so discovery/join queries do not seq-scan.

2. **Back up the database.** Affected tables: `inventory_submissions`,
   `inventory_transactions`, `inventory_items`. Take a verified backup /
   snapshot before any `--apply`.

3. **Run a PREVIEW** (read-only, persists nothing). Use explicit IDs and the
   prod DSN locally:

   ```
   go run ./cmd/util reconcile resolve \
     --db-url "postgres://USER:PASS@PROD_HOST:5432/DB?sslmode=require" \
     --inventory 1 \
     --submissions 1,2,4,6 \
     --dispose-submissions 3,5 \
     --out /tmp/reconcile_46_preview.json
   ```

   Preview also lists the inventory's pending dispose submissions so you can
   pick the `--dispose-submissions` ids. Nothing is written.

4. **Review the output.** Inspect `/tmp/reconcile_46_preview.json` and the
   sibling `/tmp/reconcile_46_preview.xlsx`. Confirm per item:
   - `final_stock` equals the last reconcile's corrected count;
   - corrections look right (`orig`→`new` for both `quantity` and `prev`);
   - synthetic sells are dated at the earlier sub; disposals at their own dates;
   - no FATAL errors (e.g. first-sub corrected stock > start stock).

5. **APPLY** in the approved window (single bounded transaction). For a
   non-localhost `--db-url` you MUST pass `--prod-confirm`:

   ```
   go run ./cmd/util reconcile resolve \
     --db-url "postgres://USER:PASS@PROD_HOST:5432/DB?sslmode=require" \
     --inventory 1 \
     --submissions 1,2,4,6 \
     --dispose-submissions 3,5 \
     --out /tmp/reconcile_46_apply.json \
     --apply --prod-confirm
   ```

   Apply re-checks that all target submissions are still pending, optimistic-
   locks the in-scope items, inserts the backdated txns, column-scopes the
   source `consumed_quantity` updates (never touching `created_by`/`created_at`),
   soft-deletes each corrected submission and recreates a clone (same
   `created_at`/`created_by`, `ID=0`, corrected payload), and marks the rest
   approved+completed. Any error aborts the whole transaction (no partial
   writes).

## Safety notes

- Preview == apply: both run the identical compute path.
- This one-off assumes an unconsumed purchase ledger (`consumed_quantity = 0`);
  it aborts if that does not hold.
- Drive by explicit submission/item IDs only. Do not run blind inventory scans.
