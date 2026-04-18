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
