package repository

import (
	"testing"

	"cim-backend/internal/models"

	"github.com/stretchr/testify/assert"
)

// sqlActiveReconcilePredicate evaluates the active-reconcile SQL predicate the
// ListActiveReconciliations query emits — submission_type='reconcile' AND
// processing_status='pending' AND reconcile_status IN (activeReconcileStatuses) —
// over an in-memory row, sourced from the SAME activeReconcileStatuses var the
// query uses. It is the SQL-layer counterpart to models.IsActiveReconcile.
func sqlActiveReconcilePredicate(s models.InventorySubmission) bool {
	if s.SubmissionType != models.InventorySubmissionTypeReconcile {
		return false
	}
	if s.ProcessingStatus != models.InventorySubmissionStatusPending {
		return false
	}
	for _, st := range activeReconcileStatuses {
		if s.ReconcileStatus == st {
			return true
		}
	}
	return false
}

// TestActiveReconcilePredicate_AntiDivergence drives BOTH the Go helper
// (models.IsActiveReconcile) and the SQL predicate (over the shared
// activeReconcileStatuses source) across the same fixtures and asserts they never
// disagree — the two layers are one definition. A canceled reconcile is non-active
// in both by construction (no `!= canceled` bolt-on).
func TestActiveReconcilePredicate_AntiDivergence(t *testing.T) {
	fix := func(typ models.SubmissionType, proc models.SubmissionProcessingStatus, rec models.ReconcileLifecycleStatus) models.InventorySubmission {
		return models.InventorySubmission{SubmissionType: typ, ProcessingStatus: proc, ReconcileStatus: rec}
	}
	const rc = models.InventorySubmissionTypeReconcile
	const pend = models.InventorySubmissionStatusPending

	cases := []struct {
		name string
		row  models.InventorySubmission
		want bool
	}{
		{"open+pending", fix(rc, pend, models.ReconcileLifecycleStatusOpen), true},
		{"closed+pending", fix(rc, pend, models.ReconcileLifecycleStatusClosed), true},
		{"canceled+pending", fix(rc, pend, models.ReconcileLifecycleStatusCanceled), false},
		{"canceled+canceled", fix(rc, models.InventorySubmissionStatusCanceled, models.ReconcileLifecycleStatusCanceled), false},
		{"processing", fix(rc, pend, models.ReconcileLifecycleStatusProcessing), false},
		{"processed+completed", fix(rc, models.InventorySubmissionStatusCompleted, models.ReconcileLifecycleStatusProcessed), false},
		{"open+completed", fix(rc, models.InventorySubmissionStatusCompleted, models.ReconcileLifecycleStatusOpen), false},
		{"dispose", fix(models.InventorySubmissionTypeDispose, pend, ""), false},
		{"transfer+open", fix(models.InventorySubmissionTypeTransfer, pend, models.ReconcileLifecycleStatusOpen), false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			goHelper := c.row.IsActiveReconcile()
			sqlPred := sqlActiveReconcilePredicate(c.row)
			assert.Equal(t, c.want, goHelper, "Go helper")
			assert.Equal(t, goHelper, sqlPred, "Go helper and SQL predicate must agree (anti-divergence)")
		})
	}
}
