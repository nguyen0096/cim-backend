package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/pkg"
)

// newAwaitingProcessingService wires a real inventoryService over a sqlmock-backed
// gorm handle, so ListActiveReconciliations is exercised end-to-end
// through the real repository query (no mocked repo) and the lightweight mapper.
func newAwaitingProcessingService(t *testing.T) (*inventoryService, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: db}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Error),
	})
	require.NoError(t, err)
	baseRepo := repository.NewBaseRepository(gormDB)
	return &inventoryService{
		inventorySubmissionRepo: repository.NewInventorySubmissionRepository(baseRepo),
		baseRepo:                baseRepo,
	}, mock
}

// reconViewCtxAwaiting returns a context carrying the recon_item_view permission —
// the shared read action held by staff AND admin/accountant — so the service-layer
// auth gate passes. Uses a staff identity to make explicit that staff (not just
// admin/accountant) may read this queue.
func reconViewCtxAwaiting() context.Context {
	ctx := pkg.WithUserEmail(context.Background(), "staff@cim.local")
	perms := map[pkg.UserPermission]struct{}{
		{Resource: pkg.RBACResourceInventorySubmissions, Action: pkg.RBACActionReconItemView}: {},
	}
	return context.WithValue(ctx, pkg.AuthContextKeyUserPermissions, perms)
}

// TestListActiveReconciliations_RequiresReconItemView verifies the
// service-layer auth gate: without the recon_item_view permission the call returns
// a 403-coded forbidden error and issues NO DB query (auth is enforced in the
// service, not via route middleware).
func TestListActiveReconciliations_RequiresReconItemView(t *testing.T) {
	svc, mock := newAwaitingProcessingService(t)

	// A caller with NO recon_item_view permission in context.
	ctx := pkg.WithUserEmail(context.Background(), "nobody@cim.local")

	rows, total, err := svc.ListActiveReconciliations(ctx, models.ListParams{Page: 1, Limit: 20, Sort: "updated_at", Order: "desc"}, nil)
	require.Error(t, err)
	assert.Nil(t, rows)
	assert.Zero(t, total)
	assert.True(t, pkg.IsErrorCode(err, pkg.ErrorCodeForbidden), "expected a 403/forbidden error, got %v", err)
	// No query may have run — the gate short-circuits before touching the repo.
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestListActiveReconciliations_AllowsStaffWithReconItemView verifies
// that a staff caller holding only recon_item_view (NOT recon_manage) passes the
// gate and reaches the repository — staff must see the same active-reconcile queue
// as admin/accountant.
func TestListActiveReconciliations_AllowsStaffWithReconItemView(t *testing.T) {
	svc, mock := newAwaitingProcessingService(t)

	mock.ExpectQuery(`SELECT count\(\*\) FROM "inventory_submissions"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`SELECT \* FROM "inventory_submissions"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	rows, total, err := svc.ListActiveReconciliations(reconViewCtxAwaiting(),
		models.ListParams{Page: 1, Limit: 20, Sort: "updated_at", Order: "desc"}, nil)
	require.NoError(t, err, "a staff caller with recon_item_view must be allowed")
	assert.Zero(t, total)
	assert.Empty(t, rows)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestListActiveReconciliations_LightweightMapping verifies the
// authorized path: the rows map to dto.SubmissionResponse via the LIGHTWEIGHT
// mapper — embedded inventory + reconcile_status are present, the synthesized
// review fields (review_label / count_breakdown) are ABSENT (zero value, omitempty),
// and Items serializes as `[]` (not null) because it is NOT omitempty and is
// initialized to an empty slice.
func TestListActiveReconciliations_LightweightMapping(t *testing.T) {
	svc, mock := newAwaitingProcessingService(t)

	mock.ExpectQuery(`SELECT count\(\*\) FROM "inventory_submissions"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT \* FROM "inventory_submissions"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "inventory_id", "submission_type", "processing_status", "approval_status", "reconcile_status", "reason"}).
			AddRow(7, 10, "reconcile", "pending", "pending", "open", "monthly count"))
	mock.ExpectQuery(`SELECT \* FROM "inventories"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(10, "Main Warehouse"))

	rows, total, err := svc.ListActiveReconciliations(reconViewCtxAwaiting(),
		models.ListParams{Page: 1, Limit: 20, Sort: "updated_at", Order: "desc"}, nil)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, rows, 1)
	require.NoError(t, mock.ExpectationsWereMet())

	got := rows[0]
	assert.Equal(t, uint(7), got.ID)
	assert.Equal(t, uint(10), got.InventoryID)
	assert.Equal(t, models.InventorySubmissionTypeReconcile, got.SubmissionType)
	assert.Equal(t, models.InventorySubmissionStatusPending, got.Status)
	assert.Equal(t, models.ReconcileLifecycleStatusOpen, got.ReconcileStatus)
	assert.Equal(t, "monthly count", got.Reason)
	require.NotNil(t, got.Inventory, "the inventory must be preloaded and embedded")
	assert.Equal(t, "Main Warehouse", got.Inventory.Name)

	// Lightweight mapper drops the synthesized review fields.
	assert.Empty(t, got.ReviewLabel, "review_label must be dropped on the queue surface")
	assert.Empty(t, got.CountBreakdown, "count_breakdown must be dropped on the queue surface")

	// Items must be a non-nil empty slice so it serializes as `"items":[]`, not null.
	require.NotNil(t, got.Items)
	assert.Len(t, got.Items, 0)
	raw, err := json.Marshal(got)
	require.NoError(t, err)
	var asMap map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &asMap))
	assert.JSONEq(t, `[]`, string(asMap["items"]), "items must serialize as [] not null")
	_, hasReviewLabel := asMap["review_label"]
	assert.False(t, hasReviewLabel, "review_label must not appear in the JSON")
	_, hasBreakdown := asMap["count_breakdown"]
	assert.False(t, hasBreakdown, "count_breakdown must not appear in the JSON")
}
