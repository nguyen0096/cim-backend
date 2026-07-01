package repository

import (
	"context"
	"testing"

	"cim-backend/internal/models"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// awaitingParams is the default page used by the awaiting-processing queue tests.
func awaitingParams() models.ListParams {
	return models.ListParams{Page: 1, Limit: 20, Sort: "updated_at", Order: "desc"}
}

// TestListActiveReconciliations_Predicate verifies the cross-inventory
// awaiting-processing reconcile queue query (issue #88) emits the locked predicate
// `submission_type='reconcile' AND processing_status='pending' AND reconcile_status
// IN (...)` — and, crucially, is NOT scoped to a single inventory_id (it is the
// cross-inventory queue, not ListSubmissions). GORM's soft-delete scope adds
// deleted_at IS NULL, so soft-deleted rows are excluded by construction.
func TestListActiveReconciliations_Predicate(t *testing.T) {
	gormDB, mock := setupTestDB(t)
	repo := NewInventorySubmissionRepository(NewBaseRepository(gormDB))

	// The count query must carry the full predicate and no inventory_id scope.
	mock.ExpectQuery(`SELECT count\(\*\) FROM "inventory_submissions"`).
		WithArgs(models.InventorySubmissionTypeReconcile, models.InventorySubmissionStatusPending,
			string(models.ReconcileLifecycleStatusOpen), string(models.ReconcileLifecycleStatusClosed)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	// The list query appends the pagination LIMIT as a trailing bind arg.
	mock.ExpectQuery(`SELECT \* FROM "inventory_submissions"`).
		WithArgs(models.InventorySubmissionTypeReconcile, models.InventorySubmissionStatusPending,
			string(models.ReconcileLifecycleStatusOpen), string(models.ReconcileLifecycleStatusClosed), 20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "inventory_id", "submission_type", "processing_status", "reconcile_status"}).
			AddRow(1, 10, "reconcile", "pending", "open"))

	mock.ExpectQuery(`SELECT \* FROM "inventories"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(10, "Main"))

	rows, total, err := repo.ListActiveReconciliations(context.Background(), awaitingParams(), nil)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, rows, 1)
	assert.Equal(t, uint(10), rows[0].InventoryID)
	// The WithArgs above pin the predicate to exactly {reconcile, pending, open,
	// closed} with NO inventory_id argument — proof the query is cross-inventory
	// and carries no inventory_id = ? scope (that is ListSubmissions' hard scope).
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestListActiveReconciliations_StatusFilter verifies an explicit
// reconcile_status filter narrows the IN(...) set to the caller-supplied subset
// rather than the default {open,closed}.
func TestListActiveReconciliations_StatusFilter(t *testing.T) {
	gormDB, mock := setupTestDB(t)
	repo := NewInventorySubmissionRepository(NewBaseRepository(gormDB))

	mock.ExpectQuery(`SELECT count\(\*\) FROM "inventory_submissions"`).
		WithArgs(models.InventorySubmissionTypeReconcile, models.InventorySubmissionStatusPending,
			string(models.ReconcileLifecycleStatusClosed)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(`SELECT \* FROM "inventory_submissions"`).
		WithArgs(models.InventorySubmissionTypeReconcile, models.InventorySubmissionStatusPending,
			string(models.ReconcileLifecycleStatusClosed), 20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "inventory_id", "reconcile_status"}).AddRow(2, 11, "closed"))

	mock.ExpectQuery(`SELECT \* FROM "inventories"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(11, "Annex"))

	rows, total, err := repo.ListActiveReconciliations(context.Background(), awaitingParams(),
		[]string{string(models.ReconcileLifecycleStatusClosed)})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, rows, 1)
	assert.Equal(t, models.ReconcileLifecycleStatus("closed"), rows[0].ReconcileStatus)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestListActiveReconciliations_MultiStateFixture drives a multi-row,
// multi-inventory page through a realistic candidate set and asserts the rejected
// edges are EXCLUDED by the predicate. The DB applies the predicate, so this test
// asserts intent at the boundary: only the rows the WHERE admits (the open+pending
// and closed+pending reconciles) are returned in one page with the correct total,
// while the rejected edges — most importantly canceled+open (reviewer-B regression
// guard) — are not present in the result set.
func TestListActiveReconciliations_MultiStateFixture(t *testing.T) {
	gormDB, mock := setupTestDB(t)
	repo := NewInventorySubmissionRepository(NewBaseRepository(gormDB))

	// Candidate fixture (what a real table might hold); the predicate admits only
	// the first two. The rejected edges are listed for documentation — the SQL
	// predicate (asserted via WithArgs) is what enforces their exclusion, so they
	// never appear in the returned rows the mock hands back.
	//
	// ADMITTED:
	//   (1, inv 10) reconcile / pending / open
	//   (2, inv 11) reconcile / pending / closed
	// REJECTED (must NOT be listed):
	//   (3, inv 12) reconcile / CANCELED / open   <- reviewer-B edge (canceled+open)
	//   (4, inv 13) reconcile / completed / processed
	//   (5, inv 14) DISPOSE   / pending / ""       <- wrong type
	//   (6, inv 15) reconcile / pending / open / soft-deleted (deleted_at IS NULL scope)
	mock.ExpectQuery(`SELECT count\(\*\) FROM "inventory_submissions"`).
		WithArgs(models.InventorySubmissionTypeReconcile, models.InventorySubmissionStatusPending,
			string(models.ReconcileLifecycleStatusOpen), string(models.ReconcileLifecycleStatusClosed)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	mock.ExpectQuery(`SELECT \* FROM "inventory_submissions"`).
		WithArgs(models.InventorySubmissionTypeReconcile, models.InventorySubmissionStatusPending,
			string(models.ReconcileLifecycleStatusOpen), string(models.ReconcileLifecycleStatusClosed), 20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "inventory_id", "submission_type", "processing_status", "reconcile_status"}).
			AddRow(1, 10, "reconcile", "pending", "open").
			AddRow(2, 11, "reconcile", "pending", "closed"))

	mock.ExpectQuery(`SELECT \* FROM "inventories"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(10, "Main").AddRow(11, "Annex"))

	rows, total, err := repo.ListActiveReconciliations(context.Background(), awaitingParams(), nil)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total, "page total must count only the admitted rows")
	require.Len(t, rows, 2)

	// Assert the admitted rows surface across both inventories, and none of the
	// rejected edges (canceled+open, processed, dispose, soft-deleted) leaked in.
	gotInventories := map[uint]models.ReconcileLifecycleStatus{}
	for _, r := range rows {
		assert.Equal(t, models.InventorySubmissionTypeReconcile, r.SubmissionType)
		assert.Equal(t, models.InventorySubmissionStatusPending, r.ProcessingStatus)
		assert.Contains(t, []models.ReconcileLifecycleStatus{
			models.ReconcileLifecycleStatusOpen, models.ReconcileLifecycleStatusClosed,
		}, r.ReconcileStatus, "canceled+open and other rejected edges must be excluded")
		gotInventories[r.InventoryID] = r.ReconcileStatus
	}
	assert.Equal(t, models.ReconcileLifecycleStatusOpen, gotInventories[10])
	assert.Equal(t, models.ReconcileLifecycleStatusClosed, gotInventories[11])
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestListActiveReconciliations_BoundedQueryCount is the N+1
// regression guard (issue #88 plan v4 blocker fix). A multi-row (>=3),
// multi-inventory page must issue a CONSTANT, bounded number of query statements
// independent of row count: the count query, the list query, and GORM's single
// batched Preload("Inventory") follow-up = 3 total. There must be NO per-row
// query. sqlmock's ExpectationsWereMet fails if any unexpected (e.g. per-row)
// query is issued, so registering exactly these three and asserting it locks the
// bound.
func TestListActiveReconciliations_BoundedQueryCount(t *testing.T) {
	// awaitingProcessingQueryStatements is the asserted constant: count + list +
	// one batched inventory preload. This is the real bound (3, incl. the GORM
	// preload), NOT 2 — the preload is a real, but bounded and row-count-
	// independent, additional statement.
	const awaitingProcessingQueryStatements = 3

	// Build the mock with a custom QueryMatcher that COUNTS every statement the
	// repo call issues (then delegates to the standard regexp matcher), so we
	// assert the ACTUAL number of queries — not merely that the three we registered
	// were met. A re-introduced per-row synthesis (3 queries/row => 9+ for a 3-row
	// page) would push the count well past the constant and fail the assertion.
	// Count via a custom QueryMatcher. With in-order matching (the default), sqlmock
	// compares each incoming statement against exactly ONE expectation, so the
	// matcher fires once per real query — a faithful statement count. A
	// re-introduced per-row synthesis would issue extra statements (each firing the
	// matcher) and inflate the count past the constant.
	var queryCount int
	countingMatcher := sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		queryCount++
		return sqlmock.QueryMatcherRegexp.Match(expectedSQL, actualSQL)
	})
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(countingMatcher))
	require.NoError(t, err)
	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: db}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Error),
	})
	require.NoError(t, err)
	repo := NewInventorySubmissionRepository(NewBaseRepository(gormDB))

	mock.ExpectQuery(`SELECT count\(\*\) FROM "inventory_submissions"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
	mock.ExpectQuery(`SELECT \* FROM "inventory_submissions"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "inventory_id", "reconcile_status"}).
			AddRow(1, 10, "open").
			AddRow(2, 11, "closed").
			AddRow(3, 12, "open"))
	mock.ExpectQuery(`SELECT \* FROM "inventories"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).
			AddRow(10, "A").AddRow(11, "B").AddRow(12, "C"))

	rows, total, err := repo.ListActiveReconciliations(context.Background(), awaitingParams(), nil)
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	require.Len(t, rows, 3, "multi-inventory page returns all admitted rows")

	// The real bound: exactly count + list + one batched inventory preload = 3,
	// independent of the 3 rows. Asserting the actual constant (not == 2) per the
	// plan implementer note.
	assert.Equal(t, awaitingProcessingQueryStatements, queryCount,
		"awaiting-processing page must issue a constant 3 statements (count+list+preload), no per-row N+1")
	require.NoError(t, mock.ExpectationsWereMet())
}
