package services

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

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

// reconManageCtx returns a context for an admin/accountant: carries the email
// plus the recon_manage permission so guardParentEditable/guardOwnership treat
// the caller as a manager (may edit a closed submission's rows, bypassing
// ownership).
func reconManageCtx(email string) context.Context {
	ctx := pkg.WithUserEmail(context.Background(), email)
	perms := map[pkg.UserPermission]struct{}{
		{Resource: pkg.RBACResourceInventorySubmissions, Action: pkg.RBACActionReconManage}: {},
	}
	return context.WithValue(ctx, pkg.AuthContextKeyUserPermissions, perms)
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
	expectParentReconcileLoadStatus(mock, submissionID, string(models.ReconcileLifecycleStatusOpen))
}

// expectParentReconcileLoadStatus is expectParentReconcileLoad with an explicit
// reconcile_status (open/closed/...) so the staff-immutability guard can be
// exercised.
func expectParentReconcileLoadStatus(mock sqlmock.Sqlmock, submissionID uint, reconcileStatus string) {
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "inventory_submissions"`)+`.*FOR UPDATE`).
		WithArgs(submissionID, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "submission_type", "processing_status", "approval_status", "reconcile_status"}).
			AddRow(submissionID, string(models.InventorySubmissionTypeReconcile), "pending", "pending", reconcileStatus))
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

// expectInventoryItemsByIDs queues the full GetByIDs read that resolveProductName
// performs on a count-baseline rejection branch: the main inventory_items SELECT
// PLUS the Inventory/Product/Unit preload SELECTs (GetByIDs is
// Unscoped().Preload("Inventory").Preload("Product").Preload("Unit")). GORM emits
// these as separate ordered queries through the single sqlmock queue, so all four
// must be queued for ExpectationsWereMet() to pass.
//
// items maps inventory_item_id -> product name. Each item row carries a distinct
// product_id/unit_id/inventory_id (derived from the item id) so the preloads have
// non-zero FKs to resolve.
func expectInventoryItemsByIDs(mock sqlmock.Sqlmock, items map[uint]string) {
	itemRows := sqlmock.NewRows([]string{"id", "inventory_id", "product_id", "unit_id"})
	invRows := sqlmock.NewRows([]string{"id", "name"})
	prodRows := sqlmock.NewRows([]string{"id", "name"})
	unitRows := sqlmock.NewRows([]string{"id", "name"})
	for id, name := range items {
		// Stable, distinct FK ids derived from the item id.
		invID := 1000 + id
		prodID := 2000 + id
		unitID := 3000 + id
		itemRows.AddRow(id, invID, prodID, unitID)
		invRows.AddRow(invID, "Inv")
		prodRows.AddRow(prodID, name)
		unitRows.AddRow(unitID, "Unit")
	}
	// Main query + the three preloads, matched loosely by table so column/clause
	// ordering differences across GORM versions don't break the match.
	mock.ExpectQuery(regexp.QuoteMeta(`FROM "inventory_items"`)).WillReturnRows(itemRows)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM "inventories"`)).WillReturnRows(invRows)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM "products"`)).WillReturnRows(prodRows)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM "units"`)).WillReturnRows(unitRows)
}

// expectInventoryItemNilProduct queues a GetByIDs read where the inventory item
// resolves but its product is soft-deleted: GetByIDs is Unscoped() so the item row
// comes back, but the scoped Preload("Product") finds no row, leaving Product ==
// nil. resolveProductName must return "" here (its nil-guard) rather than panic.
// GORM still issues the products preload (product_id is non-zero) but it returns no
// row. Inventory/Unit preloads resolve normally.
func expectInventoryItemNilProduct(mock sqlmock.Sqlmock, itemID uint) {
	invID := 1000 + itemID
	prodID := 2000 + itemID
	unitID := 3000 + itemID
	mock.ExpectQuery(regexp.QuoteMeta(`FROM "inventory_items"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "inventory_id", "product_id", "unit_id"}).
			AddRow(itemID, invID, prodID, unitID))
	mock.ExpectQuery(regexp.QuoteMeta(`FROM "inventories"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(invID, "Inv"))
	// Soft-deleted product: scoped preload returns NO row -> Product stays nil.
	mock.ExpectQuery(regexp.QuoteMeta(`FROM "products"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))
	mock.ExpectQuery(regexp.QuoteMeta(`FROM "units"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(unitID, "Unit"))
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

// ownedRow is a live child row carrying its owner (created_by) and ROW-level label,
// for exercising the issue-#73 row-label rule (which scopes per (submission, owner)).
type ownedRow struct {
	id      uint
	owner   string
	label   string
	payload string
}

// expectOwnedSiblingRows queues the ListBySubmission read returning rows WITH their
// created_by + label columns, so the row-label rule (validateRowLabel) sees the
// owner's other live rows and their labels.
func expectOwnedSiblingRows(mock sqlmock.Sqlmock, submissionID uint, rows []ownedRow) {
	r := sqlmock.NewRows([]string{"id", "submission_id", "created_by", "label", "payload"})
	for _, sr := range rows {
		r.AddRow(sr.id, submissionID, sr.owner, sr.label, []byte(sr.payload))
	}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "reconciliation_request_items"`) + `.*submission_id`).
		WithArgs(submissionID).
		WillReturnRows(r)
}

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
	expectSiblingRows(mock, submissionID, nil)
	expectSnapshotBaselines(mock, map[uint]string{10: "100", 11: "5"})
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
	assert.Equal(t, string(models.ReconciliationRequestItemStatusInProgress), item.Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateReconciliationItem_CountedExceedsSnapshot_Rejected(t *testing.T) {
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)

	const submissionID = uint(50)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectSiblingRows(mock, submissionID, nil)
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
	// counted 120 > baseline 100 -> validation error, rollback, no insert.
	// resolveProductName resolves item 10 -> "Sản phẩm A" on the rejecting branch.
	expectInventoryItemsByIDs(mock, map[uint]string{10: "Sản phẩm A"})
	mock.ExpectRollback()

	// VI locale so we assert the issue's exact reworded VI string.
	ctx = pkg.WithLanguage(ctx, pkg.LangVI)
	_, err := svc.CreateReconciliationItem(ctx, dto.CreateReconciliationItemRequest{
		SubmissionID: submissionID,
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(120)}},
	})
	require.Error(t, err)
	assert.Equal(t, pkg.ErrorCodeValidation, appErrCode(t, err))
	// Message renders the product NAME and the new wording, never the raw item ID.
	assert.Contains(t, err.Error(), "Sản phẩm A")
	assert.Contains(t, err.Error(), "số lượng ghi nhận tại thời điểm bắt đầu đối soát")
	assert.NotContains(t, err.Error(), "sản phẩm 10")
	assert.NotContains(t, err.Error(), "số liệu nền")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateReconciliationItem_CountedExceedsSnapshot_NilProduct_Graceful(t *testing.T) {
	// The rejecting item's product is soft-deleted, so GetByIDs (Unscoped) returns the
	// item but the scoped Product preload finds no row -> Product == nil.
	// resolveProductName's nil-guard must return "" (no panic), and the message renders
	// an empty name inside guillemets («») with the new wording.
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectSiblingRows(mock, submissionID, nil)
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
	// Soft-deleted product: item resolves, product preload finds nothing.
	expectInventoryItemNilProduct(mock, 10)
	mock.ExpectRollback()

	// VI locale so we assert the reworded VI string + empty-name guillemets.
	ctx = pkg.WithLanguage(ctx, pkg.LangVI)
	_, err := svc.CreateReconciliationItem(ctx, dto.CreateReconciliationItemRequest{
		SubmissionID: submissionID,
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(120)}},
	})
	require.Error(t, err)
	assert.Equal(t, pkg.ErrorCodeValidation, appErrCode(t, err))
	// Empty name renders inside guillemets; new wording present; no raw ID, no old jargon.
	assert.Contains(t, err.Error(), "«»")
	assert.Contains(t, err.Error(), "số lượng ghi nhận tại thời điểm bắt đầu đối soát")
	assert.NotContains(t, err.Error(), "sản phẩm 10")
	assert.NotContains(t, err.Error(), "số liệu nền")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateReconciliationItem_CountedEqualsSnapshot_Allowed(t *testing.T) {
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectSiblingRows(mock, submissionID, nil)
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
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
	expectSiblingRows(mock, submissionID, nil)
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
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
	expectSiblingRows(mock, submissionID, nil)
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
	// resolveProductName resolves item 99 -> "Sản phẩm Z" on the rejecting branch.
	expectInventoryItemsByIDs(mock, map[uint]string{99: "Sản phẩm Z"})
	mock.ExpectRollback()

	// VI locale so we assert the issue's exact reworded VI string.
	ctx = pkg.WithLanguage(ctx, pkg.LangVI)
	_, err := svc.CreateReconciliationItem(ctx, dto.CreateReconciliationItemRequest{
		SubmissionID: submissionID,
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 99, Quantity: decPtr(1)}},
	})
	require.Error(t, err)
	assert.Equal(t, pkg.ErrorCodeValidation, appErrCode(t, err))
	// Message renders the product NAME and the new wording, never the raw item ID.
	assert.Contains(t, err.Error(), "Sản phẩm Z")
	assert.Contains(t, err.Error(), "số lượng ghi nhận tại thời điểm bắt đầu đối soát")
	assert.NotContains(t, err.Error(), "sản phẩm 99")
	assert.NotContains(t, err.Error(), "số liệu nền")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateReconciliationItem_DuplicateLine_Rejected(t *testing.T) {
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectSiblingRows(mock, submissionID, nil)
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
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
	expectSiblingRows(mock, submissionID, nil)
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
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
	expectSiblingRows(mock, submissionID, nil)
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
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

// ============================ LABELS (issue #73) ============================

// reconLineLabeled builds a one-item LABELED counted payload string for a sibling
// row ({"items":[{"inventory_item_id","quantity","label"}]}).
func reconLineLabeled(itemID uint, qty int64, label string) string {
	return `{"items":[{"inventory_item_id":` + itoa(itemID) + `,"quantity":"` + itoa64(qty) + `","label":"` + label + `"}]}`
}

func TestCreateReconciliationItem_SingleBlankLabel_Allowed(t *testing.T) {
	// The first/single count of an item may carry a blank label (not retro-labeled).
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectSiblingRows(mock, submissionID, nil)
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "reconciliation_request_items"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()

	_, err := svc.CreateReconciliationItem(ctx, dto.CreateReconciliationItemRequest{
		SubmissionID: submissionID,
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(30)}},
	})
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateReconciliationItem_LabelPersisted(t *testing.T) {
	// The count label is persisted and surfaces in the response Items (issue #73).
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectSiblingRows(mock, submissionID, nil)
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "reconciliation_request_items"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()

	created, err := svc.CreateReconciliationItem(ctx, dto.CreateReconciliationItemRequest{
		SubmissionID: submissionID,
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(30), Label: "shelf"}},
	})
	require.NoError(t, err)
	require.Len(t, created.Items, 1)
	assert.Equal(t, "shelf", created.Items[0].Label)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestValidateCountLabelDistinctness_OrderInsensitive(t *testing.T) {
	// Codex P2: the per-item count-label rule must be ORDER-INSENSITIVE — a payload
	// and any permutation of it must validate identically. Exercises the pure helper
	// in BOTH orders for each case.
	ctx := reconCtx(reconStaffEmail)
	mk := func(labels ...string) []dto.ReconciliationCountItem {
		items := make([]dto.ReconciliationCountItem, 0, len(labels))
		for _, l := range labels {
			items = append(items, dto.ReconciliationCountItem{InventoryItemID: 10, Quantity: decPtr(1), Label: l})
		}
		return items
	}
	reversed := func(items []dto.ReconciliationCountItem) []dto.ReconciliationCountItem {
		out := make([]dto.ReconciliationCountItem, len(items))
		for i := range items {
			out[i] = items[len(items)-1-i]
		}
		return out
	}

	cases := []struct {
		name      string
		labels    []string
		wantValid bool
	}{
		{"non-blank + blank (the Codex case)", []string{"dock", ""}, true},
		{"two blanks", []string{"", ""}, false},
		{"duplicate non-blank", []string{"dock", "dock"}, false},
		{"two distinct non-blank", []string{"shelf", "dock"}, true},
		{"single blank", []string{""}, true},
	}
	for _, tc := range cases {
		for _, order := range []struct {
			suffix string
			items  []dto.ReconciliationCountItem
		}{
			{"forward", mk(tc.labels...)},
			{"reversed", reversed(mk(tc.labels...))},
		} {
			t.Run(tc.name+"/"+order.suffix, func(t *testing.T) {
				err := validateCountLabelDistinctness(ctx, order.items)
				if tc.wantValid {
					assert.NoError(t, err)
				} else {
					require.Error(t, err)
					assert.Equal(t, pkg.ErrorCodeValidation, appErrCode(t, err))
				}
			})
		}
	}
}

func TestCreateReconciliationItem_BlankThenLabeledSameItem_Allowed(t *testing.T) {
	// Codex P2 end-to-end: the same item with ("dock","") must be accepted in BOTH
	// orders. This is the order ("dock" first, blank second) that the old positional
	// logic wrongly rejected. 30 + 25 = 55 <= 100 baseline.
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectSiblingRows(mock, submissionID, nil)
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "reconciliation_request_items"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()

	_, err := svc.CreateReconciliationItem(ctx, dto.CreateReconciliationItemRequest{
		SubmissionID: submissionID,
		Items: []dto.ReconciliationCountItem{
			{InventoryItemID: 10, Quantity: decPtr(30), Label: "dock"},
			{InventoryItemID: 10, Quantity: decPtr(25), Label: ""},
		},
	})
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateReconciliationItem_TwoDistinctLabelsSameItemInOnePayload_Allowed(t *testing.T) {
	// The SAME item may appear twice in one payload provided the labels are distinct
	// non-empty (issue #73): milk "shelf" 30 + "dock" 25 against a baseline of 100.
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectSiblingRows(mock, submissionID, nil)
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "reconciliation_request_items"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()

	_, err := svc.CreateReconciliationItem(ctx, dto.CreateReconciliationItemRequest{
		SubmissionID: submissionID,
		Items: []dto.ReconciliationCountItem{
			{InventoryItemID: 10, Quantity: decPtr(30), Label: "shelf"},
			{InventoryItemID: 10, Quantity: decPtr(25), Label: "dock"},
		},
	})
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateReconciliationItem_SecondBlankLabelSameItem_Rejected(t *testing.T) {
	// A 2nd blank-label count of an item (one blank already present in this payload)
	// is rejected: the 2nd+ count needs a non-empty label to tell it apart.
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectSiblingRows(mock, submissionID, nil)
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
	mock.ExpectRollback()

	_, err := svc.CreateReconciliationItem(ctx, dto.CreateReconciliationItemRequest{
		SubmissionID: submissionID,
		Items: []dto.ReconciliationCountItem{
			{InventoryItemID: 10, Quantity: decPtr(30)},
			{InventoryItemID: 10, Quantity: decPtr(25)},
		},
	})
	require.Error(t, err)
	var appErr *pkg.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, pkg.ErrReconItemLabelRequiredForDuplicate(ctx, 10).Error(), appErr.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateReconciliationItem_DuplicateLabelSameItem_Rejected(t *testing.T) {
	// Two counts of the same item with the SAME non-empty label are rejected: labels
	// must be distinct per item.
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectSiblingRows(mock, submissionID, nil)
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
	mock.ExpectRollback()

	_, err := svc.CreateReconciliationItem(ctx, dto.CreateReconciliationItemRequest{
		SubmissionID: submissionID,
		Items: []dto.ReconciliationCountItem{
			{InventoryItemID: 10, Quantity: decPtr(30), Label: "shelf"},
			{InventoryItemID: 10, Quantity: decPtr(25), Label: "shelf"},
		},
	})
	require.Error(t, err)
	var appErr *pkg.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, pkg.ErrReconItemLabelConflict(ctx, 10, "shelf").Error(), appErr.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateReconciliationItem_BlankCountLabelWhenSiblingHasBlankCount_Allowed(t *testing.T) {
	// Issue #73 RE-SCOPE: the COUNT-label rule is now PER ROW. A sibling row counting
	// the same item with a blank count label no longer forces this row's count to be
	// labelled — two DIFFERENT rows may each carry a blank count of the same item (the
	// ROW-level label distinguishes the rows). The aggregate quantity still holds:
	// 20 (sibling) + 20 (new) = 40 <= 100, so the create succeeds. (The mock sibling
	// has no created_by, so the row-label rule does not require a row label here.)
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectSiblingRows(mock, submissionID, []siblingRow{{id: 700, payload: reconLine(10, 20)}})
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "reconciliation_request_items"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(701))
	mock.ExpectCommit()

	_, err := svc.CreateReconciliationItem(ctx, dto.CreateReconciliationItemRequest{
		SubmissionID: submissionID,
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(20)}},
	})
	require.NoError(t, err, "per-row count-label scope: a sibling's blank count must not force this row's count to be labelled")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateReconciliationItem_CountLabelReusedAcrossRows_Allowed(t *testing.T) {
	// Issue #73 RE-SCOPE: the previously-rejected cross-row count-label collision is
	// now ALLOWED. A sibling row already counts item 10 under count label "dock"; this
	// new (different) row may reuse "dock" for its own count of item 10 — count-label
	// distinctness is scoped to the row being edited, not the submission.
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectSiblingRows(mock, submissionID, []siblingRow{{id: 700, payload: reconLineLabeled(10, 20, "dock")}})
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "reconciliation_request_items"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(701))
	mock.ExpectCommit()

	_, err := svc.CreateReconciliationItem(ctx, dto.CreateReconciliationItemRequest{
		SubmissionID: submissionID,
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(20), Label: "dock"}},
	})
	require.NoError(t, err, "per-row count-label scope: two different rows may reuse the same count label")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateReconciliationItem_DistinctLabelFromSibling_Allowed(t *testing.T) {
	// A 2nd count of an item under a label distinct from the sibling's is allowed
	// (and the aggregate stays within baseline: 60 dock + 30 shelf = 90 <= 100).
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectSiblingRows(mock, submissionID, []siblingRow{{id: 700, payload: reconLineLabeled(10, 60, "dock")}})
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "reconciliation_request_items"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(701))
	mock.ExpectCommit()

	_, err := svc.CreateReconciliationItem(ctx, dto.CreateReconciliationItemRequest{
		SubmissionID: submissionID,
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(30), Label: "shelf"}},
	})
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateReconciliationItem_DifferentItemsShareLabelAndBlank_Allowed(t *testing.T) {
	// The label rule is PER item: two DIFFERENT items may share a label, and both may
	// be blank. No conflict across distinct inventory_item_ids.
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectSiblingRows(mock, submissionID, nil)
	expectSnapshotBaselines(mock, map[uint]string{10: "100", 11: "100", 12: "100", 13: "100"})
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "reconciliation_request_items"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()

	_, err := svc.CreateReconciliationItem(ctx, dto.CreateReconciliationItemRequest{
		SubmissionID: submissionID,
		Items: []dto.ReconciliationCountItem{
			{InventoryItemID: 10, Quantity: decPtr(5), Label: "shelf"},
			{InventoryItemID: 11, Quantity: decPtr(5), Label: "shelf"}, // same label, different item
			{InventoryItemID: 12, Quantity: decPtr(5)},                 // blank
			{InventoryItemID: 13, Quantity: decPtr(5)},                 // blank, different item
		},
	})
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateReconciliationItem_LabelTooLong_Rejected(t *testing.T) {
	// A label longer than 255 RUNES is rejected.
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectSiblingRows(mock, submissionID, nil)
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
	mock.ExpectRollback()

	_, err := svc.CreateReconciliationItem(ctx, dto.CreateReconciliationItemRequest{
		SubmissionID: submissionID,
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(5), Label: strings.Repeat("x", 256)}},
	})
	require.Error(t, err)
	var appErr *pkg.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, pkg.ErrReconItemLabelTooLong(ctx, 10, 255).Error(), appErr.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateReconciliationItem_VietnameseLabel255Runes_Allowed(t *testing.T) {
	// A 255-RUNE Vietnamese label is accepted even though it is 765 bytes: the limit
	// is on RUNES (utf8.RuneCountInString), not bytes, so multibyte labels in-limit
	// are not wrongly rejected.
	label := strings.Repeat("ằ", 255) // 255 runes, 765 bytes
	require.Equal(t, 255, utf8.RuneCountInString(label))
	require.Greater(t, len(label), 255)

	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectSiblingRows(mock, submissionID, nil)
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "reconciliation_request_items"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()

	_, err := svc.CreateReconciliationItem(ctx, dto.CreateReconciliationItemRequest{
		SubmissionID: submissionID,
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(5), Label: label}},
	})
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ===================== ROW-LEVEL (count-session) LABEL (issue #73) =====================

func TestCreateReconciliationItem_RowLabel_FirstRowMayBeBlank(t *testing.T) {
	// The first/only row of a user may carry a blank row label.
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectOwnedSiblingRows(mock, submissionID, nil)
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "reconciliation_request_items"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()

	created, err := svc.CreateReconciliationItem(ctx, dto.CreateReconciliationItemRequest{
		SubmissionID: submissionID,
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(30)}},
	})
	require.NoError(t, err)
	assert.Equal(t, "", created.Label)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateReconciliationItem_RowLabel_RequiredOnSecondBlankRow(t *testing.T) {
	// The caller already has a blank live row; a 2nd blank row would break
	// one-unlabelled-per-user, so it's rejected.
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectOwnedSiblingRows(mock, submissionID, []ownedRow{
		{id: 700, owner: reconStaffEmail, label: "", payload: reconLine(10, 20)},
	})
	mock.ExpectRollback()

	_, err := svc.CreateReconciliationItem(ctx, dto.CreateReconciliationItemRequest{
		SubmissionID: submissionID,
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(20)}},
	})
	require.Error(t, err)
	var appErr *pkg.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, pkg.ErrReconRowLabelRequired(ctx).Error(), appErr.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateReconciliationItem_RowLabel_BlankRowWithLabelledSibling_Allowed(t *testing.T) {
	// The caller's only other row is labelled; a blank row is the owner's single
	// unlabelled session and is allowed.
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectOwnedSiblingRows(mock, submissionID, []ownedRow{
		{id: 700, owner: reconStaffEmail, label: "Morning", payload: reconLine(10, 20)},
	})
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "reconciliation_request_items"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(701))
	mock.ExpectCommit()

	created, err := svc.CreateReconciliationItem(ctx, dto.CreateReconciliationItemRequest{
		SubmissionID: submissionID,
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(30)}},
	})
	require.NoError(t, err)
	assert.Equal(t, "", created.Label)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateReconciliationItem_RowLabel_DistinctPerSubmissionAndUser(t *testing.T) {
	// A 2nd row whose row label collides with the caller's OWN existing row label is
	// rejected (distinct per (submission, user)).
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectOwnedSiblingRows(mock, submissionID, []ownedRow{
		{id: 700, owner: reconStaffEmail, label: "Morning", payload: reconLine(10, 20)},
	})
	mock.ExpectRollback()

	_, err := svc.CreateReconciliationItem(ctx, dto.CreateReconciliationItemRequest{
		SubmissionID: submissionID,
		Label:        "Morning",
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(20)}},
	})
	require.Error(t, err)
	var appErr *pkg.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, pkg.ErrReconRowLabelConflict(ctx, "Morning").Error(), appErr.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateReconciliationItem_RowLabel_DistinctFromOwnRow_Allowed(t *testing.T) {
	// A 2nd row with a row label distinct from the caller's existing row is allowed.
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectOwnedSiblingRows(mock, submissionID, []ownedRow{
		{id: 700, owner: reconStaffEmail, label: "Morning", payload: reconLine(10, 20)},
	})
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "reconciliation_request_items"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(701))
	mock.ExpectCommit()

	created, err := svc.CreateReconciliationItem(ctx, dto.CreateReconciliationItemRequest{
		SubmissionID: submissionID,
		Label:        "Afternoon",
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(20)}},
	})
	require.NoError(t, err)
	assert.Equal(t, "Afternoon", created.Label)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateReconciliationItem_RowLabel_AnotherUsersLabelDoesNotConflict_Allowed(t *testing.T) {
	// The row-label rule is per (submission, USER): the caller's first row may reuse a
	// label another user already used, and may be blank (caller has no other row).
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectOwnedSiblingRows(mock, submissionID, []ownedRow{
		{id: 700, owner: reconOtherEmail, label: "Morning", payload: reconLine(10, 20)},
	})
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "reconciliation_request_items"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(701))
	mock.ExpectCommit()

	created, err := svc.CreateReconciliationItem(ctx, dto.CreateReconciliationItemRequest{
		SubmissionID: submissionID,
		Label:        "Morning", // same label as ANOTHER user's row — allowed
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(20)}},
	})
	require.NoError(t, err)
	assert.Equal(t, "Morning", created.Label)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateReconciliationItem_RowLabel_TooLong_Rejected(t *testing.T) {
	// A row label longer than 255 runes is rejected (before any sibling read).
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectOwnedSiblingRows(mock, submissionID, nil)
	mock.ExpectRollback()

	_, err := svc.CreateReconciliationItem(ctx, dto.CreateReconciliationItemRequest{
		SubmissionID: submissionID,
		Label:        strings.Repeat("x", 256),
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(20)}},
	})
	require.Error(t, err)
	var appErr *pkg.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, pkg.ErrReconRowLabelTooLong(ctx, 255).Error(), appErr.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateReconciliationItem_RowLabel_Vietnamese255Runes_Allowed(t *testing.T) {
	// A 255-rune Vietnamese row label (765 bytes) is accepted: the cap is on runes.
	label := strings.Repeat("ằ", 255)
	require.Equal(t, 255, utf8.RuneCountInString(label))
	require.Greater(t, len(label), 255)

	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectOwnedSiblingRows(mock, submissionID, nil)
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "reconciliation_request_items"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()

	created, err := svc.CreateReconciliationItem(ctx, dto.CreateReconciliationItemRequest{
		SubmissionID: submissionID,
		Label:        label,
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(20)}},
	})
	require.NoError(t, err)
	assert.Equal(t, label, created.Label)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateReconciliationItem_FullReplaceWithLabels_Persisted(t *testing.T) {
	// Update is a full-replace of the row's label + payload: the new labeled lines
	// fully replace the old payload. The row's own old counts are excluded from the
	// sibling aggregate (item.ID excluded). Two distinct count labels for item 10 in
	// one row are persisted and surface in the response Items.
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)
	const itemID = uint(777)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectItemLoad(mock, itemID, submissionID, reconStaffEmail, string(models.ReconciliationRequestItemStatusInProgress))
	// The row under update (777) currently counts 90 of item 10; it is EXCLUDED, so
	// the new 30+25=55 is validated against the baseline with no sibling counts.
	expectSiblingRows(mock, submissionID, []siblingRow{{id: 777, payload: reconLine(10, 90)}})
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "reconciliation_request_items" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	updated, err := svc.UpdateReconciliationItem(ctx, dto.UpdateReconciliationItemRequest{
		SubmissionID: submissionID,
		ItemID:       itemID,
		Label:        "Morning count",
		Items: []dto.ReconciliationCountItem{
			{InventoryItemID: 10, Quantity: decPtr(30), Label: "shelf"},
			{InventoryItemID: 10, Quantity: decPtr(25), Label: "dock"},
		},
	})
	require.NoError(t, err)
	// The response carries the replaced row label and both labeled count lines.
	assert.Equal(t, "Morning count", updated.Label)
	require.Len(t, updated.Items, 2)
	assert.Equal(t, "shelf", updated.Items[0].Label)
	assert.Equal(t, "dock", updated.Items[1].Label)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateReconciliationItem_UnlabelledStaysBlankWithLabelledSibling_Allowed(t *testing.T) {
	// The owner's unlabelled session (777) is updated while staying blank; a labelled
	// sibling ("Morning") exists. One-unlabelled-per-user is satisfied, so it's allowed.
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)
	const itemID = uint(777)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectItemLoad(mock, itemID, submissionID, reconStaffEmail, string(models.ReconciliationRequestItemStatusInProgress))
	expectOwnedSiblingRows(mock, submissionID, []ownedRow{
		{id: 700, owner: reconStaffEmail, label: "Morning", payload: reconLine(10, 20)},
		{id: itemID, owner: reconStaffEmail, label: "", payload: reconLine(10, 5)},
	})
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "reconciliation_request_items" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	updated, err := svc.UpdateReconciliationItem(ctx, dto.UpdateReconciliationItemRequest{
		SubmissionID: submissionID,
		ItemID:       itemID,
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(30)}},
	})
	require.NoError(t, err)
	assert.Equal(t, "", updated.Label)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateReconciliationItem_SecondBlankSession_Rejected(t *testing.T) {
	// The owner already has a blank sibling; keeping THIS session blank too would break
	// one-unlabelled-per-user, so it's rejected.
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)
	const itemID = uint(777)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectItemLoad(mock, itemID, submissionID, reconStaffEmail, string(models.ReconciliationRequestItemStatusInProgress))
	expectOwnedSiblingRows(mock, submissionID, []ownedRow{
		{id: 700, owner: reconStaffEmail, label: "", payload: reconLine(10, 20)},
		{id: itemID, owner: reconStaffEmail, label: "Morning", payload: reconLine(10, 5)},
	})
	mock.ExpectRollback()

	_, err := svc.UpdateReconciliationItem(ctx, dto.UpdateReconciliationItemRequest{
		SubmissionID: submissionID,
		ItemID:       itemID,
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(30)}},
	})
	require.Error(t, err)
	var appErr *pkg.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, pkg.ErrReconRowLabelRequired(ctx).Error(), appErr.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateReconciliationItem_LabelCollidesWithSibling_Rejected(t *testing.T) {
	// Renaming this session to a label another of the owner's sessions holds is rejected.
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)
	const itemID = uint(777)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectItemLoad(mock, itemID, submissionID, reconStaffEmail, string(models.ReconciliationRequestItemStatusInProgress))
	expectOwnedSiblingRows(mock, submissionID, []ownedRow{
		{id: 700, owner: reconStaffEmail, label: "Morning", payload: reconLine(10, 20)},
		{id: itemID, owner: reconStaffEmail, label: "Afternoon", payload: reconLine(10, 5)},
	})
	mock.ExpectRollback()

	_, err := svc.UpdateReconciliationItem(ctx, dto.UpdateReconciliationItemRequest{
		SubmissionID: submissionID,
		ItemID:       itemID,
		Label:        "Morning",
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(30)}},
	})
	require.Error(t, err)
	var appErr *pkg.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, pkg.ErrReconRowLabelConflict(ctx, "Morning").Error(), appErr.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateReconciliationItem_KeepsOwnLabel_ExcludedFromUniqueness_Allowed(t *testing.T) {
	// A labelled session keeps its own label; it must not conflict with itself (the
	// current session is excluded from the uniqueness check).
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)
	const itemID = uint(777)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectItemLoad(mock, itemID, submissionID, reconStaffEmail, string(models.ReconciliationRequestItemStatusInProgress))
	expectOwnedSiblingRows(mock, submissionID, []ownedRow{
		{id: itemID, owner: reconStaffEmail, label: "Morning", payload: reconLine(10, 5)},
	})
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "reconciliation_request_items" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	updated, err := svc.UpdateReconciliationItem(ctx, dto.UpdateReconciliationItemRequest{
		SubmissionID: submissionID,
		ItemID:       itemID,
		Label:        "Morning",
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(30)}},
	})
	require.NoError(t, err)
	assert.Equal(t, "Morning", updated.Label)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================ AGGREGATE (cross-row) BASELINE ============================

func TestCreateReconciliationItem_AggregateExceedsBaseline_SecondRowRejected(t *testing.T) {
	// Two staff rows of 80 each against a baseline of 100: the FIRST row already
	// counts 80 (a live sibling, blank label); a SECOND row of 80 under a DISTINCT
	// label ("dock") alone passes the per-row check (80 <= 100) but 80 + 80 = 160 >
	// 100, so the aggregate guard must reject it. The distinct label clears the
	// issue-#73 label rule so the AGGREGATE error is the one that fires (not the
	// label-required error).
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	// One existing live sibling row already counted 80 of item 10.
	expectSiblingRows(mock, submissionID, []siblingRow{{id: 700, payload: reconLine(10, 80)}})
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
	// resolveProductName resolves item 10 -> "Sản phẩm A" on the aggregate-reject branch.
	expectInventoryItemsByIDs(mock, map[uint]string{10: "Sản phẩm A"})
	mock.ExpectRollback()

	// VI locale so we assert the issue's exact reworded VI string.
	ctx = pkg.WithLanguage(ctx, pkg.LangVI)
	_, err := svc.CreateReconciliationItem(ctx, dto.CreateReconciliationItemRequest{
		SubmissionID: submissionID,
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(80), Label: "dock"}},
	})
	require.Error(t, err)
	assert.Equal(t, pkg.ErrorCodeValidation, appErrCode(t, err))
	// Specifically the aggregate error (total 160 across staff submissions), not the
	// per-row exceeds error. The builder now takes the product NAME (not the item ID),
	// so both sides resolve to the same name-bearing message.
	var appErr *pkg.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t,
		pkg.ErrReconItemAggregateExceedsBaseline(ctx, "Sản phẩm A", decimal.NewFromInt(160), decimal.NewFromInt(100)).Error(),
		appErr.Error())
	assert.Contains(t, appErr.Error(), "Sản phẩm A")
	assert.Contains(t, appErr.Error(), "số lượng ghi nhận tại thời điểm bắt đầu đối soát")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateReconciliationItem_FragmentedCounts_Allowed(t *testing.T) {
	// Fragmented counts 60 + 40 against a baseline of 100: an existing sibling row
	// counted 60 (blank label); this new row counts 40 under a DISTINCT non-empty
	// label ("dock"); 60 + 40 = 100 <= 100, so it is allowed. Per the issue-#73
	// rule, a 2nd count of the same item across the live rows needs a non-empty
	// label distinct from the sibling's (which holds the single allowed blank).
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectSiblingRows(mock, submissionID, []siblingRow{{id: 700, payload: reconLine(10, 60)}})
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "reconciliation_request_items"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(701))
	mock.ExpectCommit()

	_, err := svc.CreateReconciliationItem(ctx, dto.CreateReconciliationItemRequest{
		SubmissionID: submissionID,
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(40), Label: "dock"}},
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
	// Sibling already counted 50 of item 10 and 25 of item 11.
	expectSiblingRows(mock, submissionID, []siblingRow{
		{id: 700, payload: `{"items":[{"inventory_item_id":10,"quantity":"50"},{"inventory_item_id":11,"quantity":"25"}]}`},
	})
	expectSnapshotBaselines(mock, map[uint]string{10: "100", 11: "30"})
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

// ============================ UPDATE ============================

func TestUpdateReconciliationItem_StaffWhileOpen_OK(t *testing.T) {
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)
	const itemID = uint(777)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID) // open
	expectItemLoad(mock, itemID, submissionID, reconStaffEmail, string(models.ReconciliationRequestItemStatusInProgress))
	expectSiblingRows(mock, submissionID, nil)
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "reconciliation_request_items" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	item, err := svc.UpdateReconciliationItem(ctx, dto.UpdateReconciliationItemRequest{
		SubmissionID: submissionID,
		ItemID:       itemID,
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(50)}},
	})
	require.NoError(t, err)
	assert.Equal(t, string(models.ReconciliationRequestItemStatusInProgress), item.Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateReconciliationItem_AggregatePushesOverBaseline_Rejected(t *testing.T) {
	// The row under update (777) currently counts 30; a SIBLING row (700) counts 80
	// (blank label). Updating 777 from 30 to 50 under a DISTINCT label ("dock"):
	// 80 (sibling) + 50 (new) = 130 > 100. Rejected on the AGGREGATE guard (the
	// distinct label clears the issue-#73 label rule so the aggregate check is the
	// one that fires). The row's OWN old value of 30 is EXCLUDED from the sibling sum.
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)
	const itemID = uint(777)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectItemLoad(mock, itemID, submissionID, reconStaffEmail, string(models.ReconciliationRequestItemStatusInProgress))
	// ListBySubmission returns BOTH the updated row (777, old 30 — excluded) and the
	// sibling (700, 80 — counted). The exclusion is by row id inside the service.
	expectSiblingRows(mock, submissionID, []siblingRow{
		{id: 700, payload: reconLine(10, 80)},
		{id: 777, payload: reconLine(10, 30)},
	})
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
	// resolveProductName resolves item 10 -> "Sản phẩm A" on the aggregate-reject branch.
	expectInventoryItemsByIDs(mock, map[uint]string{10: "Sản phẩm A"})
	mock.ExpectRollback()

	// VI locale so we assert the issue's exact reworded VI string.
	ctx = pkg.WithLanguage(ctx, pkg.LangVI)
	_, err := svc.UpdateReconciliationItem(ctx, dto.UpdateReconciliationItemRequest{
		SubmissionID: submissionID,
		ItemID:       itemID,
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(50), Label: "dock"}},
	})
	require.Error(t, err)
	assert.Equal(t, pkg.ErrorCodeValidation, appErrCode(t, err))
	var appErr *pkg.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t,
		pkg.ErrReconItemAggregateExceedsBaseline(ctx, "Sản phẩm A", decimal.NewFromInt(130), decimal.NewFromInt(100)).Error(),
		appErr.Error())
	assert.Contains(t, appErr.Error(), "Sản phẩm A")
	assert.Contains(t, appErr.Error(), "số lượng ghi nhận tại thời điểm bắt đầu đối soát")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateReconciliationItem_StaysWithinExcludingOwnOldValue_Allowed(t *testing.T) {
	// Sibling (700) counts 40 (blank label); the updated row (777) currently counts
	// 80. Updating 777 to 60 under a DISTINCT label ("dock"): if its own OLD 80 were
	// wrongly counted, 40+80+60 would exceed; with the row excluded, 40 (sibling) +
	// 60 (new) = 100 <= 100, so it is allowed. This proves the update path excludes
	// the row being replaced from the sibling sum (the distinct label clears the
	// issue-#73 rule so the aggregate path is what is exercised).
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)
	const itemID = uint(777)

	mock.ExpectBegin()
	expectParentReconcileLoad(mock, submissionID)
	expectItemLoad(mock, itemID, submissionID, reconStaffEmail, string(models.ReconciliationRequestItemStatusInProgress))
	expectSiblingRows(mock, submissionID, []siblingRow{
		{id: 700, payload: reconLine(10, 40)},
		{id: 777, payload: reconLine(10, 80)},
	})
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "reconciliation_request_items" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	_, err := svc.UpdateReconciliationItem(ctx, dto.UpdateReconciliationItemRequest{
		SubmissionID: submissionID,
		ItemID:       itemID,
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(60), Label: "dock"}},
	})
	require.NoError(t, err, "40 (sibling) + 60 (new) == 100 must be allowed; own old 80 excluded")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateReconciliationItem_StaffWhileClosed_Rejected(t *testing.T) {
	// Once closed, a staff edit is rejected at the guardParentEditable chokepoint
	// (before the item is even loaded).
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail) // staff: no recon_manage permission in ctx
	const submissionID = uint(50)
	const itemID = uint(777)

	mock.ExpectBegin()
	expectParentReconcileLoadStatus(mock, submissionID, string(models.ReconcileLifecycleStatusClosed))
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

func TestUpdateReconciliationItem_AdminWhileClosed_OK(t *testing.T) {
	// Admin/accountant (recon_manage) may edit ANY row while the submission is
	// closed — the ownership guard is bypassed and the closed-guard lets them through.
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconManageCtx("admin@cim.local")
	const submissionID = uint(50)
	const itemID = uint(777)

	mock.ExpectBegin()
	expectParentReconcileLoadStatus(mock, submissionID, string(models.ReconcileLifecycleStatusClosed))
	// Row owned by a staff member, not the admin — ownership is bypassed.
	expectItemLoad(mock, itemID, submissionID, reconStaffEmail, string(models.ReconciliationRequestItemStatusInProgress))
	expectSiblingRows(mock, submissionID, nil)
	expectSnapshotBaselines(mock, map[uint]string{10: "100"})
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "reconciliation_request_items" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	_, err := svc.UpdateReconciliationItem(ctx, dto.UpdateReconciliationItemRequest{
		SubmissionID: submissionID,
		ItemID:       itemID,
		Items:        []dto.ReconciliationCountItem{{InventoryItemID: 10, Quantity: decPtr(50)}},
	})
	require.NoError(t, err)
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

func TestDeleteReconciliationItem_StaffWhileClosed_Rejected(t *testing.T) {
	// Once closed, staff cannot delete — rejected at the guardParentEditable
	// chokepoint before the item is loaded.
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)
	ctx := reconCtx(reconStaffEmail)
	const submissionID = uint(50)
	const itemID = uint(777)

	mock.ExpectBegin()
	expectParentReconcileLoadStatus(mock, submissionID, string(models.ReconcileLifecycleStatusClosed))
	mock.ExpectRollback()

	err := svc.DeleteReconciliationItem(ctx, dto.DeleteReconciliationItemRequest{
		SubmissionID: submissionID, ItemID: itemID,
	})
	require.Error(t, err)
	assert.Equal(t, pkg.ErrorCodeConflict, appErrCode(t, err))
	assert.NoError(t, mock.ExpectationsWereMet())
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
