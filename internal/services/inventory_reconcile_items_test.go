package services

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
)

// These tests exercise the staff reconciliation child-item lifecycle (epic #38,
// Part 4) over a real inventoryService wired to sqlmock-backed repositories, so
// they assert the actual SQL/transaction shape, the state machine, the
// ownership/status guards, and the counted>snapshot validation — the
// data-correctness bar for this part.

const (
	reconStaffEmail = "staff@cim.local"
	reconOtherEmail = "other-staff@cim.local"
)

// newReconItemServiceReal wires a real inventoryService over the sqlmock-backed
// gorm handle using the actual repositories + the single BaseRepository, so the
// child-item flow exercises the real repository SQL/transaction shapes.
func newReconItemServiceReal(gormDB *gorm.DB) *inventoryService {
	baseRepo := repository.NewBaseRepository(gormDB)
	return &inventoryService{
		inventoryRepo:           repository.NewInventoryRepository(baseRepo),
		inventoryItemRepo:       repository.NewInventoryItemRepository(baseRepo),
		inventorySubmissionRepo: repository.NewInventorySubmissionRepository(baseRepo),
		snapshotRepo:            repository.NewReconciliationSnapshotRepository(baseRepo),
		reconItemRepo:           repository.NewReconciliationRequestItemRepository(baseRepo),
		baseRepo:                baseRepo,
	}
}

// reconCtx returns a context carrying the given owner email (used for both the
// Base hooks and the ownership guard).
func reconCtx(email string) context.Context {
	return pkg.WithUserEmail(context.Background(), email)
}

func gormErrRecordNotFound() error { return gorm.ErrRecordNotFound }

// decPtr returns a pointer to a decimal built from an int, for the pointer-typed
// ReconciliationCountItem.Quantity (nil = omitted, distinct from an explicit 0).
func decPtr(n int64) *decimal.Decimal {
	d := decimal.NewFromInt(n)
	return &d
}

// --- expectation helpers (mirror the repo SQL shapes) ---

// expectParentReconcileLoad queues the parent locking load (GetByIDForUpdate,
// SELECT ... FOR UPDATE) + the snapshot ExistsForSubmission count, for a parent
// that is a valid initiated reconcile. The regexp asserts the FOR UPDATE clause
// is emitted, so every child-write path (which all funnel through
// loadActiveReconcileParent) is verified to lock the parent row inside its tx.
func expectParentReconcileLoad(mock sqlmock.Sqlmock, submissionID uint) {
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "inventory_submissions"`)+`.*FOR UPDATE`).
		WithArgs(submissionID, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "submission_type", "processing_status", "approval_status"}).
			AddRow(submissionID, string(models.InventorySubmissionTypeReconcile), "pending", "pending"))
	// ExistsForSubmission: Count -> 1 snapshot row exists
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "reconciliation_snapshots"`)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
}

// expectSnapshotBaselines queues GetPrevQuantitiesBySubmission returning the
// given item->baseline map.
func expectSnapshotBaselines(mock sqlmock.Sqlmock, baselines map[uint]string) {
	rows := sqlmock.NewRows([]string{"inventory_item_id", "prev_quantity"})
	for id, q := range baselines {
		rows.AddRow(id, q)
	}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT "inventory_item_id","prev_quantity" FROM "reconciliation_snapshots"`)).
		WillReturnRows(rows)
}

// expectSiblingRows queues the ListBySubmission read that the aggregate
// (sum-across-live-child-rows) baseline check performs inside validateCountsAgainstSnapshot.
// Each entry is one live child row: its id and its persisted counted payload
// ({item_id: counted}). The aggregate guard sums these (excluding the row under
// update) and adds the incoming payload before comparing to the snapshot baseline.
func expectSiblingRows(mock sqlmock.Sqlmock, submissionID uint, rows []siblingRow) {
	r := sqlmock.NewRows([]string{"id", "submission_id", "payload"})
	for _, sr := range rows {
		r.AddRow(sr.id, submissionID, []byte(sr.payload))
	}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "reconciliation_request_items"`) + `.*submission_id`).
		WithArgs(submissionID).
		WillReturnRows(r)
}

// siblingRow is a single live child row for expectSiblingRows: its primary key and
// the raw counted-payload JSON in the on-row reconcile shape.
type siblingRow struct {
	id      uint
	payload string
}

// reconLine builds a one-item counted payload string ({"items":[{...}]}) for a
// sibling row.
func reconLine(itemID uint, qty int64) string {
	return `{"items":[{"inventory_item_id":` + itoa(itemID) + `,"quantity":"` + itoa64(qty) + `"}]}`
}

func itoa(n uint) string    { return decimal.NewFromInt(int64(n)).String() }
func itoa64(n int64) string { return decimal.NewFromInt(n).String() }

// expectItemLoad queues the child-item GetByID returning a row with the given
// submission/owner/status.
func expectItemLoad(mock sqlmock.Sqlmock, itemID, submissionID uint, owner, status string) {
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "reconciliation_request_items"`)).
		WithArgs(itemID, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "submission_id", "created_by", "status", "payload"}).
			AddRow(itemID, submissionID, owner, status, []byte(`{"items":[]}`)))
}

func appErrCode(t *testing.T, err error) pkg.ErrorCode {
	t.Helper()
	var appErr *pkg.AppError
	require.True(t, errors.As(err, &appErr), "expected *pkg.AppError, got %T: %v", err, err)
	return appErr.Code
}

// ============================ CREATE ============================

func TestCreateReconciliationItem_HappyPath(t *testing.T) {
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)

	const submissionID = uint(50)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectSnapshotBaselines(mock, map[uint]string{10: "100", 11: "5"})
	expectSiblingRows(mock, submissionID, nil)
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "reconciliation_request_items"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(777))
	mock.ExpectCommit()

	item, err := svc.CreateReconciliationItem(ctx, dto.CreateReconciliationItemRequest{
		SubmissionID: submissionID,
		Items: []dto.ReconciliationCountItem{
			{InventoryItemID: 10, Quantity: decPtr(80)},
			{InventoryItemID: 11, Quantity: decPtr(5)},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, item)
	assert.Equal(t, uint(777), item.ID)
	assert.Equal(t, models.ReconciliationRequestItemStatusInProgress, item.Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateReconciliationItem_CountedExceedsSnapshot_Rejected(t *testing.T) {
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)

	const submissionID = uint(50)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
	expectSiblingRows(mock, submissionID, nil)
	// counted 120 > baseline 100 -> validation error, rollback, no insert.
	mock.ExpectRollback()

	_, err := svc.CreateReconciliationItem(ctx, dto.CreateReconciliationItemRequest{
		SubmissionID: submissionID,
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(120)}},
	})
	require.Error(t, err)
	assert.Equal(t, pkg.ErrorCodeValidation, appErrCode(t, err))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateReconciliationItem_CountedEqualsSnapshot_Allowed(t *testing.T) {
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
	expectSiblingRows(mock, submissionID, nil)
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "reconciliation_request_items"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()

	_, err := svc.CreateReconciliationItem(ctx, dto.CreateReconciliationItemRequest{
		SubmissionID: submissionID,
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(100)}},
	})
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateReconciliationItem_NegativeQuantity_Rejected(t *testing.T) {
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
	expectSiblingRows(mock, submissionID, nil)
	mock.ExpectRollback()

	_, err := svc.CreateReconciliationItem(ctx, dto.CreateReconciliationItemRequest{
		SubmissionID: submissionID,
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(-1)}},
	})
	require.Error(t, err)
	assert.Equal(t, pkg.ErrorCodeValidation, appErrCode(t, err))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateReconciliationItem_NoSnapshotBaseline_Rejected(t *testing.T) {
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	// Baseline map has item 10 only; counted item 99 has no snapshot -> reject.
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
	expectSiblingRows(mock, submissionID, nil)
	mock.ExpectRollback()

	_, err := svc.CreateReconciliationItem(ctx, dto.CreateReconciliationItemRequest{
		SubmissionID: submissionID,
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 99, Quantity: decPtr(1)}},
	})
	require.Error(t, err)
	assert.Equal(t, pkg.ErrorCodeValidation, appErrCode(t, err))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateReconciliationItem_DuplicateLine_Rejected(t *testing.T) {
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
	expectSiblingRows(mock, submissionID, nil)
	mock.ExpectRollback()

	_, err := svc.CreateReconciliationItem(ctx, dto.CreateReconciliationItemRequest{
		SubmissionID: submissionID,
		Items: []dto.ReconciliationCountItem{
			{InventoryItemID: 10, Quantity: decPtr(1)},
			{InventoryItemID: 10, Quantity: decPtr(2)},
		},
	})
	require.Error(t, err)
	assert.Equal(t, pkg.ErrorCodeValidation, appErrCode(t, err))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateReconciliationItem_ParentNotInitiated_Rejected(t *testing.T) {
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)

	mock.ExpectBegin()
	// Parent is a reconcile, but no snapshots exist -> not an initiated reconcile.
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "inventory_submissions"`) + `.*FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "submission_type"}).
			AddRow(submissionID, string(models.InventorySubmissionTypeReconcile)))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "reconciliation_snapshots"`)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectRollback()

	_, err := svc.CreateReconciliationItem(ctx, dto.CreateReconciliationItemRequest{
		SubmissionID: submissionID,
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(1)}},
	})
	require.Error(t, err)
	assert.Equal(t, pkg.ErrorCodeValidation, appErrCode(t, err))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateReconciliationItem_ParentNotFound_Rejected(t *testing.T) {
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "inventory_submissions"`) + `.*FOR UPDATE`).
		WillReturnError(gormErrRecordNotFound())
	mock.ExpectRollback()

	_, err := svc.CreateReconciliationItem(ctx, dto.CreateReconciliationItemRequest{
		SubmissionID: submissionID,
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(1)}},
	})
	require.Error(t, err)
	assert.Equal(t, pkg.ErrorCodeNotFound, appErrCode(t, err))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateReconciliationItem_ParentRejected_Rejected(t *testing.T) {
	// A parent reconcile that was rejected (approval_status=rejected, processing
	// canceled) is terminal: child-item writes must be refused even though the
	// snapshot rows remain. Reachable today via ProcessSubmission's reject path.
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "inventory_submissions"`) + `.*FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "submission_type", "processing_status", "approval_status"}).
			AddRow(submissionID, string(models.InventorySubmissionTypeReconcile), "canceled", "rejected"))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "reconciliation_snapshots"`)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectRollback()

	_, err := svc.CreateReconciliationItem(ctx, dto.CreateReconciliationItemRequest{
		SubmissionID: submissionID,
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(1)}},
	})
	require.Error(t, err)
	assert.Equal(t, pkg.ErrorCodeConflict, appErrCode(t, err))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateReconciliationItem_MissingQuantity_Rejected(t *testing.T) {
	// An omitted quantity (nil pointer) is distinct from an explicit zero count and
	// must be rejected so a malformed payload is never read as full shrinkage.
	// ReconciliationCountItem.Quantity intentionally carries NO validate:"required"
	// tag, so a missing/null quantity flows past the binding validator to the
	// service nil-check and yields the LOCALIZED recon_item_missing_quantity domain
	// error (not the generic validator error). This asserts that exact localized
	// message reaches the caller.
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
	expectSiblingRows(mock, submissionID, nil)
	mock.ExpectRollback()

	_, err := svc.CreateReconciliationItem(ctx, dto.CreateReconciliationItemRequest{
		SubmissionID: submissionID,
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: nil}},
	})
	require.Error(t, err)
	assert.Equal(t, pkg.ErrorCodeValidation, appErrCode(t, err))
	// Assert the specific localized domain error, not a generic validator error:
	// the message must match pkg.ErrReconItemMissingQuantity for item 10.
	var appErr *pkg.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, pkg.ErrReconItemMissingQuantity(ctx, 10).Error(), appErr.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateReconciliationItem_ExplicitZero_Allowed(t *testing.T) {
	// An explicit zero count (counted nothing of an item present in the snapshot) is
	// a legitimate full-shrinkage input and must pass: 0 <= baseline, not negative.
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
	expectSiblingRows(mock, submissionID, nil)
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "reconciliation_request_items"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()

	_, err := svc.CreateReconciliationItem(ctx, dto.CreateReconciliationItemRequest{
		SubmissionID: submissionID,
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(0)}},
	})
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================ AGGREGATE (cross-row) BASELINE ============================

func TestCreateReconciliationItem_AggregateExceedsBaseline_SecondRowRejected(t *testing.T) {
	// Two staff rows of 80 each against a baseline of 100: the FIRST row already
	// counts 80 (a live sibling); a SECOND row of 80 alone passes the per-row check
	// (80 <= 100) but 80 + 80 = 160 > 100, so the aggregate guard must reject it.
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
	// One existing live sibling row already counted 80 of item 10.
	expectSiblingRows(mock, submissionID, []siblingRow{{id: 700, payload: reconLine(10, 80)}})
	mock.ExpectRollback()

	_, err := svc.CreateReconciliationItem(ctx, dto.CreateReconciliationItemRequest{
		SubmissionID: submissionID,
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(80)}},
	})
	require.Error(t, err)
	assert.Equal(t, pkg.ErrorCodeValidation, appErrCode(t, err))
	// Specifically the aggregate error (total 160 across staff submissions), not the
	// per-row exceeds error.
	var appErr *pkg.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t,
		pkg.ErrReconItemAggregateExceedsBaseline(ctx, 10, decimal.NewFromInt(160), decimal.NewFromInt(100)).Error(),
		appErr.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateReconciliationItem_FragmentedCounts_Allowed(t *testing.T) {
	// Fragmented counts 60 + 40 against a baseline of 100: an existing sibling row
	// counted 60; this new row counts 40; 60 + 40 = 100 <= 100, so it is allowed.
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
	expectSiblingRows(mock, submissionID, []siblingRow{{id: 700, payload: reconLine(10, 60)}})
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "reconciliation_request_items"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(701))
	mock.ExpectCommit()

	_, err := svc.CreateReconciliationItem(ctx, dto.CreateReconciliationItemRequest{
		SubmissionID: submissionID,
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(40)}},
	})
	require.NoError(t, err, "60 + 40 == baseline 100 must be allowed")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateReconciliationItem_AggregateMultiItem_OneOverRejected(t *testing.T) {
	// Multi-item payload: item 10 aggregate stays within baseline, item 11 pushes
	// over. The whole create must be rejected (atomic — nothing persists).
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectSnapshotBaselines(mock, map[uint]string{10: "100", 11: "30"})
	// Sibling already counted 50 of item 10 and 25 of item 11.
	expectSiblingRows(mock, submissionID, []siblingRow{
		{id: 700, payload: `{"items":[{"inventory_item_id":10,"quantity":"50"},{"inventory_item_id":11,"quantity":"25"}]}`},
	})
	mock.ExpectRollback()

	_, err := svc.CreateReconciliationItem(ctx, dto.CreateReconciliationItemRequest{
		SubmissionID: submissionID,
		Items: []dto.ReconciliationCountItem{
			{InventoryItemID: 10, Quantity: decPtr(40)}, // 50+40=90 <= 100 OK
			{InventoryItemID: 11, Quantity: decPtr(10)}, // 25+10=35 > 30 REJECT
		},
	})
	require.Error(t, err)
	assert.Equal(t, pkg.ErrorCodeValidation, appErrCode(t, err))
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================ UPDATE / ESCAPE HATCH ============================

// editResetsTo asserts that editing a row in the given start status resets it to
// in_progress (the escape hatch for approved; the re-open for ready).
func runUpdateResetsToInProgress(t *testing.T, startStatus string) {
	t.Helper()
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)
	const itemID = uint(777)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectItemLoad(mock, itemID, submissionID, reconStaffEmail, startStatus)
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
	expectSiblingRows(mock, submissionID, nil)
	// Expect the UPDATE writing payload + status='in_progress'.
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "reconciliation_request_items" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	item, err := svc.UpdateReconciliationItem(ctx, dto.UpdateReconciliationItemRequest{
		SubmissionID: submissionID,
		ItemID:       itemID,
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(50)}},
	})
	require.NoError(t, err)
	assert.Equal(t, models.ReconciliationRequestItemStatusInProgress, item.Status,
		"editing a %s row must reset it to in_progress", startStatus)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateReconciliationItem_ApprovedRow_EscapeHatchResetsToInProgress(t *testing.T) {
	runUpdateResetsToInProgress(t, string(models.ReconciliationRequestItemStatusApproved))
}

func TestUpdateReconciliationItem_ReadyRow_ResetsToInProgress(t *testing.T) {
	runUpdateResetsToInProgress(t, string(models.ReconciliationRequestItemStatusReady))
}

func TestUpdateReconciliationItem_AggregatePushesOverBaseline_Rejected(t *testing.T) {
	// The row under update (777) currently counts 30; a SIBLING row (700) counts 80.
	// Updating 777 from 30 to 50: 80 (sibling) + 50 (new) = 130 > 100. Rejected.
	// (The row's OWN old value of 30 must be EXCLUDED from the sibling sum.)
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)
	const itemID = uint(777)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectItemLoad(mock, itemID, submissionID, reconStaffEmail, string(models.ReconciliationRequestItemStatusInProgress))
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
	// ListBySubmission returns BOTH the updated row (777, old 30 — excluded) and the
	// sibling (700, 80 — counted). The exclusion is by row id inside the service.
	expectSiblingRows(mock, submissionID, []siblingRow{
		{id: 700, payload: reconLine(10, 80)},
		{id: 777, payload: reconLine(10, 30)},
	})
	mock.ExpectRollback()

	_, err := svc.UpdateReconciliationItem(ctx, dto.UpdateReconciliationItemRequest{
		SubmissionID: submissionID,
		ItemID:       itemID,
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(50)}},
	})
	require.Error(t, err)
	assert.Equal(t, pkg.ErrorCodeValidation, appErrCode(t, err))
	var appErr *pkg.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t,
		pkg.ErrReconItemAggregateExceedsBaseline(ctx, 10, decimal.NewFromInt(130), decimal.NewFromInt(100)).Error(),
		appErr.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateReconciliationItem_StaysWithinExcludingOwnOldValue_Allowed(t *testing.T) {
	// Sibling (700) counts 40; the updated row (777) currently counts 80. Updating
	// 777 to 60: if its own OLD 80 were wrongly counted, 40+80+60 would exceed; with
	// the row excluded, 40 (sibling) + 60 (new) = 100 <= 100, so it is allowed. This
	// proves the update path excludes the row being replaced from the sibling sum.
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)
	const itemID = uint(777)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectItemLoad(mock, itemID, submissionID, reconStaffEmail, string(models.ReconciliationRequestItemStatusInProgress))
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
	expectSiblingRows(mock, submissionID, []siblingRow{
		{id: 700, payload: reconLine(10, 40)},
		{id: 777, payload: reconLine(10, 80)},
	})
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "reconciliation_request_items" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	_, err := svc.UpdateReconciliationItem(ctx, dto.UpdateReconciliationItemRequest{
		SubmissionID: submissionID,
		ItemID:       itemID,
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(60)}},
	})
	require.NoError(t, err, "40 (sibling) + 60 (new) == 100 must be allowed; own old 80 excluded")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateReconciliationItem_AppliedRow_Immutable(t *testing.T) {
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)
	const itemID = uint(777)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectItemLoad(mock, itemID, submissionID, reconStaffEmail, string(models.ReconciliationRequestItemStatusApplied))
	mock.ExpectRollback()

	_, err := svc.UpdateReconciliationItem(ctx, dto.UpdateReconciliationItemRequest{
		SubmissionID: submissionID,
		ItemID:       itemID,
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(50)}},
	})
	require.Error(t, err)
	assert.Equal(t, pkg.ErrorCodeConflict, appErrCode(t, err))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateReconciliationItem_NotOwned_Forbidden(t *testing.T) {
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail) // caller is staff@, row owned by other-staff@
	const submissionID = uint(50)
	const itemID = uint(777)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectItemLoad(mock, itemID, submissionID, reconOtherEmail, string(models.ReconciliationRequestItemStatusInProgress))
	mock.ExpectRollback()

	_, err := svc.UpdateReconciliationItem(ctx, dto.UpdateReconciliationItemRequest{
		SubmissionID: submissionID,
		ItemID:       itemID,
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(50)}},
	})
	require.Error(t, err)
	assert.Equal(t, pkg.ErrorCodeForbidden, appErrCode(t, err))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateReconciliationItem_ItemInOtherParent_NotFound(t *testing.T) {
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)
	const itemID = uint(777)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	// Item belongs to submission 999, not the path-scoped 50.
	expectItemLoad(mock, itemID, 999, reconStaffEmail, string(models.ReconciliationRequestItemStatusInProgress))
	mock.ExpectRollback()

	_, err := svc.UpdateReconciliationItem(ctx, dto.UpdateReconciliationItemRequest{
		SubmissionID: submissionID,
		ItemID:       itemID,
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(50)}},
	})
	require.Error(t, err)
	assert.Equal(t, pkg.ErrorCodeNotFound, appErrCode(t, err))
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================ READY / NOT-READY ============================

func TestSetReady_InProgressToReady_OK(t *testing.T) {
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)
	const itemID = uint(777)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectItemLoad(mock, itemID, submissionID, reconStaffEmail, string(models.ReconciliationRequestItemStatusInProgress))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "reconciliation_request_items" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	item, err := svc.SetReconciliationItemReady(ctx, dto.SetReconciliationItemReadyRequest{
		SubmissionID: submissionID, ItemID: itemID, Ready: true,
	})
	require.NoError(t, err)
	assert.Equal(t, models.ReconciliationRequestItemStatusReady, item.Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSetReady_ReadyToInProgress_OK(t *testing.T) {
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)
	const itemID = uint(777)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectItemLoad(mock, itemID, submissionID, reconStaffEmail, string(models.ReconciliationRequestItemStatusReady))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "reconciliation_request_items" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	item, err := svc.SetReconciliationItemReady(ctx, dto.SetReconciliationItemReadyRequest{
		SubmissionID: submissionID, ItemID: itemID, Ready: false,
	})
	require.NoError(t, err)
	assert.Equal(t, models.ReconciliationRequestItemStatusInProgress, item.Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSetReady_ApprovedToReady_IllegalTransition(t *testing.T) {
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)
	const itemID = uint(777)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectItemLoad(mock, itemID, submissionID, reconStaffEmail, string(models.ReconciliationRequestItemStatusApproved))
	mock.ExpectRollback()

	_, err := svc.SetReconciliationItemReady(ctx, dto.SetReconciliationItemReadyRequest{
		SubmissionID: submissionID, ItemID: itemID, Ready: true,
	})
	require.Error(t, err)
	assert.Equal(t, pkg.ErrorCodeConflict, appErrCode(t, err))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSetReady_AppliedRow_Immutable(t *testing.T) {
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)
	const itemID = uint(777)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectItemLoad(mock, itemID, submissionID, reconStaffEmail, string(models.ReconciliationRequestItemStatusApplied))
	mock.ExpectRollback()

	_, err := svc.SetReconciliationItemReady(ctx, dto.SetReconciliationItemReadyRequest{
		SubmissionID: submissionID, ItemID: itemID, Ready: true,
	})
	require.Error(t, err)
	assert.Equal(t, pkg.ErrorCodeConflict, appErrCode(t, err))
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================ DELETE ============================

func runDeleteAllowed(t *testing.T, status string) {
	t.Helper()
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)
	const itemID = uint(777)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectItemLoad(mock, itemID, submissionID, reconStaffEmail, status)
	// SoftDelete: audit Update then soft-delete Delete (sets deleted_at).
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "reconciliation_request_items" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "reconciliation_request_items" SET "deleted_at"`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := svc.DeleteReconciliationItem(ctx, dto.DeleteReconciliationItemRequest{
		SubmissionID: submissionID, ItemID: itemID,
	})
	require.NoError(t, err, "status %s must be deletable", status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteReconciliationItem_InProgress_OK(t *testing.T) {
	runDeleteAllowed(t, string(models.ReconciliationRequestItemStatusInProgress))
}

func TestDeleteReconciliationItem_Ready_OK(t *testing.T) {
	runDeleteAllowed(t, string(models.ReconciliationRequestItemStatusReady))
}

func runDeleteRejected(t *testing.T, status string, wantCode pkg.ErrorCode) {
	t.Helper()
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)
	const itemID = uint(777)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectItemLoad(mock, itemID, submissionID, reconStaffEmail, status)
	mock.ExpectRollback()

	err := svc.DeleteReconciliationItem(ctx, dto.DeleteReconciliationItemRequest{
		SubmissionID: submissionID, ItemID: itemID,
	})
	require.Error(t, err, "status %s must NOT be deletable", status)
	assert.Equal(t, wantCode, appErrCode(t, err))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteReconciliationItem_Approved_Rejected(t *testing.T) {
	// Approved rows cannot be deleted (only the edit-back-to-in_progress escape hatch).
	runDeleteRejected(t, string(models.ReconciliationRequestItemStatusApproved), pkg.ErrorCodeConflict)
}

func TestDeleteReconciliationItem_Applied_Immutable(t *testing.T) {
	// Applied is caught by the immutability guard (also a conflict).
	runDeleteRejected(t, string(models.ReconciliationRequestItemStatusApplied), pkg.ErrorCodeConflict)
}

func TestDeleteReconciliationItem_NotOwned_Forbidden(t *testing.T) {
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)
	const itemID = uint(777)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectItemLoad(mock, itemID, submissionID, reconOtherEmail, string(models.ReconciliationRequestItemStatusInProgress))
	mock.ExpectRollback()

	err := svc.DeleteReconciliationItem(ctx, dto.DeleteReconciliationItemRequest{
		SubmissionID: submissionID, ItemID: itemID,
	})
	require.Error(t, err)
	assert.Equal(t, pkg.ErrorCodeForbidden, appErrCode(t, err))
	assert.NoError(t, mock.ExpectationsWereMet())
}
