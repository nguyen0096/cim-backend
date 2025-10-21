package services

import (
	"context"
	"testing"

	"cim-backend/internal/models"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockPaymentReceiptFormRepository is a mock implementation of PaymentReceiptFormRepository
type MockPaymentReceiptFormRepository struct {
	mock.Mock
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

func (m *MockPaymentReceiptFormRepository) List(ctx context.Context, params models.ListParams) ([]models.PaymentReceiptForm, int64, error) {
	args := m.Called(ctx, params)
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

func (m *MockPaymentReceiptFormRepository) Search(ctx context.Context, query string, params models.ListParams) ([]models.PaymentReceiptForm, int64, error) {
	args := m.Called(ctx, query, params)
	return args.Get(0).([]models.PaymentReceiptForm), args.Get(1).(int64), args.Error(2)
}

func (m *MockPaymentReceiptFormRepository) GetLatestPendingForm(ctx context.Context) (*models.PaymentReceiptForm, error) {
	args := m.Called(ctx)
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
				FullName:       "John Doe",
				Date:           "2024-01-15",
				Department:     "Finance",
				PaymentDetails: "Office supplies",
				TotalAmount:    100.50,
				AmountInWords:  "One hundred and fifty cents",
			},
			expectError: false,
		},
		{
			name: "should return validation error when full name is empty",
			payload: &dto.PaymentReceiptFormPayload{
				FullName:       "",
				Date:           "2024-01-15",
				Department:     "Finance",
				PaymentDetails: "Office supplies",
				TotalAmount:    100.50,
				AmountInWords:  "One hundred and fifty cents",
			},
			expectError: true,
			errorCode:   pkg.ErrorCodeValidation,
		},
		{
			name: "should return validation error when department is empty",
			payload: &dto.PaymentReceiptFormPayload{
				FullName:       "John Doe",
				Date:           "2024-01-15",
				Department:     "",
				PaymentDetails: "Office supplies",
				TotalAmount:    100.50,
				AmountInWords:  "One hundred and fifty cents",
			},
			expectError: true,
			errorCode:   pkg.ErrorCodeValidation,
		},
		{
			name: "should return validation error when date format is invalid",
			payload: &dto.PaymentReceiptFormPayload{
				FullName:       "John Doe",
				Date:           "invalid-date",
				Department:     "Finance",
				PaymentDetails: "Office supplies",
				TotalAmount:    100.50,
				AmountInWords:  "One hundred and fifty cents",
			},
			expectError: true,
			errorCode:   pkg.ErrorCodeValidation,
		},
		{
			name: "should return validation error when total amount is zero or negative",
			payload: &dto.PaymentReceiptFormPayload{
				FullName:       "John Doe",
				Date:           "2024-01-15",
				Department:     "Finance",
				PaymentDetails: "Office supplies",
				TotalAmount:    0,
				AmountInWords:  "Zero",
			},
			expectError: true,
			errorCode:   pkg.ErrorCodeValidation,
		},
		{
			name: "should return validation error when there is already a pending form",
			payload: &dto.PaymentReceiptFormPayload{
				FullName:       "John Doe",
				Date:           "2024-01-15",
				Department:     "Finance",
				PaymentDetails: "Office supplies",
				TotalAmount:    100.50,
				AmountInWords:  "One hundred and fifty cents",
			},
			expectError: true,
			errorCode:   pkg.ErrorCodeValidation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mockRepo := new(MockPaymentReceiptFormRepository)
			service := NewPaymentReceiptFormService(mockRepo)

			// Mock GetLatestPendingForm call
			if tt.name == "should return validation error when there is already a pending form" {
				// When there's a pending form, return a mock form
				mockRepo.On("GetLatestPendingForm", mock.Anything).Return(&models.PaymentReceiptForm{
					FullName: "Existing User",
					Status:   models.PaymentReceiptFormStatusPending,
				}, nil)
			} else {
				// When there's no pending form, return nil
				mockRepo.On("GetLatestPendingForm", mock.Anything).Return(nil, nil)
			}

			if !tt.expectError {
				mockRepo.On("Create", mock.Anything, mock.MatchedBy(func(form *models.PaymentReceiptForm) bool {
					return form.FullName == tt.payload.FullName &&
						form.Department == tt.payload.Department &&
						form.Details == tt.payload.PaymentDetails &&
						form.TotalAmount == tt.payload.TotalAmount
				})).Return(nil)
			}

			// Act
			result, err := service.CreatePaymentReceiptForm(context.Background(), tt.payload)

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
				assert.Equal(t, tt.payload.PaymentDetails, result.Details)
				assert.Equal(t, tt.payload.TotalAmount, result.TotalAmount)
				mockRepo.AssertExpectations(t)
			}
		})
	}
}

func TestPaymentReceiptFormService_GetLatestPendingPaymentReceiptForm(t *testing.T) {
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
			service := NewPaymentReceiptFormService(mockRepo)

			if tt.expectError {
				mockRepo.On("GetLatestPendingForm", mock.Anything).Return(nil, pkg.NewAppError(tt.errorCode, "No pending payment receipt form found", nil))
			} else {
				mockRepo.On("GetLatestPendingForm", mock.Anything).Return(tt.mockForm, nil)
			}

			// Act
			result, err := service.GetLatestPendingPaymentReceiptForm(context.Background())

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
	service := NewPaymentReceiptFormService(mockRepo)

	expectedForm := &models.PaymentReceiptForm{
		FullName:    "John Doe",
		Department:  "Finance",
		Location:    "Office",
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
	service := NewPaymentReceiptFormService(mockRepo)

	expectedForms := []models.PaymentReceiptForm{
		{
			FullName:    "John Doe",
			Department:  "Finance",
			Location:    "Office",
			TotalAmount: 100.50,
			Status:      models.PaymentReceiptFormStatusPending,
		},
	}
	expectedTotal := int64(1)
	params := models.ListParams{Page: 1, Limit: 20}

	mockRepo.On("List", mock.Anything, params).Return(expectedForms, expectedTotal, nil)

	// Act
	forms, total, err := service.ListPaymentReceiptForms(context.Background(), params)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, expectedForms, forms)
	assert.Equal(t, expectedTotal, total)
	mockRepo.AssertExpectations(t)
}
