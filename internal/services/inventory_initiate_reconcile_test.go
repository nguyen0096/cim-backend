package services

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
)

// newInventoryServiceTestDB builds a *gorm.DB backed by sqlmock, mirroring the
// pattern used in selling_price_service_test.go.
func newInventoryServiceTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: db}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Error),
	})
	require.NoError(t, err)
	return gormDB, mock
}

// newInitiateReconcileService wires a real inventoryService over the sqlmock-backed
// gorm handle using the actual repositories + the single BaseRepository, so the
// test exercises the repository-layer transaction (the service no longer holds a
// tx-capable db handle for the reconcile flow). All repos share the one baseRepo.
func newInitiateReconcileService(gormDB *gorm.DB) *inventoryService {
	baseRepo := repository.NewBaseRepository(gormDB)
	return &inventoryService{
		inventoryRepo:           repository.NewInventoryRepository(baseRepo),
		inventoryItemRepo:       repository.NewInventoryItemRepository(baseRepo),
		inventorySubmissionRepo: repository.NewInventorySubmissionRepository(baseRepo),
		snapshotRepo:            repository.NewReconciliationSnapshotRepository(baseRepo),
		baseRepo:                baseRepo,
	}
}

// expectInventoryExists queues the single lightweight query InitiateReconcile's
// up-front existence check (inventoryRepo.ExistsByID) issues for an existing
// inventory: a SELECT 1 FROM "inventories" with no Items/Product/Unit preload.
// Returning one row makes the bool scan true. Call before the transaction
// expectations.
func expectInventoryExists(mock sqlmock.Sqlmock, inventoryID uint) {
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT 1 FROM "inventories"`)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(true))
}

// expectAdvisoryLock asserts the pg_advisory_xact_lock(inventory_id) that
// InitiateReconcile now takes (epic #38, Part 6 — serializes snapshot capture
// with consuming applies) right after the parent INSERT and before the snapshot
// INSERT.
func expectAdvisoryLock(mock sqlmock.Sqlmock) {
	mock.ExpectExec(regexp.QuoteMeta(`SELECT pg_advisory_xact_lock(`)).
		WillReturnResult(sqlmock.NewResult(0, 0))
}

// initiateCtx returns a context carrying both a user email (required by the
// models.Base BeforeCreate hook) and the initiate_reconciliation permission
// (required by the in-service RBAC guard).
func initiateCtx() context.Context {
	ctx := pkg.WithUserEmail(context.Background(), "admin@cim.local")
	perms := map[pkg.UserPermission]struct{}{
		{Resource: pkg.RBACResourceInventorySubmissions, Action: pkg.RBACActionInitiateReconciliation}: {},
	}
	return context.WithValue(ctx, pkg.AuthContextKeyUserPermissions, perms)
}

// TestBuildReconciliationSnapshots_RawQuery is the core data-correctness
// assertion for the snapshot capture: the baseline is built by a single
// set-based INSERT ... SELECT that copies each active item's live `quantity` into
// `prev_quantity`, scoped to the inventory's active, non-deleted items and FK'd
// to the submission. We assert the SQL shape (so prev_quantity can only ever be
// the item's current quantity) and that it is parameterised with the submission
// id, inventory id and the active status — no app-layer item materialisation.
func TestBuildReconciliationSnapshots_RawQuery(t *testing.T) {
	gormDB, mock := newInventoryServiceTestDB(t)
	repo := repository.NewReconciliationSnapshotRepository(repository.NewBaseRepository(gormDB))

	const submissionID = uint(99)
	const inventoryID = uint(7)

	// The query must SELECT quantity INTO prev_quantity for active, non-deleted
	// items of the inventory, parameterised (no value interpolation).
	// The postgres driver renders the parameterised query with $N placeholders.
	// Asserting the SELECT list (prev_quantity comes from `quantity`) and the
	// active/non-deleted scope locks the data-correctness shape. created_at/updated_at
	// are stamped with clock_timestamp() (NOT NOW()/transaction_timestamp()), so the
	// capture time is the real post-lock statement instant — the correct drift
	// window-start for Start-Processing (epic #38, Part 6; Codex P2).
	const userEmail = "admin@cim.local"
	mock.ExpectExec(regexp.QuoteMeta(
		`INSERT INTO reconciliation_snapshots
	(submission_id, inventory_item_id, prev_quantity, created_by, updated_by, created_at, updated_at)
SELECT $1, id, quantity, $2, $3, clock_timestamp(), clock_timestamp()
FROM inventory_items
WHERE inventory_id = $4
  AND status = $5
  AND deleted_at IS NULL`)).
		WithArgs(submissionID, userEmail, userEmail, inventoryID, string(models.InventoryItemStatusActive)).
		WillReturnResult(sqlmock.NewResult(0, 3))

	ctx := pkg.WithUserEmail(context.Background(), userEmail)
	count, err := repo.BuildReconciliationSnapshots(ctx, submissionID, inventoryID)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count, "rows affected = number of active items snapshotted")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestInitiateReconcile_RBACEnforced verifies the in-service permission guard
// rejects callers lacking initiate_reconciliation before any DB work.
func TestInitiateReconcile_RBACEnforced(t *testing.T) {
	svc := &inventoryService{} // no db; guard short-circuits before touching it

	ctx := pkg.WithUserEmail(context.Background(), "staff@cim.local")
	ctx = context.WithValue(ctx, pkg.AuthContextKeyUserPermissions, map[pkg.UserPermission]struct{}{
		{Resource: pkg.RBACResourceInventorySubmissions, Action: "create"}: {},
	})

	resp, err := svc.InitiateReconcile(ctx, dto.InitiateReconcileRequest{InventoryID: 7})
	require.Error(t, err)
	assert.Nil(t, resp)

	var appErr *pkg.AppError
	require.True(t, errors.As(err, &appErr), "expected *pkg.AppError, got %T", err)
	assert.Equal(t, pkg.ErrorCodeForbidden, appErr.Code)
}

// TestInitiateReconcile_MissingInventory verifies that initiating against an
// inventory id that does not exist returns a NotFound (404) from the up-front
// lightweight existence check (SELECT 1 returns no rows -> not found), before any
// transaction is opened — so the caller never gets a raw FK-violation 500 and no
// placeholder submission is attempted.
func TestInitiateReconcile_MissingInventory(t *testing.T) {
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newInitiateReconcileService(gormDB)
	ctx := initiateCtx()

	// inventoryRepo.ExistsByID -> no rows (inventory absent); no Begin/insert must follow.
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT 1 FROM "inventories"`)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}))

	submission, err := svc.InitiateReconcile(ctx, dto.InitiateReconcileRequest{InventoryID: 7})
	require.Error(t, err)
	assert.Nil(t, submission)

	var appErr *pkg.AppError
	require.True(t, errors.As(err, &appErr), "expected *pkg.AppError, got %T", err)
	assert.Equal(t, pkg.ErrorCodeNotFound, appErr.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestInitiateReconcile_CapturesSnapshotsAtomically verifies the happy path runs
// create parent -> single set-based snapshot INSERT ... SELECT inside one
// committed transaction, and returns the created submission. The baseline is
// captured by one raw query (no app-layer item load), so the only statements are
// the submission insert and the snapshot INSERT ... SELECT.
func TestInitiateReconcile_CapturesSnapshotsAtomically(t *testing.T) {
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newInitiateReconcileService(gormDB)
	ctx := initiateCtx()

	const inventoryID = uint(7)

	// Up-front existence check (returns a clean 404 for a missing inventory rather
	// than a FK-violation 500), then the transaction.
	expectInventoryExists(mock, inventoryID)

	mock.ExpectBegin()

	// 0) One-active-pending guard (epic #38, Part 3 — S5) runs first inside the tx;
	// no live pending submission for this inventory -> it passes.
	expectActivePending(mock, 0)

	// 1) Create the parent placeholder submission; return its generated id.
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "inventory_submissions"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(99))

	// 1b) Advisory lock serializes the snapshot capture with consuming applies.
	expectAdvisoryLock(mock)

	// 2) Capture the baseline with one set-based INSERT ... SELECT; two active
	// items -> two rows affected.
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO reconciliation_snapshots`)).
		WillReturnResult(sqlmock.NewResult(0, 2))

	mock.ExpectCommit()

	submission, err := svc.InitiateReconcile(ctx, dto.InitiateReconcileRequest{InventoryID: inventoryID})
	require.NoError(t, err)
	require.NotNil(t, submission)
	assert.Equal(t, uint(99), submission.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestInitiateReconcile_RollsBackOnSnapshotFailure verifies tx atomicity: if the
// snapshot insert fails, the transaction rolls back (no commit) and the call
// errors — the placeholder submission must not survive.
func TestInitiateReconcile_RollsBackOnSnapshotFailure(t *testing.T) {
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newInitiateReconcileService(gormDB)
	ctx := initiateCtx()

	const inventoryID = uint(7)

	expectInventoryExists(mock, inventoryID)

	mock.ExpectBegin()
	expectActivePending(mock, 0)
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "inventory_submissions"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(99))
	expectAdvisoryLock(mock)
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO reconciliation_snapshots`)).
		WillReturnError(errors.New("insert snapshots boom"))
	mock.ExpectRollback()

	submission, err := svc.InitiateReconcile(ctx, dto.InitiateReconcileRequest{InventoryID: inventoryID})
	require.Error(t, err)
	assert.Nil(t, submission)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestInitiateReconcile_NoActiveItems verifies that an inventory with no active
// items (the snapshot INSERT ... SELECT affects zero rows) yields a NotFound
// error and rolls back without leaving the placeholder submission behind.
func TestInitiateReconcile_NoActiveItems(t *testing.T) {
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newInitiateReconcileService(gormDB)
	ctx := initiateCtx()

	expectInventoryExists(mock, 7)

	mock.ExpectBegin()
	expectActivePending(mock, 0)
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "inventory_submissions"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(99))
	expectAdvisoryLock(mock)
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO reconciliation_snapshots`)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	submission, err := svc.InitiateReconcile(ctx, dto.InitiateReconcileRequest{InventoryID: 7})
	require.Error(t, err)
	assert.Nil(t, submission)

	var appErr *pkg.AppError
	require.True(t, errors.As(err, &appErr), "expected *pkg.AppError, got %T", err)
	assert.Equal(t, pkg.ErrorCodeNotFound, appErr.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}
