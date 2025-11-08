package services

import (
	"context"
	"testing"

	"cim-backend/internal/models"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func strPtr(s string) *string {
	return &s
}

// MockPaymentReceiptFormRepository is a mock implementation of PaymentReceiptFormRepository
type MockPaymentReceiptFormRepository struct {
	mock.Mock
}

// DeletePermanently implements repository.PaymentReceiptFormRepository.
func (m *MockPaymentReceiptFormRepository) DeletePermanently(ctx context.Context, id uint) error {
	panic("unimplemented")
}

// GetByIDFull implements repository.PaymentReceiptFormRepository.
func (m *MockPaymentReceiptFormRepository) GetByIDFull(ctx context.Context, id uint) (*models.PaymentReceiptForm, error) {
	panic("unimplemented")
}

func (m *MockPaymentReceiptFormRepository) Create(ctx context.Context, form *models.PaymentReceiptForm) error {
	args := m.Called(ctx, form)
	return args.Error(0)
}

func (m *MockPaymentReceiptFormRepository) GetByID(ctx context.Context, id uint) (*models.PaymentReceiptForm, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PaymentReceiptForm), args.Error(1)
}

func (m *MockPaymentReceiptFormRepository) List(ctx context.Context, req *dto.PaymentReceiptFormListRequest) ([]models.PaymentReceiptForm, int64, error) {
	args := m.Called(ctx, req)
	return args.Get(0).([]models.PaymentReceiptForm), args.Get(1).(int64), args.Error(2)
}

func (m *MockPaymentReceiptFormRepository) Update(ctx context.Context, form *models.PaymentReceiptForm) error {
	args := m.Called(ctx, form)
	return args.Error(0)
}

func (m *MockPaymentReceiptFormRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockPaymentReceiptFormRepository) Search(ctx context.Context, query string, req *dto.PaymentReceiptFormListRequest) ([]models.PaymentReceiptForm, int64, error) {
	args := m.Called(ctx, query, req)
	return args.Get(0).([]models.PaymentReceiptForm), args.Get(1).(int64), args.Error(2)
}

func (m *MockPaymentReceiptFormRepository) UpdateStatus(ctx context.Context, id uint, status models.PaymentReceiptFormStatus) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

func (m *MockPaymentReceiptFormRepository) GetLatestPaymentReceiptForm(ctx context.Context, purchaseOrderID uint, status models.PaymentReceiptFormStatus) (*models.PaymentReceiptForm, error) {
	args := m.Called(ctx, purchaseOrderID, status)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PaymentReceiptForm), args.Error(1)
}

func TestPaymentReceiptFormService_CreatePaymentReceiptForm(t *testing.T) {
	tests := []struct {
		name        string
		payload     *dto.PaymentReceiptFormPayload
		expectError bool
		errorCode   pkg.ErrorCode
	}{
		{
			name: "should create payment receipt form successfully when all required fields are provided and no pending form exists",
			payload: &dto.PaymentReceiptFormPayload{
				FormNumber:    strPtr("20240115-001"),
				FullName:      "John Doe",
				Date:          "2024-01-15",
				Department:    "Finance",
				Details:       "Office supplies",
				TotalAmount:   100.50,
				AmountInWords: "One hundred and fifty cents",
			},
			expectError: false,
		},
		{
			name: "should create payment receipt form even when full name is empty (validation happens during submission)",
			payload: &dto.PaymentReceiptFormPayload{
				FormNumber:    strPtr("20240115-002"),
				FullName:      "",
				Date:          "2024-01-15",
				Department:    "Finance",
				Details:       "Office supplies",
				TotalAmount:   100.50,
				AmountInWords: "One hundred and fifty cents",
			},
			expectError: false,
		},
		{
			name: "should create payment receipt form even when department is empty (validation happens during submission)",
			payload: &dto.PaymentReceiptFormPayload{
				FormNumber:    strPtr("20240115-003"),
				FullName:      "John Doe",
				Date:          "2024-01-15",
				Department:    "",
				Details:       "Office supplies",
				TotalAmount:   100.50,
				AmountInWords: "One hundred and fifty cents",
			},
			expectError: false,
		},
		{
			name: "should return validation error when date format is invalid",
			payload: &dto.PaymentReceiptFormPayload{
				FormNumber:    strPtr("20240115-004"),
				FullName:      "John Doe",
				Date:          "invalid-date",
				Department:    "Finance",
				Details:       "Office supplies",
				TotalAmount:   100.50,
				AmountInWords: "One hundred and fifty cents",
			},
			expectError: true,
			errorCode:   pkg.ErrorCodeValidation,
		},
		{
			name: "should create payment receipt form even when total amount is zero (validation happens during submission)",
			payload: &dto.PaymentReceiptFormPayload{
				FormNumber:    strPtr("20240115-005"),
				FullName:      "John Doe",
				Date:          "2024-01-15",
				Department:    "Finance",
				Details:       "Office supplies",
				TotalAmount:   0,
				AmountInWords: "Zero",
			},
			expectError: false,
		},
		{
			name: "should create payment receipt form successfully (no pending form validation in create)",
			payload: &dto.PaymentReceiptFormPayload{
				FormNumber:    strPtr("20240115-006"),
				FullName:      "John Doe",
				Date:          "2024-01-15",
				Department:    "Finance",
				Details:       "Office supplies",
				TotalAmount:   100.50,
				AmountInWords: "One hundred and fifty cents",
			},
			expectError: false,
		},
		{
			name: "should create payment receipt form with provided form number",
			payload: &dto.PaymentReceiptFormPayload{
				FormNumber:    strPtr("20240115-001"), // Provide form number to avoid auto-generation
				FullName:      "John Doe",
				Date:          "2024-01-15",
				Department:    "Finance",
				Details:       "Office supplies",
				TotalAmount:   100.50,
				AmountInWords: "One hundred and fifty cents",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mockRepo := new(MockPaymentReceiptFormRepository)
			service := NewPaymentReceiptFormService(mockRepo, nil)

			if !tt.expectError {
				mockRepo.On("Create", mock.Anything, mock.MatchedBy(func(form *models.PaymentReceiptForm) bool {
					return form.FullName == tt.payload.FullName &&
						form.Department == tt.payload.Department &&
						form.Details == tt.payload.Details &&
						form.TotalAmount == tt.payload.TotalAmount
				})).Return(nil)
			}

			// Act
			form, err := tt.payload.ToPaymentReceiptForm()
			require.NoError(t, err)
			result, err := service.CreatePaymentReceiptForm(context.Background(), form)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
				if appErr, ok := err.(*pkg.AppError); ok {
					assert.Equal(t, tt.errorCode, appErr.Code)
				}
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.payload.FullName, result.FullName)
				assert.Equal(t, tt.payload.Department, result.Department)
				assert.Equal(t, tt.payload.Details, result.Details)
				assert.Equal(t, tt.payload.TotalAmount, result.TotalAmount)
				mockRepo.AssertExpectations(t)
			}
		})
	}
}

func TestPaymentReceiptFormService_LatestPendingPaymentReceiptFormStream(t *testing.T) {
	tests := []struct {
		name        string
		expectError bool
		errorCode   pkg.ErrorCode
		mockForm    *models.PaymentReceiptForm
	}{
		{
			name:        "should get latest pending payment receipt form successfully",
			expectError: false,
			mockForm: &models.PaymentReceiptForm{
				FullName:    "John Doe",
				Department:  "Finance",
				Details:     "Office supplies",
				TotalAmount: 100.50,
				Status:      models.PaymentReceiptFormStatusPending,
			},
		},
		{
			name:        "should return not found error when no pending form exists",
			expectError: true,
			errorCode:   pkg.ErrorCodeNotFound,
			mockForm:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mockRepo := new(MockPaymentReceiptFormRepository)
			service := NewPaymentReceiptFormService(mockRepo, nil)

			if tt.expectError {
				mockRepo.On("GetLatestPaymentReceiptForm", mock.Anything, uint(0), models.PaymentReceiptFormStatusPending).Return(nil, pkg.NewAppError(tt.errorCode, "No pending payment receipt form found", nil))
			} else {
				mockRepo.On("GetLatestPaymentReceiptForm", mock.Anything, uint(0), models.PaymentReceiptFormStatusPending).Return(tt.mockForm, nil)
			}

			// Act
			result, err := service.LatestPendingPaymentReceiptFormStream(context.Background(), 0)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
				if appErr, ok := err.(*pkg.AppError); ok {
					assert.Equal(t, tt.errorCode, appErr.Code)
				}
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.mockForm.FullName, result.FullName)
				assert.Equal(t, tt.mockForm.Status, result.Status)
				mockRepo.AssertExpectations(t)
			}
		})
	}
}

func TestPaymentReceiptFormService_GetPaymentReceiptForm(t *testing.T) {
	// Arrange
	mockRepo := new(MockPaymentReceiptFormRepository)
	service := NewPaymentReceiptFormService(mockRepo, nil)

	expectedForm := &models.PaymentReceiptForm{
		FullName:    "John Doe",
		Department:  "Finance",
		TotalAmount: 100.50,
		Status:      models.PaymentReceiptFormStatusPending,
	}

	mockRepo.On("GetByID", mock.Anything, uint(1)).Return(expectedForm, nil)

	// Act
	form, err := service.GetPaymentReceiptForm(context.Background(), 1)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, expectedForm, form)
	mockRepo.AssertExpectations(t)
}

func TestPaymentReceiptFormService_ListPaymentReceiptForms(t *testing.T) {
	// Arrange
	mockRepo := new(MockPaymentReceiptFormRepository)
	service := NewPaymentReceiptFormService(mockRepo, nil)

	expectedForms := []models.PaymentReceiptForm{
		{
			FullName:    "John Doe",
			Department:  "Finance",
			TotalAmount: 100.50,
			Status:      models.PaymentReceiptFormStatusPending,
		},
	}
	expectedTotal := int64(1)
	params := &dto.PaymentReceiptFormListRequest{
		ListParams: models.ListParams{Page: 1, Limit: 20},
	}

	mockRepo.On("List", mock.Anything, params).Return(expectedForms, expectedTotal, nil)

	// Act
	forms, total, err := service.ListPaymentReceiptForms(context.Background(), params)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, expectedForms, forms)
	assert.Equal(t, expectedTotal, total)
	mockRepo.AssertExpectations(t)
}
