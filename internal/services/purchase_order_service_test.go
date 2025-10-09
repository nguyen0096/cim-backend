package services

import (
	"context"
	"errors"
	"import-export-backend/internal/mocks/repositorymocks"
	"import-export-backend/internal/models"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func Test_generatePurchaseOrderNumber(t *testing.T) {
	// Setup
	mockRepo := repositorymocks.NewPurchaseOrderRepository(t)
	// Note: Service mocks are not used in this test as it's not needed for generatePurchaseOrderNumber
	service := NewPurchaseOrderService(mockRepo, nil, nil, nil, nil, nil).(*purchaseOrderService)

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
		// Note: Service mocks are not used in these tests as they're not needed for the tested functionality
		service := NewPurchaseOrderService(mockRepo, nil, nil, nil, nil, nil)

		purchaseOrder := &models.PurchaseOrder{
			// OrderNumber is empty - should be auto-generated
			Status:      models.PurchaseOrderStatusOrderPlaced,
			TotalAmount: 1500.50,
			Notes:       "Test purchase order",
			Items: []*models.PurchaseOrderItem{
				{
					ProductID:  &[]uint{1}[0],
					Quantity:   5,
					TotalPrice: 502.50,
				},
			},
		}

		// Setup mock - expect Create to be called with purchase order that has generated order number
		mockRepo.On("Create", mock.Anything, mock.MatchedBy(func(po *models.PurchaseOrder) bool {
			// Check that order number was generated and follows the expected pattern
			pattern := `^PO-\d{6}-\d{6}-[A-Z0-9]{2}$`
			matched, _ := regexp.MatchString(pattern, po.OrderNumber)
			return matched && po.OrderNumber != ""
		})).Return(nil)

		// Execute
		err := service.CreatePurchaseOrder(context.Background(), purchaseOrder)

		// Assertions
		require.NoError(t, err)
		assert.NotEmpty(t, purchaseOrder.OrderNumber, "Order number should be generated")

		// Verify the generated order number format
		pattern := `^PO-\d{6}-\d{6}-[A-Z0-9]{2}$`
		matched, err := regexp.MatchString(pattern, purchaseOrder.OrderNumber)
		require.NoError(t, err)
		assert.True(t, matched, "Generated order number should match expected format")

		mockRepo.AssertExpectations(t)
	})

	t.Run("should create purchase order without changing provided order number", func(t *testing.T) {
		// Setup
		mockRepo := repositorymocks.NewPurchaseOrderRepository(t)
		// Note: Service mocks are not used in these tests as they're not needed for the tested functionality
		service := NewPurchaseOrderService(mockRepo, nil, nil, nil, nil, nil)

		existingOrderNumber := "CUSTOM-PO-123"
		purchaseOrder := &models.PurchaseOrder{
			OrderNumber: existingOrderNumber,
			Status:      models.PurchaseOrderStatusOrderPlaced,
			TotalAmount: 1500.50,
			Notes:       "Test purchase order with existing order number",
			Items: []*models.PurchaseOrderItem{
				{
					ProductID:  &[]uint{1}[0],
					Quantity:   5,
					TotalPrice: 502.50,
				},
			},
		}

		// Setup mock
		mockRepo.On("Create", mock.Anything, mock.MatchedBy(func(po *models.PurchaseOrder) bool {
			return po.OrderNumber == existingOrderNumber
		})).Return(nil)

		// Execute
		err := service.CreatePurchaseOrder(context.Background(), purchaseOrder)

		// Assertions
		require.NoError(t, err)
		assert.Equal(t, existingOrderNumber, purchaseOrder.OrderNumber, "Existing order number should not be changed")

		mockRepo.AssertExpectations(t)
	})

	t.Run("should return error when repository fails", func(t *testing.T) {
		// Setup
		mockRepo := repositorymocks.NewPurchaseOrderRepository(t)
		// Note: Service mocks are not used in these tests as they're not needed for the tested functionality
		service := NewPurchaseOrderService(mockRepo, nil, nil, nil, nil, nil)

		purchaseOrder := &models.PurchaseOrder{
			Status:      models.PurchaseOrderStatusOrderPlaced,
			TotalAmount: 1500.50,
			Items: []*models.PurchaseOrderItem{
				{
					ProductID:  &[]uint{1}[0],
					Quantity:   5,
					TotalPrice: 502.50,
				},
			},
		}

		// Setup mock - repository returns error
		expectedError := errors.New("database connection failed")
		mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.PurchaseOrder")).Return(expectedError)

		// Execute
		err := service.CreatePurchaseOrder(context.Background(), purchaseOrder)

		// Assertions
		require.Error(t, err)
		assert.Equal(t, expectedError, err)
		assert.NotEmpty(t, purchaseOrder.OrderNumber, "Order number should still be generated even if repository fails")

		mockRepo.AssertExpectations(t)
	})
}
