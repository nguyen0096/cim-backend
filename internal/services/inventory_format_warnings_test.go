package services

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cim-backend/internal/models"
	"cim-backend/internal/services/dto"
)

func invItem(id uint, name, qty string) *models.InventoryItem {
	it := &models.InventoryItem{
		Product:  &models.Product{Name: name},
		Quantity: decimal.RequireFromString(qty),
	}
	it.ID = id
	return it
}

func TestFormatWarnings_ReconcileStockChangedEmitsItemWarning(t *testing.T) {
	submission := models.InventorySubmission{
		SubmissionType: models.InventorySubmissionTypeReconcile,
		ApprovalStatus: models.InventorySubmissionApprovalStatusPending,
	}
	qty := decimal.RequireFromString("10")
	items := []dto.QuantityItem{
		{InventoryItemID: 5, Quantity: &qty, PrevQuantity: decimal.RequireFromString("8")},
	}
	itemsMap := map[uint]*models.InventoryItem{5: invItem(5, "Bột mì", "12")} // live 12 != prev 8

	warnings, itemWarnings := formatWarnings(submission, items, itemsMap)

	require.Len(t, warnings, 1)
	require.Len(t, itemWarnings, 1)
	assert.Equal(t, uint(5), itemWarnings[0].InventoryItemID)
	assert.Equal(t, dto.SubmissionItemWarningStockChanged, itemWarnings[0].Code)
	assert.Equal(t, warnings[0], itemWarnings[0].Message, "structured message mirrors the string warning")
}

func TestFormatWarnings_DisposeInsufficientEmitsItemWarning(t *testing.T) {
	submission := models.InventorySubmission{
		SubmissionType: models.InventorySubmissionTypeDispose,
		ApprovalStatus: models.InventorySubmissionApprovalStatusPending,
	}
	qty := decimal.RequireFromString("20") // request 20 > available 5
	items := []dto.QuantityItem{
		{InventoryItemID: 7, Quantity: &qty, PrevQuantity: decimal.Zero},
	}
	itemsMap := map[uint]*models.InventoryItem{7: invItem(7, "Đường", "5")}

	warnings, itemWarnings := formatWarnings(submission, items, itemsMap)

	require.Len(t, warnings, 1)
	require.Len(t, itemWarnings, 1)
	assert.Equal(t, uint(7), itemWarnings[0].InventoryItemID)
	assert.Equal(t, dto.SubmissionItemWarningInsufficientQuantity, itemWarnings[0].Code)
	assert.Equal(t, warnings[0], itemWarnings[0].Message)
}

func TestFormatWarnings_NoDiffNoItemWarning(t *testing.T) {
	submission := models.InventorySubmission{
		SubmissionType: models.InventorySubmissionTypeReconcile,
		ApprovalStatus: models.InventorySubmissionApprovalStatusPending,
	}
	qty := decimal.RequireFromString("10")
	items := []dto.QuantityItem{
		{InventoryItemID: 5, Quantity: &qty, PrevQuantity: decimal.RequireFromString("8")},
	}
	itemsMap := map[uint]*models.InventoryItem{5: invItem(5, "Bột mì", "8")} // live == prev

	warnings, itemWarnings := formatWarnings(submission, items, itemsMap)

	assert.Empty(t, warnings)
	assert.Empty(t, itemWarnings)
}
