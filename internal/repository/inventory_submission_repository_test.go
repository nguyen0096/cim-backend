package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"cim-backend/internal/models"
	"cim-backend/pkg"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInventorySubmissionRepository_Create_TranslatesActivePendingConflict
// verifies the repo maps a uq_inventory_submissions_one_active_pending violation
// to ErrActivePendingReconcileConflict (the race-loser case), passes unrelated
// errors through, and leaves a successful insert clean.
func TestInventorySubmissionRepository_Create_TranslatesActivePendingConflict(t *testing.T) {
	submission := func() *models.InventorySubmission {
		return &models.InventorySubmission{
			InventoryID:    7,
			SubmissionType: models.InventorySubmissionTypeReconcile,
		}
	}

	t.Run("index violation -> domain conflict", func(t *testing.T) {
		gormDB, mock := setupTestDB(t)
		repo := NewInventorySubmissionRepository(NewBaseRepository(gormDB))

		// An authenticated context is required so models.Base.BeforeCreate passes
		// and the INSERT (and the simulated 23505 violation) actually runs.
		ctx := pkg.WithUserEmail(context.Background(), "test@cim.local")

		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "inventory_submissions"`)).
			WillReturnError(errors.New(`ERROR: duplicate key value violates unique constraint "uq_inventory_submissions_one_active_pending" (SQLSTATE 23505)`))
		mock.ExpectRollback()

		err := repo.Create(ctx, submission())
		require.Error(t, err)
		assert.True(t, pkg.IsErrorCode(err, pkg.ErrorCodeActivePendingReconcileConflict),
			"expected ErrorCodeActivePendingReconcileConflict, got %v", err)
	})

	t.Run("unrelated unique violation passes through", func(t *testing.T) {
		gormDB, mock := setupTestDB(t)
		repo := NewInventorySubmissionRepository(NewBaseRepository(gormDB))

		ctx := pkg.WithUserEmail(context.Background(), "test@cim.local")

		raw := errors.New(`ERROR: duplicate key value violates unique constraint "some_other_unique" (SQLSTATE 23505)`)
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "inventory_submissions"`)).
			WillReturnError(raw)
		mock.ExpectRollback()

		err := repo.Create(ctx, submission())
		require.Error(t, err)
		assert.False(t, pkg.IsErrorCode(err, pkg.ErrorCodeActivePendingReconcileConflict),
			"unrelated unique violation must not be mapped to the active-pending conflict")
	})
}

// TestActivePendingReconcileConflict_Localization verifies the domain conflict
// defers localization to the request language (EN fallback / VI).
func TestActivePendingReconcileConflict_Localization(t *testing.T) {
	err := pkg.ErrActivePendingReconcileConflict(7, nil)

	en := err.LocalizedMessage(pkg.WithLanguage(context.Background(), pkg.LangEN))
	vi := err.LocalizedMessage(pkg.WithLanguage(context.Background(), pkg.LangVI))

	assert.Contains(t, en, "7")
	assert.Contains(t, vi, "7")
	assert.NotEqual(t, en, vi, "EN and VI messages must differ")
}
