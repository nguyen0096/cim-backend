package services

import (
	"cim-backend/internal/mocks/repositorymocks"
	"cim-backend/internal/models"
	"cim-backend/internal/services/dto"
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func Test_generatePurchaseOrderNumber(t *testing.T) {
	// Setup
	mockRepo := repositorymocks.NewPurchaseOrderRepository(t)
	// Note: Service mocks are not used in this test as it's not needed for generatePurchaseOrderNumber
	service := NewPurchaseOrderService(mockRepo, nil, nil, nil, nil, nil, nil, nil, nil).(*purchaseOrderService)

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
		service := NewPurchaseOrderService(mockRepo, nil, nil, nil, nil, nil, nil, nil, nil)

		purchaseOrder := &models.PurchaseOrder{
			// OrderNumber is empty - should be auto-generated
			Status:      models.PurchaseOrderStatusOrderPlaced,
			TotalAmount: decimal.NewFromFloat(1500.50),
			Notes:       "Test purchase order",
			Items: []*models.PurchaseOrderItem{
				{
					ProductID:   &[]uint{1}[0],
					SupplierID:  &[]uint{1}[0],
					UnitID:      &[]uint{1}[0],
					Quantity:    decimal.NewFromInt(5),
					TotalAmount: decimal.NewFromFloat(502.50),
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
		service := NewPurchaseOrderService(mockRepo, nil, nil, nil, nil, nil, nil, nil, nil)

		existingOrderNumber := "CUSTOM-PO-123"
		purchaseOrder := &models.PurchaseOrder{
			OrderNumber: existingOrderNumber,
			Status:      models.PurchaseOrderStatusOrderPlaced,
			TotalAmount: decimal.NewFromFloat(1500.50),
			Notes:       "Test purchase order with existing order number",
			Items: []*models.PurchaseOrderItem{
				{
					ProductID:   &[]uint{1}[0],
					SupplierID:  &[]uint{1}[0],
					UnitID:      &[]uint{1}[0],
					Quantity:    decimal.NewFromInt(5),
					TotalAmount: decimal.NewFromFloat(502.50),
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
		service := NewPurchaseOrderService(mockRepo, nil, nil, nil, nil, nil, nil, nil, nil)

		purchaseOrder := &models.PurchaseOrder{
			Status:      models.PurchaseOrderStatusOrderPlaced,
			TotalAmount: decimal.NewFromFloat(1500.50),
			Items: []*models.PurchaseOrderItem{
				{
					ProductID:   &[]uint{1}[0],
					SupplierID:  &[]uint{1}[0],
					UnitID:      &[]uint{1}[0],
					Quantity:    decimal.NewFromInt(5),
					TotalAmount: decimal.NewFromFloat(502.50),
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

func TestUpdatePurchaseOrderStatus_WithApprovalCheck(t *testing.T) {
	t.Run("should return error when trying to complete purchase order without approved payment receipt form", func(t *testing.T) {
		// Setup
		mockRepo := repositorymocks.NewPurchaseOrderRepository(t)
		mockPaymentRepo := &mockPaymentReceiptFormRepository{}
		service := NewPurchaseOrderService(mockRepo, mockPaymentRepo, nil, nil, nil, nil, nil, nil, nil)

		// Mock the repository to return success for status update
		mockRepo.On("UpdateStatus", mock.Anything, uint(1), models.PurchaseOrderStatusCompleted).Return(nil)

		// Mock payment receipt form repository to return nil (no approved form found)
		mockPaymentRepo.approvedForm = nil
		mockPaymentRepo.err = nil

		// Execute
		err := service.UpdatePurchaseOrderStatus(context.Background(), 1, models.PurchaseOrderStatusCompleted)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no approved payment receipt form found")
		mockRepo.AssertExpectations(t)
	})

	t.Run("should complete purchase order when approved payment receipt form exists", func(t *testing.T) {
		// Setup
		mockRepo := repositorymocks.NewPurchaseOrderRepository(t)
		mockPaymentRepo := &mockPaymentReceiptFormRepository{}
		service := NewPurchaseOrderService(mockRepo, mockPaymentRepo, nil, nil, nil, nil, nil, nil, nil)

		// Mock the repository to return success for status update
		mockRepo.On("UpdateStatus", mock.Anything, uint(1), models.PurchaseOrderStatusCompleted).Return(nil)

		// Mock payment receipt form repository to return an approved form
		mockPaymentRepo.approvedForm = &models.PaymentReceiptForm{
			Base:            models.Base{ID: 1},
			PurchaseOrderID: 1,
			Status:          models.PaymentReceiptFormStatusApproved,
		}
		mockPaymentRepo.err = nil

		// Execute
		err := service.UpdatePurchaseOrderStatus(context.Background(), 1, models.PurchaseOrderStatusCompleted)

		// Assert
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})
}

// mockPaymentReceiptFormRepository is a simple mock for testing
type mockPaymentReceiptFormRepository struct {
	approvedForm *models.PaymentReceiptForm
	err          error
}

func (m *mockPaymentReceiptFormRepository) DeletePermanently(ctx context.Context, id uint) error {
	return nil
}
func (m *mockPaymentReceiptFormRepository) GetByIDFull(ctx context.Context, id uint) (*models.PaymentReceiptForm, error) {
	return nil, nil
}
func (m *mockPaymentReceiptFormRepository) GetLatestPaymentReceiptForms(ctx context.Context, purchaseOrderID uint, status models.PaymentReceiptFormStatus, limit int) ([]*models.PaymentReceiptForm, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.approvedForm == nil {
		return []*models.PaymentReceiptForm{}, nil
	}
	return []*models.PaymentReceiptForm{m.approvedForm}, nil
}

// Implement other required methods as no-ops for this test
func (m *mockPaymentReceiptFormRepository) Create(ctx context.Context, form *models.PaymentReceiptForm) error {
	return nil
}
func (m *mockPaymentReceiptFormRepository) GetByID(ctx context.Context, id uint) (*models.PaymentReceiptForm, error) {
	return nil, nil
}
func (m *mockPaymentReceiptFormRepository) List(ctx context.Context, req *dto.PaymentReceiptFormListRequest) ([]models.PaymentReceiptForm, int64, error) {
	return nil, 0, nil
}
func (m *mockPaymentReceiptFormRepository) Update(ctx context.Context, form *models.PaymentReceiptForm) error {
	return nil
}
func (m *mockPaymentReceiptFormRepository) UpdateStatus(ctx context.Context, id uint, status models.PaymentReceiptFormStatus) error {
	return nil
}
func (m *mockPaymentReceiptFormRepository) Delete(ctx context.Context, id uint) error { return nil }
func (m *mockPaymentReceiptFormRepository) Search(ctx context.Context, query string, req *dto.PaymentReceiptFormListRequest) ([]models.PaymentReceiptForm, int64, error) {
	return nil, 0, nil
}
