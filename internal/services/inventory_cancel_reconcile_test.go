package services

import (
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cim-backend/internal/models"
	"cim-backend/pkg"
)

// expectCancelParentLoadStatus queues loadActiveReconcileParent's locking load for
// a cancel: the parent FOR UPDATE read (with the given reconcile_status) + the
// snapshot existence count. approval_status stays 'pending' so the in-flight gate
// passes and the from-state gate is the one that decides.
func expectCancelParentLoadStatus(mock sqlmock.Sqlmock, submissionID uint, reconcileStatus string) {
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "inventory_submissions"`)+`.*FOR UPDATE`).
		WithArgs(submissionID, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "inventory_id", "submission_type", "processing_status", "approval_status", "reconcile_status"}).
			AddRow(submissionID, 10, string(models.InventorySubmissionTypeReconcile), "pending", "pending", reconcileStatus))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "reconciliation_snapshots"`)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
}

// TestCancelReconciliation_AllowedFromStates drives the from-state matrix: cancel
// succeeds from open and closed, writing the terminal reconcile_status='canceled' +
// processing_status='canceled' in one UPDATE under the advisory lock, with NO stock
// mutation (any consume/apply query would be an unexpected statement sqlmock fails
// on).
func TestCancelReconciliation_AllowedFromStates(t *testing.T) {
	for _, from := range []string{
		string(models.ReconcileLifecycleStatusOpen),
		string(models.ReconcileLifecycleStatusClosed),
	} {
		t.Run(from, func(t *testing.T) {
			gormDB, mock := newInventoryServiceTestDB(t)
			svc := newReconItemServiceReal(gormDB)
			const submissionID = uint(50)

			mock.ExpectBegin()
			expectCancelParentLoadStatus(mock, submissionID, from)
			mock.ExpectExec(regexp.QuoteMeta(`SELECT pg_advisory_xact_lock(`)).
				WillReturnResult(sqlmock.NewResult(0, 0))
			mock.ExpectExec(regexp.QuoteMeta(`UPDATE "inventory_submissions" SET`)).
				WithArgs(string(models.InventorySubmissionStatusCanceled),
					string(models.ReconcileLifecycleStatusCanceled),
					sqlmock.AnyArg(), sqlmock.AnyArg(), submissionID).
				WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()

			got, err := svc.CancelReconciliation(reconManageCtx("admin@cim.local"), submissionID)
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, models.ReconcileLifecycleStatusCanceled, got.ReconcileStatus)
			assert.Equal(t, models.InventorySubmissionStatusCanceled, got.ProcessingStatus)
			assert.False(t, got.IsActiveReconcile(), "a canceled reconcile must be non-active")
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestCancelReconciliation_InvalidFromStates asserts the from-gate rejects any
// non-open/closed source (processing/processed/canceled) with a clean 409 and NO
// write: the tx rolls back after the load, no UPDATE is issued. The already-canceled
// case is the idempotency-409 / re-cancel guard.
func TestCancelReconciliation_InvalidFromStates(t *testing.T) {
	for _, from := range []string{
		string(models.ReconcileLifecycleStatusProcessing),
		string(models.ReconcileLifecycleStatusProcessed),
		string(models.ReconcileLifecycleStatusCanceled),
	} {
		t.Run(from, func(t *testing.T) {
			gormDB, mock := newInventoryServiceTestDB(t)
			svc := newReconItemServiceReal(gormDB)
			const submissionID = uint(50)

			mock.ExpectBegin()
			expectCancelParentLoadStatus(mock, submissionID, from)
			mock.ExpectRollback()

			_, err := svc.CancelReconciliation(reconManageCtx("admin@cim.local"), submissionID)
			require.Error(t, err)
			assert.Equal(t, pkg.ErrorCodeConflict, appErrCode(t, err))
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestIsActiveReconcile_IgnoresApprovalStatus asserts the re-based gate no longer
// depends on approval_status: an open/closed pending reconcile is active for any
// approval_status, and a canceled reconcile is non-active by construction.
func TestIsActiveReconcile_IgnoresApprovalStatus(t *testing.T) {
	base := func(rec models.ReconcileLifecycleStatus, approval models.SubmissionApprovalStatus, proc models.SubmissionProcessingStatus) models.InventorySubmission {
		return models.InventorySubmission{
			SubmissionType:   models.InventorySubmissionTypeReconcile,
			ProcessingStatus: proc,
			ApprovalStatus:   approval,
			ReconcileStatus:  rec,
		}
	}
	const pend = models.InventorySubmissionStatusPending

	for _, approval := range []models.SubmissionApprovalStatus{
		models.InventorySubmissionApprovalStatusPending,
		models.InventorySubmissionApprovalStatusApproved,
		models.InventorySubmissionApprovalStatusRejected,
	} {
		assert.True(t, isActiveReconcile(base(models.ReconcileLifecycleStatusOpen, approval, pend)),
			"open+pending must be active regardless of approval_status %q", approval)
		assert.True(t, isActiveReconcile(base(models.ReconcileLifecycleStatusClosed, approval, pend)),
			"closed+pending must be active regardless of approval_status %q", approval)
	}

	// Canceled is non-active with no `!= canceled` bolt-on.
	assert.False(t, isActiveReconcile(base(models.ReconcileLifecycleStatusCanceled,
		models.InventorySubmissionApprovalStatusPending, models.InventorySubmissionStatusCanceled)))
}

// TestCancelReconciliation_RequiresReconManage asserts the service RBAC backstop:
// a caller without recon_manage is forbidden and issues NO DB query.
func TestCancelReconciliation_RequiresReconManage(t *testing.T) {
	gormDB, mock := newInventoryServiceTestDB(t)
	svc := newReconItemServiceReal(gormDB)

	_, err := svc.CancelReconciliation(reconCtx("staff@cim.local"), 50)
	require.Error(t, err)
	assert.True(t, pkg.IsErrorCode(err, pkg.ErrorCodeForbidden))
	assert.NoError(t, mock.ExpectationsWereMet())
}
