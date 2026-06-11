package main

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"cim-backend/internal/models"
	"cim-backend/pkg"
)

// TestCorrectedCloneCreatedByPreserved is a focused, DB-backed (in-memory
// sqlite, no external service) assertion of the audit-integrity fix in the
// reconcile apply path: when cloning a corrected submission, models.Base
// .BeforeCreate unconditionally stamps created_by/updated_by from the context
// user (the system reconcile actor on this path). The follow-up hook-free
// UpdateColumn must restore created_by to the ORIGINAL submitter while leaving
// updated_by as the system actor and created_at untouched.
func TestCorrectedCloneCreatedByPreserved(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.InventorySubmission{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	const origSubmitter = "alice@cim.local"
	origCreatedAt := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)

	// The apply tx runs with the system actor in context (see openReconcileDB /
	// the apply path); BeforeCreate reads exactly this value.
	ctx := pkg.WithUserEmail(context.Background(), reconcileActor)
	tx := db.WithContext(ctx)

	// Mirror the clone construction in applyResolution: original created_at /
	// created_by carried over, ID=0.
	clone := models.InventorySubmission{
		Base: models.Base{
			CreatedAt: origCreatedAt,
			CreatedBy: origSubmitter,
		},
		InventoryID:      7,
		SubmissionType:   models.InventorySubmissionTypeReconcile,
		ProcessingStatus: models.InventorySubmissionStatusCompleted,
		ApprovalStatus:   models.InventorySubmissionApprovalStatusApproved,
		Reason:           reconcileOneOffReason,
	}

	if err := tx.Create(&clone).Error; err != nil {
		t.Fatalf("create clone: %v", err)
	}

	// Sanity: confirm the regression actually exists — BeforeCreate clobbered
	// created_by with the system actor despite us setting orig above.
	var afterCreate models.InventorySubmission
	if err := tx.First(&afterCreate, clone.ID).Error; err != nil {
		t.Fatalf("reload after create: %v", err)
	}
	if afterCreate.CreatedBy != reconcileActor {
		t.Fatalf("precondition: expected BeforeCreate to stamp created_by=%q, got %q",
			reconcileActor, afterCreate.CreatedBy)
	}

	// The fix: restore the original submitter with a hook-free column update.
	if err := tx.Model(&clone).UpdateColumn("created_by", origSubmitter).Error; err != nil {
		t.Fatalf("restore created_by: %v", err)
	}

	var got models.InventorySubmission
	if err := tx.First(&got, clone.ID).Error; err != nil {
		t.Fatalf("reload after fix: %v", err)
	}

	if got.CreatedBy != origSubmitter {
		t.Errorf("created_by = %q, want original submitter %q", got.CreatedBy, origSubmitter)
	}
	if !got.CreatedAt.Equal(origCreatedAt) {
		t.Errorf("created_at = %v, want preserved %v", got.CreatedAt, origCreatedAt)
	}
	// updated_by intentionally stays the system actor that performed the
	// correction; UpdateColumn must not have touched it.
	if got.UpdatedBy != reconcileActor {
		t.Errorf("updated_by = %q, want system actor %q (UpdateColumn must skip BeforeUpdate)",
			got.UpdatedBy, reconcileActor)
	}
}
