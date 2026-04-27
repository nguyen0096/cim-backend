package services

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"cim-backend/internal/mocks/repositorymocks"
	"cim-backend/internal/models"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
	"cim-backend/pkg/testutil/fixture"
)

func Test_generatePurchaseOrderNumber(t *testing.T) {
	// Setup
	mockRepo := repositorymocks.NewPurchaseOrderRepository(t)
	// Note: Service mocks are not used in this test as it's not needed for generatePurchaseOrderNumber
	service := NewPurchaseOrderService(mockRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).(*purchaseOrderService)

	t.Run("should generate valid purchase order number format", func(t *testing.T) {
		// Execute
		orderNumber, err := service.generatePurchaseOrderNumber()

		// Assertions
		require.NoError(t, err)
		assert.NotEmpty(t, orderNumber)

		// Verify format: PO-YYMMDD-HHMMSS-XX
		pattern := `^PO-\d{6}-\d{6}-[A-Z0-9]{2}$`
		matched, err := regexp.MatchString(pattern, orderNumber)
		require.NoError(t, err)
		assert.True(t, matched, "Order number %s should match pattern %s", orderNumber, pattern)

		// Verify length: PO- (3) + YYMMDD (6) + - (1) + HHMMSS (6) + - (1) + XX (2) = 19 characters
		expectedLength := len("PO-250925-143052-AB")
		assert.Equal(t, expectedLength, len(orderNumber))
	})

	t.Run("should generate unique order numbers on consecutive calls", func(t *testing.T) {
		// Execute multiple generations
		orderNumber1, err1 := service.generatePurchaseOrderNumber()
		orderNumber2, err2 := service.generatePurchaseOrderNumber()

		// Assertions
		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.NotEqual(t, orderNumber1, orderNumber2, "Consecutive calls should generate different order numbers")
	})
}

func TestCreatePurchaseOrder(t *testing.T) {
	t.Run("should create purchase order with auto-generated order number when empty", func(t *testing.T) {
		// Setup
		mockRepo := repositorymocks.NewPurchaseOrderRepository(t)
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockUnitRepo := repositorymocks.NewUnitRepository(t)
		service := NewPurchaseOrderService(mockRepo, nil, mockUnitRepo, mockProductRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil)

		inventory := fixture.ValidInventory()
		unit := fixture.ValidBaseUnit()
		product := fixture.ValidProduct(unit.ID)
		supplier := fixture.ValidSupplier()
		po := fixture.ValidPurchaseOrder(inventory.ID, product.ID, supplier.ID, unit.ID)

		// Setup mock - expect Create to be called with purchase order that has generated order number
		mockProductRepo.On("GetByID", mock.Anything, product.ID).Return(&product, nil)
		mockUnitRepo.On("GetByID", mock.Anything, unit.ID).Return(&unit, nil)

		mockRepo.On("Create", mock.Anything, mock.MatchedBy(func(po *models.PurchaseOrder) bool {
			// Check that order number was generated and follows the expected pattern
			pattern := `^PO-\d{6}-\d{6}-[A-Z0-9]{2}$`
			matched, _ := regexp.MatchString(pattern, po.OrderNumber)
			return matched && po.OrderNumber != ""
		})).Return(nil)

		mockRepo.On("GetByID", mock.AnythingOfType("uint")).Return(&po, nil)

		// Execute
		err := service.CreatePurchaseOrder(context.Background(), &po)

		// Assertions
		require.NoError(t, err)
		assert.NotEmpty(t, po.OrderNumber, "Order number should be generated")

		// Verify the generated order number format
		pattern := `^PO-\d{6}-\d{6}-[A-Z0-9]{2}$`
		matched, err := regexp.MatchString(pattern, po.OrderNumber)
		require.NoError(t, err)
		assert.True(t, matched, "Generated order number should match expected format")

		mockRepo.AssertExpectations(t)
	})

	t.Run("should create purchase order without changing provided order number", func(t *testing.T) {
		// Setup
		mockRepo := repositorymocks.NewPurchaseOrderRepository(t)
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockUnitRepo := repositorymocks.NewUnitRepository(t)
		service := NewPurchaseOrderService(mockRepo, nil, mockUnitRepo, mockProductRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil)

		inventory := fixture.ValidInventory()
		unit := fixture.ValidBaseUnit()
		product := fixture.ValidProduct(unit.ID)
		supplier := fixture.ValidSupplier()
		po := fixture.ValidPurchaseOrder(inventory.ID, product.ID, supplier.ID, unit.ID)
		existingOrderNumber := "CUSTOM-PO-123"
		po.OrderNumber = existingOrderNumber

		// Setup mock
		mockProductRepo.On("GetByID", mock.Anything, product.ID).Return(&product, nil)
		mockUnitRepo.On("GetByID", mock.Anything, unit.ID).Return(&unit, nil)
		mockRepo.On("Create", mock.Anything, mock.MatchedBy(func(po *models.PurchaseOrder) bool {
			return po.OrderNumber == existingOrderNumber
		})).Return(nil).Once()
		mockRepo.On("GetByID", mock.AnythingOfType("uint")).Return(&po, nil).Once()

		// Execute
		err := service.CreatePurchaseOrder(context.Background(), &po)

		// Assertions
		require.NoError(t, err)
		assert.Equal(t, existingOrderNumber, po.OrderNumber, "Existing order number should not be changed")

		mockRepo.AssertExpectations(t)
	})

	t.Run("should return error when repository fails", func(t *testing.T) {
		unit := &models.Unit{
			Base:             models.Base{ID: 1},
			Name:             "unit",
			Symbol:           "unit",
			UnitType:         "general",
			ConversionFactor: 1,
		}

		product := &models.Product{
			Base:   models.Base{ID: 1},
			Name:   "product",
			UnitID: unit.ID,
		}

		// Setup
		mockRepo := repositorymocks.NewPurchaseOrderRepository(t)
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockUnitRepo := repositorymocks.NewUnitRepository(t)
		service := NewPurchaseOrderService(mockRepo, nil, mockUnitRepo, mockProductRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil)

		purchaseOrder := &models.PurchaseOrder{
			InventoryID: pkg.Ptr(uint(1)),
			Status:      models.PurchaseOrderStatusOrderPlaced,
			TotalAmount: decimal.NewFromFloat(1500.50),
			Notes:       "Test purchase order",
			Items: []*models.PurchaseOrderItem{
				{
					ProductID:  pkg.Ptr(product.ID),
					SupplierID: pkg.Ptr(uint(1)),
					UnitID:     pkg.Ptr(unit.ID),
					Quantity:   decimal.NewFromInt(1),
				},
			},
		}

		// Setup mock - repository returns error
		expectedError := errors.New("database connection failed")
		mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.PurchaseOrder")).Return(expectedError).Once()

		mockProductRepo.On("GetByID", mock.Anything, mock.AnythingOfType("uint")).Return(product, nil).Once()

		mockUnitRepo.On("GetByID", mock.Anything, mock.AnythingOfType("uint")).Return(unit, nil)

		// Execute
		err := service.CreatePurchaseOrder(context.Background(), purchaseOrder)

		// Assertions
		require.Error(t, err)
		assert.Equal(t, expectedError, err)
		assert.NotEmpty(t, purchaseOrder.OrderNumber, "Order number should still be generated even if repository fails")

		mockRepo.AssertExpectations(t)
	})
}

func TestUpdatePurchaseOrderStatus_WithApprovalCheck(t *testing.T) {
	t.Run("should return error when trying to complete purchase order without approved payment receipt form", func(t *testing.T) {
		// Setup
		mockRepo := repositorymocks.NewPurchaseOrderRepository(t)
		mockPaymentRepo := repositorymocks.NewPaymentReceiptFormRepository(t)
		service := NewPurchaseOrderService(mockRepo, mockPaymentRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

		// Mock payment receipt form repository to return empty slice (no approved form found)
		mockPaymentRepo.On("GetLatestPaymentReceiptForms", mock.Anything, uint(1), models.PaymentReceiptFormStatusApproved, 0).Return([]*models.PaymentReceiptForm{}, nil)

		// Execute
		err := service.UpdatePurchaseOrderStatus(context.Background(), 1, models.PurchaseOrderStatusCompleted)

		// Assert
		require.Error(t, err)
		var appErr *pkg.AppError
		assert.ErrorAs(t, err, &appErr)
		assert.Equal(t, pkg.ErrorCodePurchaseOrderNoApprovedPaymentReceipt, appErr.Code)
		// Note: UpdateStatus should NOT be called since validation fails before reaching that point
	})

	t.Run("should complete purchase order when approved payment receipt form exists", func(t *testing.T) {
		// Setup
		mockRepo := repositorymocks.NewPurchaseOrderRepository(t)
		mockPaymentRepo := repositorymocks.NewPaymentReceiptFormRepository(t)
		service := NewPurchaseOrderService(mockRepo, mockPaymentRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

		// Mock payment receipt form repository to return an approved form
		approvedForm := &models.PaymentReceiptForm{
			Base:            models.Base{ID: 1},
			PurchaseOrderID: 1,
			Status:          models.PaymentReceiptFormStatusApproved,
		}
		mockPaymentRepo.On("GetLatestPaymentReceiptForms", mock.Anything, uint(1), models.PaymentReceiptFormStatusApproved, 0).Return([]*models.PaymentReceiptForm{approvedForm}, nil)

		// Mock the repository to return success for status update
		mockRepo.On("UpdateStatus", mock.Anything, uint(1), models.PurchaseOrderStatusCompleted).Return(nil)

		// Execute
		err := service.UpdatePurchaseOrderStatus(context.Background(), 1, models.PurchaseOrderStatusCompleted)

		// Assert
		assert.NoError(t, err)
	})
}

func Test_createPOItemSellingPrices(t *testing.T) {
	t.Run("creates pisp rows for all items when none exist", func(t *testing.T) {
		ctx := context.Background()
		spRepo := repositorymocks.NewSellingPriceRepository(t)
		service := &purchaseOrderService{sellingPriceRepo: spRepo}

		invID := uint(10)
		po := &models.PurchaseOrder{
			Base:        models.Base{ID: 1},
			InventoryID: &invID,
			Items: []*models.PurchaseOrderItem{
				{Base: models.Base{ID: 100}, ProductID: pkg.Ptr(uint(1))},
				{Base: models.Base{ID: 101}, ProductID: pkg.Ptr(uint(2))},
			},
		}

		spRepo.On("GetPOItemSellingPricesByPOItemIDs", ctx, []uint{100, 101}).Return(nil, nil).Once()
		spRepo.On("GetLatestForProducts", ctx, []uint{1, 2}, &invID, mock.Anything).Return(map[uint]*models.SellingPrice{
			1: {Base: models.Base{ID: 50}, ProductID: 1, Price: decimal.NewFromInt(20)},
			2: {Base: models.Base{ID: 51}, ProductID: 2, Price: decimal.NewFromInt(30)},
		}, nil).Once()
		spRepo.On("CreatePOItemSellingPrice", ctx, mock.MatchedBy(func(p *models.POItemSellingPrice) bool {
			return p.PurchaseOrderItemID == 100 && p.SellingPriceID != nil && *p.SellingPriceID == 50 && p.SellingPrice == nil
		})).Return(nil).Once()
		spRepo.On("CreatePOItemSellingPrice", ctx, mock.MatchedBy(func(p *models.POItemSellingPrice) bool {
			return p.PurchaseOrderItemID == 101 && p.SellingPriceID != nil && *p.SellingPriceID == 51 && p.SellingPrice == nil
		})).Return(nil).Once()

		err := service.createPOItemSellingPrices(ctx, po)
		require.NoError(t, err)
	})

	t.Run("idempotent: skips items that already have a pisp row", func(t *testing.T) {
		ctx := context.Background()
		spRepo := repositorymocks.NewSellingPriceRepository(t)
		service := &purchaseOrderService{sellingPriceRepo: spRepo}

		invID := uint(10)
		po := &models.PurchaseOrder{
			Base:        models.Base{ID: 1},
			InventoryID: &invID,
			Items: []*models.PurchaseOrderItem{
				{Base: models.Base{ID: 100}, ProductID: pkg.Ptr(uint(1))},
				{Base: models.Base{ID: 101}, ProductID: pkg.Ptr(uint(2))},
			},
		}

		spRepo.On("GetPOItemSellingPricesByPOItemIDs", ctx, []uint{100, 101}).Return([]*models.POItemSellingPrice{
			{PurchaseOrderItemID: 100},
		}, nil).Once()
		spRepo.On("GetLatestForProducts", ctx, []uint{1, 2}, &invID, mock.Anything).Return(map[uint]*models.SellingPrice{
			2: {Base: models.Base{ID: 51}, ProductID: 2, Price: decimal.NewFromInt(30)},
		}, nil).Once()
		// Only item 101 should get inserted; item 100 already has a pisp row.
		spRepo.On("CreatePOItemSellingPrice", ctx, mock.MatchedBy(func(p *models.POItemSellingPrice) bool {
			return p.PurchaseOrderItemID == 101
		})).Return(nil).Once()

		err := service.createPOItemSellingPrices(ctx, po)
		require.NoError(t, err)
	})

	t.Run("inserts pisp with nil selling_price_id when no ledger price exists", func(t *testing.T) {
		ctx := context.Background()
		spRepo := repositorymocks.NewSellingPriceRepository(t)
		service := &purchaseOrderService{sellingPriceRepo: spRepo}

		invID := uint(10)
		po := &models.PurchaseOrder{
			Base:        models.Base{ID: 1},
			InventoryID: &invID,
			Items: []*models.PurchaseOrderItem{
				{Base: models.Base{ID: 100}, ProductID: pkg.Ptr(uint(1))},
			},
		}

		spRepo.On("GetPOItemSellingPricesByPOItemIDs", ctx, []uint{100}).Return(nil, nil).Once()
		spRepo.On("GetLatestForProducts", ctx, []uint{1}, &invID, mock.Anything).Return(map[uint]*models.SellingPrice{}, nil).Once()
		spRepo.On("CreatePOItemSellingPrice", ctx, mock.MatchedBy(func(p *models.POItemSellingPrice) bool {
			return p.PurchaseOrderItemID == 100 && p.SellingPriceID == nil && p.SellingPrice == nil
		})).Return(nil).Once()

		err := service.createPOItemSellingPrices(ctx, po)
		require.NoError(t, err)
	})

	t.Run("no inserts when every item already has a pisp row", func(t *testing.T) {
		ctx := context.Background()
		spRepo := repositorymocks.NewSellingPriceRepository(t)
		service := &purchaseOrderService{sellingPriceRepo: spRepo}

		invID := uint(10)
		po := &models.PurchaseOrder{
			Base:        models.Base{ID: 1},
			InventoryID: &invID,
			Items: []*models.PurchaseOrderItem{
				{Base: models.Base{ID: 100}, ProductID: pkg.Ptr(uint(1))},
			},
		}

		spRepo.On("GetPOItemSellingPricesByPOItemIDs", ctx, []uint{100}).Return([]*models.POItemSellingPrice{
			{PurchaseOrderItemID: 100},
		}, nil).Once()
		spRepo.On("GetLatestForProducts", ctx, []uint{1}, &invID, mock.Anything).Return(map[uint]*models.SellingPrice{}, nil).Once()
		// No CreatePOItemSellingPrice expected — all items already have pisp.

		err := service.createPOItemSellingPrices(ctx, po)
		require.NoError(t, err)
	})

	t.Run("returns error when sellingPriceRepo is nil", func(t *testing.T) {
		service := &purchaseOrderService{sellingPriceRepo: nil}
		po := &models.PurchaseOrder{
			Items: []*models.PurchaseOrderItem{
				{Base: models.Base{ID: 100}, ProductID: pkg.Ptr(uint(1))},
			},
		}
		err := service.createPOItemSellingPrices(context.Background(), po)
		require.Error(t, err)
	})

	t.Run("returns error when PO has no items", func(t *testing.T) {
		spRepo := repositorymocks.NewSellingPriceRepository(t)
		service := &purchaseOrderService{sellingPriceRepo: spRepo}
		err := service.createPOItemSellingPrices(context.Background(), &models.PurchaseOrder{})
		require.Error(t, err)
	})
}

func TestUpdatePurchaseOrder_EnsuresPOItemSellingPrices(t *testing.T) {
	t.Run("calls createPOItemSellingPrices after successful repo update", func(t *testing.T) {
		ctx := context.Background()
		mockPORepo := repositorymocks.NewPurchaseOrderRepository(t)
		spRepo := repositorymocks.NewSellingPriceRepository(t)
		service := NewPurchaseOrderService(mockPORepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, spRepo)

		invID := uint(10)
		// Items have UnitID nil → service skips the unit-conversion path entirely.
		req := dto.UpdatePurchaseOrderRequest{
			InventoryID: &invID,
			Items: []dto.UpdatePurchaseOrderItemRequest{
				{ProductID: pkg.Ptr(uint(1)), SupplierID: pkg.Ptr(uint(2)), Quantity: decimal.NewFromInt(5)},
			},
		}
		// Repo returns a PO with one new item whose pisp doesn't yet exist.
		updatedPO := &models.PurchaseOrder{
			Base:        models.Base{ID: 1},
			InventoryID: &invID,
			Items: []*models.PurchaseOrderItem{
				{Base: models.Base{ID: 100}, ProductID: pkg.Ptr(uint(1))},
			},
		}
		mockPORepo.On("UpdatePurchaseOrder", ctx, uint(1), req).Return(updatedPO, nil).Once()

		spRepo.On("GetPOItemSellingPricesByPOItemIDs", ctx, []uint{100}).Return(nil, nil).Once()
		spRepo.On("GetLatestForProducts", ctx, []uint{1}, &invID, mock.Anything).Return(map[uint]*models.SellingPrice{
			1: {Base: models.Base{ID: 50}, ProductID: 1, Price: decimal.NewFromInt(20)},
		}, nil).Once()
		spRepo.On("CreatePOItemSellingPrice", ctx, mock.MatchedBy(func(p *models.POItemSellingPrice) bool {
			return p.PurchaseOrderItemID == 100 && p.SellingPriceID != nil && *p.SellingPriceID == 50
		})).Return(nil).Once()

		po, err := service.UpdatePurchaseOrder(ctx, 1, req)
		require.NoError(t, err)
		require.NotNil(t, po)
	})

	t.Run("update succeeds even if createPOItemSellingPrices fails (logged, not propagated)", func(t *testing.T) {
		ctx := context.Background()
		mockPORepo := repositorymocks.NewPurchaseOrderRepository(t)
		spRepo := repositorymocks.NewSellingPriceRepository(t)
		service := NewPurchaseOrderService(mockPORepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, spRepo)

		invID := uint(10)
		req := dto.UpdatePurchaseOrderRequest{
			InventoryID: &invID,
			Items: []dto.UpdatePurchaseOrderItemRequest{
				{ProductID: pkg.Ptr(uint(1)), SupplierID: pkg.Ptr(uint(2)), Quantity: decimal.NewFromInt(5)},
			},
		}
		updatedPO := &models.PurchaseOrder{
			Base:        models.Base{ID: 1},
			InventoryID: &invID,
			Items: []*models.PurchaseOrderItem{
				{Base: models.Base{ID: 100}, ProductID: pkg.Ptr(uint(1))},
			},
		}
		mockPORepo.On("UpdatePurchaseOrder", ctx, uint(1), req).Return(updatedPO, nil).Once()
		// Existing-pisp lookup fails — UpdatePurchaseOrder must still succeed.
		spRepo.On("GetPOItemSellingPricesByPOItemIDs", ctx, []uint{100}).Return(nil, errors.New("db down")).Once()

		po, err := service.UpdatePurchaseOrder(ctx, 1, req)
		require.NoError(t, err)
		require.NotNil(t, po)
	})
}
