package services

import (
	"context"
	"testing"

	"cim-backend/internal/mocks/repositorymocks"
	"cim-backend/internal/models"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validSourceItem is an active item whose on-hand matches its single unconsumed
// purchase transaction, so getActiveInventoryItems' state validation passes.
func validSourceItem(id, inventoryID uint) *models.InventoryItem {
	return &models.InventoryItem{
		Base:        models.Base{ID: id},
		InventoryID: inventoryID,
		ProductID:   1,
		Quantity:    decimal.NewFromInt(10),
		Status:      models.InventoryItemStatusActive,
		Product:     &models.Product{Base: models.Base{ID: 1}, Name: "P1"},
		ConsumableTransactions: []*models.InventoryTransaction{
			{Base: models.Base{ID: 1}, Quantity: decimal.NewFromInt(10), ConsumedQuantity: decimal.Zero},
		},
	}
}

// Test_consumeFIFO_RejectsNegativeQuantity guards against on-hand inflation from
// item.Quantity.Sub(negative) when the FIFO loop is skipped.
func Test_consumeFIFO_RejectsNegativeQuantity(t *testing.T) {
	ctx := context.Background()
	svc := &inventoryService{}

	item := validSourceItem(1, 1)
	original := item.Quantity

	handler := func(i *models.InventoryItem, txn *models.InventoryTransaction, q decimal.Decimal) []*models.InventoryTransaction {
		return []*models.InventoryTransaction{{InventoryItemID: i.ID, Quantity: q}}
	}

	changes, txns, err := svc.consumeFIFO(ctx, newProcessingState(nil, nil), []*models.InventoryItem{item},
		map[uint]decimal.Decimal{1: decimal.NewFromInt(-20)}, handler)

	require.Error(t, err)
	assert.Nil(t, changes)
	assert.Nil(t, txns)
	assert.True(t, item.Quantity.Equal(original), "on-hand must be untouched, got %s", item.Quantity)
}

// newGuardService wires only the repos the guards touch before rejecting.
func newGuardService(itemRepo *repositorymocks.InventoryItemRepository, invRepo *repositorymocks.InventoryRepository) *inventoryService {
	return &inventoryService{
		inventoryRepo:     invRepo,
		inventoryItemRepo: itemRepo,
	}
}

func TestCreateDisposeSubmission_RejectsNonPositiveQuantity(t *testing.T) {
	for _, q := range []int64{-20, 0} {
		ctx := context.Background()
		itemRepo := repositorymocks.NewInventoryItemRepository(t)
		svc := newGuardService(itemRepo, nil)

		itemRepo.On("GetActiveInventoryItems", ctx, uint(1), []uint{1}).
			Return([]*models.InventoryItem{validSourceItem(1, 1)}, nil)

		qty := decimal.NewFromInt(q)
		_, err := svc.CreateDisposeSubmission(ctx, dto.DisposeInventoryRequest{
			InventoryID: 1,
			Items:       []dto.QuantityItem{{InventoryItemID: 1, Quantity: &qty}},
		})

		require.Error(t, err, "quantity %d must be rejected", q)
		var appErr *pkg.AppError
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, pkg.ErrorCodeValidation, appErr.Code)
	}
}

func TestValidateDisposeUpdate_RejectsNonPositiveQuantity(t *testing.T) {
	for _, q := range []int64{-20, 0} {
		ctx := context.Background()
		itemRepo := repositorymocks.NewInventoryItemRepository(t)
		svc := newGuardService(itemRepo, nil)

		itemRepo.On("GetActiveInventoryItems", ctx, uint(1), []uint{1}).
			Return([]*models.InventoryItem{validSourceItem(1, 1)}, nil)

		qty := decimal.NewFromInt(q)
		err := svc.validateDisposeUpdate(ctx, 1, []dto.QuantityItem{{InventoryItemID: 1, Quantity: &qty}})

		require.Error(t, err, "quantity %d must be rejected", q)
		var appErr *pkg.AppError
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, pkg.ErrorCodeValidation, appErr.Code)
	}
}

func TestValidateTransferUpdate_RejectsNonPositiveQuantity(t *testing.T) {
	for _, q := range []int64{-30, 0} {
		ctx := context.Background()
		itemRepo := repositorymocks.NewInventoryItemRepository(t)
		svc := newGuardService(itemRepo, nil)

		itemRepo.On("GetActiveInventoryItems", ctx, uint(1), []uint{1}).
			Return([]*models.InventoryItem{validSourceItem(1, 1)}, nil)

		submission := &models.InventorySubmission{
			Payload: []byte(`{"source_inventory_id":1,"destination_inventory_id":2}`),
		}
		qty := decimal.NewFromInt(q)
		err := svc.validateTransferUpdate(ctx, submission, []dto.QuantityItem{{InventoryItemID: 1, Quantity: &qty}})

		require.Error(t, err, "quantity %d must be rejected", q)
		var appErr *pkg.AppError
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, pkg.ErrorCodeValidation, appErr.Code)
	}
}

func TestCreateTransferSubmission_RejectsNonPositiveQuantity(t *testing.T) {
	for _, q := range []int64{-30, 0} {
		ctx := context.Background()
		itemRepo := repositorymocks.NewInventoryItemRepository(t)
		invRepo := repositorymocks.NewInventoryRepository(t)
		svc := newGuardService(itemRepo, invRepo)

		invRepo.On("GetByID", ctx, uint(1)).Return(&models.Inventory{Base: models.Base{ID: 1}}, nil)
		invRepo.On("GetByID", ctx, uint(2)).Return(&models.Inventory{Base: models.Base{ID: 2}}, nil)
		itemRepo.On("GetActiveInventoryItems", ctx, uint(1), []uint{1}).
			Return([]*models.InventoryItem{validSourceItem(1, 1)}, nil)

		qty := decimal.NewFromInt(q)
		_, err := svc.CreateTransferSubmission(ctx, dto.TransferInventoryRequest{
			SourceInventoryID:      1,
			DestinationInventoryID: 2,
			Items:                  []dto.QuantityItem{{InventoryItemID: 1, Quantity: &qty}},
		})

		require.Error(t, err, "quantity %d must be rejected", q)
		var appErr *pkg.AppError
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, pkg.ErrorCodeValidation, appErr.Code)
	}
}
