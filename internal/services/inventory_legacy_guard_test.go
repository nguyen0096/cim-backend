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

	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
)

// newLegacyGuardService wires a real inventoryService over the sqlmock-backed
// gorm handle using the actual submission + snapshot repositories, so the
// legacy-path guard (guardNotInitiatedReconcile) is exercised end-to-end.
func newLegacyGuardService(gormDB *gorm.DB) *inventoryService {
	baseRepo := repository.NewBaseRepository(gormDB)
	return &inventoryService{
		inventorySubmissionRepo: repository.NewInventorySubmissionRepository(baseRepo),
		snapshotRepo:            repository.NewReconciliationSnapshotRepository(baseRepo),
	}
}

// approveCtx carries a user email plus the approve permission required by
// ProcessSubmission's in-service RBAC guard.
func approveCtx() context.Context {
	ctx := pkg.WithUserEmail(context.Background(), "admin@cim.local")
	perms := map[pkg.UserPermission]struct{}{
		{Resource: pkg.RBACResourceInventorySubmissions, Action: pkg.RBACActionApprove}: {},
	}
	return context.WithValue(ctx, pkg.AuthContextKeyUserPermissions, perms)
}

// expectPendingReconcileLoad mocks GetByID returning a pending reconcile
// submission (the shape both legacy paths first load), and the snapshot
// existence count returning 1 — the new-model marker that means this
// submission was started via reconcile-initiate.
func expectInitiatedReconcileLoad(mock sqlmock.Sqlmock, submissionID uint) {
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "inventory_submissions"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "inventory_id", "submission_type", "approval_status", "processing_status", "payload"}).
			AddRow(submissionID, 7, string(models.InventorySubmissionTypeReconcile),
				string(models.InventorySubmissionApprovalStatusPending),
				string(models.InventorySubmissionStatusPending), []byte("")))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "reconciliation_snapshots"`)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
}

// TestProcessSubmission_RejectsInitiatedReconcile is the approve-before-synthesis
// case: an initiated reconcile (pending, empty payload, has snapshot rows) must
// NOT be approvable through the legacy ProcessSubmission path, or processSubmission
// would fail unmarshalling the empty payload and leave it approved/failed.
func TestProcessSubmission_RejectsInitiatedReconcile(t *testing.T) {
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newLegacyGuardService(gormDB)

	const submissionID = uint(42)
	expectInitiatedReconcileLoad(mock, submissionID)
	// No UPDATE/INSERT expectations: the guard must short-circuit before any write.

	resp, err := svc.ProcessSubmission(approveCtx(), dto.SubmissionApprovalRequest{
		SubmissionID: submissionID,
		Action:       string(models.InventorySubmissionActionApprove),
	})
	require.Error(t, err)
	assert.Nil(t, resp)

	var appErr *pkg.AppError
	require.True(t, errors.As(err, &appErr), "expected *pkg.AppError, got %T", err)
	assert.Equal(t, pkg.ErrorCodeConflict, appErr.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestProcessSubmission_AllowsRejectOfInitiatedReconcile is the safety carve-out:
// action=reject on an initiated reconcile (pending, empty payload, has snapshot
// rows) must be ALLOWED. Reject only marks the submission rejected/canceled and
// never unmarshals the payload, so it carries no snapshot-bypass risk. The guard
// (which runs only for approve) must therefore NOT fire here — no snapshot count
// query is issued — and the submission ends rejected/canceled.
func TestProcessSubmission_AllowsRejectOfInitiatedReconcile(t *testing.T) {
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newLegacyGuardService(gormDB)

	const submissionID = uint(42)

	// Load the initiated reconcile placeholder. Crucially, NO snapshot count query
	// is queued: the guard must be skipped for reject.
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "inventory_submissions"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "inventory_id", "submission_type", "approval_status", "processing_status", "payload"}).
			AddRow(submissionID, 7, string(models.InventorySubmissionTypeReconcile),
				string(models.InventorySubmissionApprovalStatusPending),
				string(models.InventorySubmissionStatusPending), []byte("")))

	// Reject persists approval_status=rejected then processing_status=canceled.
	// Each repo Updates runs in GORM's implicit transaction (Begin/Commit).
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "inventory_submissions"`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "inventory_submissions"`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	resp, err := svc.ProcessSubmission(approveCtx(), dto.SubmissionApprovalRequest{
		SubmissionID: submissionID,
		Action:       string(models.InventorySubmissionActionReject),
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, models.InventorySubmissionApprovalStatusRejected, resp.ApprovalStatus)
	assert.Equal(t, models.InventorySubmissionStatusCanceled, resp.ProcessingStatus)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestUpdateSubmission_RejectsInitiatedReconcile is the overwrite-attempt case: a
// caller must NOT be able to overwrite the initiated reconcile placeholder with a
// legacy payload (which would carry caller-supplied prev_quantity and bypass the
// reconciliation_snapshots baseline).
func TestUpdateSubmission_RejectsInitiatedReconcile(t *testing.T) {
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newLegacyGuardService(gormDB)

	const submissionID = uint(42)
	expectInitiatedReconcileLoad(mock, submissionID)
	// No payload UPDATE expectation: the guard must short-circuit before any write.

	resp, err := svc.UpdateSubmission(context.Background(), dto.UpdateSubmissionRequest{
		SubmissionID: submissionID,
		Items:        []dto.QuantityItem{{InventoryItemID: 11}},
	})
	require.Error(t, err)
	assert.Nil(t, resp)

	var appErr *pkg.AppError
	require.True(t, errors.As(err, &appErr), "expected *pkg.AppError, got %T", err)
	assert.Equal(t, pkg.ErrorCodeConflict, appErr.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGuardNotInitiatedReconcile_Scoping confirms the guard is correctly scoped:
//   - a legacy reconcile submission (no snapshot rows) passes (nil error), so the
//     existing legacy flow is unaffected;
//   - a dispose/transfer submission is never checked (no snapshot query issued at
//     all), so non-reconcile flows are untouched.
func TestGuardNotInitiatedReconcile_Scoping(t *testing.T) {
	t.Run("legacy reconcile without snapshots is allowed", func(t *testing.T) {
		gormDB, mock := newInventoryServiceTestDB(t)
		svc := newLegacyGuardService(gormDB)

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "reconciliation_snapshots"`)).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

		err := svc.guardNotInitiatedReconcile(context.Background(), &models.InventorySubmission{
			Base:           models.Base{ID: 7},
			SubmissionType: models.InventorySubmissionTypeReconcile,
		})
		require.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("dispose submission is never snapshot-checked", func(t *testing.T) {
		gormDB, mock := newInventoryServiceTestDB(t)
		svc := newLegacyGuardService(gormDB)
		// No snapshot query expected: a non-reconcile type returns immediately.

		err := svc.guardNotInitiatedReconcile(context.Background(), &models.InventorySubmission{
			Base:           models.Base{ID: 9},
			SubmissionType: models.InventorySubmissionTypeDispose,
		})
		require.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
