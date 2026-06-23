package services

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"cim-backend/internal/repository"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
)

// newOneActivePendingService wires a real inventoryService over the sqlmock-backed
// gorm handle using the actual submission + snapshot repositories and the single
// BaseRepository, so the one-active-pending guard (epic #38, Part 3 — S5) is
// exercised end-to-end through the real ExistsActivePending repository query and
// the InitiateReconcile WithinTx flow.
func newOneActivePendingService(gormDB *gorm.DB) *inventoryService {
	baseRepo := repository.NewBaseRepository(gormDB)
	return &inventoryService{
		inventoryRepo:           repository.NewInventoryRepository(baseRepo),
		inventoryItemRepo:       repository.NewInventoryItemRepository(baseRepo),
		inventorySubmissionRepo: repository.NewInventorySubmissionRepository(baseRepo),
		snapshotRepo:            repository.NewReconciliationSnapshotRepository(baseRepo),
		baseRepo:                baseRepo,
	}
}

// expectActivePending queues the guard's existence query (ExistsActivePending)
// returning `count`, the live pending-RECONCILE count for the inventory. A
// non-zero count means an active/pending reconcile already exists. The query is
// scoped to submission_type='reconcile' (reconcile-only guard per the human's
// decision), so a pending dispose/transfer is never counted.
func expectActivePending(mock sqlmock.Sqlmock, count int64) {
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "inventory_submissions"`)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(count))
}

func assertConflict(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	var appErr *pkg.AppError
	require.True(t, errors.As(err, &appErr), "expected *pkg.AppError, got %T", err)
	assert.Equal(t, pkg.ErrorCodeActivePendingReconcileConflict, appErr.Code)
}

// TestGuardNoActivePending verifies the guard's two outcomes directly: a live
// pending submission for the inventory yields a Conflict, while no pending
// submission (count 0 — e.g. a prior submission that was rejected->canceled or
// applied->completed) passes. This is the data-correctness core of S5.
func TestGuardNoActivePending(t *testing.T) {
	t.Run("blocks when a pending submission exists", func(t *testing.T) {
		gormDB, mock := newInventoryServiceTestDB(t)
		svc := newOneActivePendingService(gormDB)

		expectActivePending(mock, 1)

		err := svc.guardNoActivePending(context.Background(), 7)
		assertConflict(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("allows when no pending submission exists", func(t *testing.T) {
		gormDB, mock := newInventoryServiceTestDB(t)
		svc := newOneActivePendingService(gormDB)

		// A prior rejected (canceled) / applied (completed) / failed submission has
		// processing_status != 'pending', so the predicate counts zero.
		expectActivePending(mock, 0)

		err := svc.guardNoActivePending(context.Background(), 7)
		require.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// TestInitiateReconcile_BlockedByActivePending verifies the service-level S5
// pre-check on the initiate path: a second initiate while a pending submission
// already exists for the inventory is rejected with Conflict, inside the
// transaction, before the placeholder submission is inserted. The guard runs as
// the first statement inside WithinTx, so a non-zero count must roll back with no
// INSERT.
func TestInitiateReconcile_BlockedByActivePending(t *testing.T) {
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newOneActivePendingService(gormDB)
	ctx := initiateCtx()

	const inventoryID = uint(7)

	expectInventoryExists(mock, inventoryID)
	mock.ExpectBegin()
	// Guard query inside the tx returns a live pending submission -> conflict.
	expectActivePending(mock, 1)
	// No submission INSERT and no snapshot INSERT must follow; the tx rolls back.
	mock.ExpectRollback()

	submission, err := svc.InitiateReconcile(ctx, dto.InitiateReconcileRequest{InventoryID: inventoryID})
	assert.Nil(t, submission)
	assertConflict(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestInitiateReconcile_NotBlockedByTerminalSubmission verifies the carve-out: a
// prior rejected/applied submission (processing_status != 'pending') leaves the
// guard's count at zero, so a new initiate proceeds to create the placeholder and
// capture snapshots normally.
func TestInitiateReconcile_NotBlockedByTerminalSubmission(t *testing.T) {
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newOneActivePendingService(gormDB)
	ctx := initiateCtx()

	const inventoryID = uint(7)

	expectInventoryExists(mock, inventoryID)
	mock.ExpectBegin()
	// No live pending submission (the prior one is canceled/completed) -> guard passes.
	expectActivePending(mock, 0)
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "inventory_submissions"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(101))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO reconciliation_snapshots`)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()

	submission, err := svc.InitiateReconcile(ctx, dto.InitiateReconcileRequest{InventoryID: inventoryID})
	require.NoError(t, err)
	require.NotNil(t, submission)
	assert.Equal(t, uint(101), submission.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestCreateReconcileSubmission_BlockedByActivePending verifies the legacy
// reconcile create path is also guarded: with a pending submission present it
// returns Conflict before loading active items or inserting.
func TestCreateReconcileSubmission_BlockedByActivePending(t *testing.T) {
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newOneActivePendingService(gormDB)

	expectActivePending(mock, 1)
	// No active-items load, no INSERT: the guard is the first thing the path does.

	submission, err := svc.CreateReconcileSubmission(context.Background(), dto.ReconcileInventoryRequest{
		InventoryID: 7,
		Items:       []dto.QuantityItem{{InventoryItemID: 11}},
	})
	assert.Nil(t, submission)
	assertConflict(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// indexViolationErr mimics the error GORM surfaces when the partial unique index
// uq_inventory_submissions_one_active_pending rejects a concurrent duplicate
// reconcile INSERT (the race the service pre-check cannot catch). The translation
// keys off the index name in the error text, so the message must contain it.
func indexViolationErr() error {
	return errors.New(`ERROR: duplicate key value violates unique constraint "uq_inventory_submissions_one_active_pending" (SQLSTATE 23505)`)
}

// TestInitiateReconcile_RaceLoserMappedToConflict verifies the race backstop on
// the initiate path: a second initiate that passes the in-tx guard (count 0) but
// then loses the INSERT race against the partial unique index must surface as a
// clean ErrorCodeConflict (409), not a raw 500. This is the concurrency case the
// check-then-insert pre-check cannot prevent.
func TestInitiateReconcile_RaceLoserMappedToConflict(t *testing.T) {
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newOneActivePendingService(gormDB)
	ctx := initiateCtx()

	const inventoryID = uint(7)

	expectInventoryExists(mock, inventoryID)
	mock.ExpectBegin()
	// Guard passes (the racing peer has not committed yet) ...
	expectActivePending(mock, 0)
	// ... but the placeholder INSERT trips the partial unique index.
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "inventory_submissions"`)).
		WillReturnError(indexViolationErr())
	mock.ExpectRollback()

	submission, err := svc.InitiateReconcile(ctx, dto.InitiateReconcileRequest{InventoryID: inventoryID})
	assert.Nil(t, submission)
	assertConflict(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestCreateDisposeSubmission_NotGuardedByActivePending verifies the dispose
// create path is NOT subject to the one-active-pending guard (epic #38, Part 3 is
// reconcile-only per the human's decision). The proof is structural: the dispose
// path goes straight to loading active inventory items WITHOUT first issuing the
// guard's `SELECT count(*) FROM "inventory_submissions"` existence query. We let
// the active-items load come back empty so the path short-circuits early; the
// critical assertion is that NO ExistsActivePending count query was queued —
// sqlmock would fail on an unexpected query if dispose still called the guard.
func TestCreateDisposeSubmission_NotGuardedByActivePending(t *testing.T) {
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newOneActivePendingService(gormDB)

	// First DB statement on the dispose path must be the active-items load (run
	// inside GetActiveInventoryItems' Transaction), NOT a guard count query.
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "inventory_items"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectRollback()

	submission, err := svc.CreateDisposeSubmission(context.Background(), dto.DisposeInventoryRequest{
		InventoryID: 7,
		Items:       []dto.QuantityItem{{InventoryItemID: 11}},
	})
	assert.Nil(t, submission)
	// Early NotFound from the empty active-items load — not a Conflict from any guard.
	require.Error(t, err)
	var appErr *pkg.AppError
	require.True(t, errors.As(err, &appErr), "expected *pkg.AppError, got %T", err)
	assert.Equal(t, pkg.ErrorCodeNotFound, appErr.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestCreateTransferSubmission_NotGuardedByActivePending verifies the transfer
// create path is likewise NOT guarded (reconcile-only scope). Transfer first loads
// the source inventory; with it missing the path errors out there — and crucially
// the FIRST DB statement is that source-inventory load, never the guard's
// ExistsActivePending count query (sqlmock would fail on an unexpected query if it
// were). The error must not be a Conflict (which a guard hit would produce).
func TestCreateTransferSubmission_NotGuardedByActivePending(t *testing.T) {
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newOneActivePendingService(gormDB)

	// First statement must be the source-inventory load, NOT a guard count query.
	// Empty rows -> First yields ErrRecordNotFound -> path returns an error here.
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "inventories"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	submission, err := svc.CreateTransferSubmission(context.Background(), dto.TransferInventoryRequest{
		SourceInventoryID:      7,
		DestinationInventoryID: 8,
		Items:                  []dto.QuantityItem{{InventoryItemID: 11}},
	})
	assert.Nil(t, submission)
	require.Error(t, err)
	// Whatever the failure mode, it must NOT be the guard's Conflict.
	var appErr *pkg.AppError
	if errors.As(err, &appErr) {
		assert.NotEqual(t, pkg.ErrorCodeConflict, appErr.Code)
	}
	assert.NoError(t, mock.ExpectationsWereMet())
}
