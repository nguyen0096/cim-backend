package services

import (
	"context"
	"testing"
	"time"

	"cim-backend/internal/mocks/repositorymocks"
	"cim-backend/internal/models"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

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
				FormNumber:    pkg.Ptr("20240115-001"),
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
				FormNumber:    pkg.Ptr("20240115-002"),
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
				FormNumber:    pkg.Ptr("20240115-003"),
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
				FormNumber:    pkg.Ptr("20240115-004"),
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
				FormNumber:    pkg.Ptr("20240115-005"),
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
				FormNumber:    pkg.Ptr("20240115-006"),
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
				FormNumber:    pkg.Ptr("20240115-001"), // Provide form number to avoid auto-generation
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
			mockRepo := repositorymocks.NewPaymentReceiptFormRepository(t)
			service := NewPaymentReceiptFormService(mockRepo, nil, nil, nil)

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
			if err != nil {
				// Handle conversion errors
				if tt.expectError {
					assert.Error(t, err)
					if appErr, ok := err.(*pkg.AppError); ok {
						assert.Equal(t, tt.errorCode, appErr.Code)
					}
					return
				}
				require.NoError(t, err)
			}

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
			}
		})
	}
}

func TestPaymentReceiptFormService_GetLatestPaymentReceiptForms(t *testing.T) {
	tests := []struct {
		name           string
		expectedForms  []*models.PaymentReceiptForm
		expectedLength int
		expectError    bool
	}{
		{
			name: "should get latest pending payment receipt form successfully",
			expectedForms: []*models.PaymentReceiptForm{
				{
					FullName:    "John Doe",
					Department:  "Finance",
					Details:     "Office supplies",
					TotalAmount: 100.50,
					Status:      models.PaymentReceiptFormStatusPending,
				},
			},
			expectedLength: 1,
		},
		{
			name:           "should return empty slice when no pending form exists",
			expectedForms:  []*models.PaymentReceiptForm{},
			expectedLength: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := repositorymocks.NewPaymentReceiptFormRepository(t)
			service := NewPaymentReceiptFormService(mockRepo, nil, nil, nil)

			mockRepo.On("GetLatestPaymentReceiptForms", mock.Anything, uint(0), models.PaymentReceiptFormStatusPending, 5).Return(tt.expectedForms, nil)

			result, err := service.GetLatestPaymentReceiptForms(context.Background(), 0, models.PaymentReceiptFormStatusPending, 5)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
				return
			}

			assert.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.expectedLength, len(result))
		})
	}
}

func TestPaymentReceiptFormService_GetPaymentReceiptForm(t *testing.T) {
	// Arrange
	mockRepo := repositorymocks.NewPaymentReceiptFormRepository(t)
	service := NewPaymentReceiptFormService(mockRepo, nil, nil, nil)

	expectedForm := &models.PaymentReceiptForm{
		FullName:    "John Doe",
		Department:  "Finance",
		TotalAmount: 100.50,
		Status:      models.PaymentReceiptFormStatusPending,
	}

	mockRepo.On("GetByIDFull", mock.Anything, uint(1)).Return(expectedForm, nil)

	// Act
	form, err := service.GetPaymentReceiptForm(context.Background(), 1)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, expectedForm, form)
}

func TestPaymentReceiptFormService_ApprovePaymentReceiptForm(t *testing.T) {
	ctx := context.Background()
	inventoryID := uint(1)
	formID := uint(123)

	setup := func(t *testing.T) (*repositorymocks.PaymentReceiptFormRepository, *repositorymocks.RevenueExpenseFinalizationRepository, PaymentReceiptFormService) {
		mockRepo := repositorymocks.NewPaymentReceiptFormRepository(t)
		mockFinalizationRepo := repositorymocks.NewRevenueExpenseFinalizationRepository(t)
		service := NewPaymentReceiptFormService(mockRepo, nil, nil, mockFinalizationRepo)
		return mockRepo, mockFinalizationRepo, service
	}

	t.Run("should approve form successfully", func(t *testing.T) {
		mockRepo, mockFinalizationRepo, service := setup(t)
		form := &models.PaymentReceiptForm{
			Base:   models.Base{ID: formID},
			Status: models.PaymentReceiptFormStatusPending,
			Date:   time.Now(),
			PurchaseOrder: &models.PurchaseOrder{
				InventoryID: &inventoryID,
			},
		}

		mockRepo.On("GetByIDFull", ctx, formID).Return(form, nil).Once()
		mockFinalizationRepo.On("GetLastSuccessful", ctx).Return(nil, nil).Once()
		mockRepo.On("GenerateNextFormNumber", ctx, form.Date, inventoryID).Return("20240115-1-1", nil).Once()
		mockRepo.On("Update", ctx, mock.MatchedBy(func(f *models.PaymentReceiptForm) bool {
			return f.Status == models.PaymentReceiptFormStatusApproved && *f.FormNumber == "20240115-1-1"
		})).Return(nil).Once()

		err := service.ApprovePaymentReceiptForm(ctx, formID)
		assert.NoError(t, err)
	})

	t.Run("should use last finalized date if available", func(t *testing.T) {
		mockRepo, mockFinalizationRepo, service := setup(t)
		finalizedDate := time.Now().AddDate(0, 0, -1)
		form := &models.PaymentReceiptForm{
			Base:   models.Base{ID: formID},
			Status: models.PaymentReceiptFormStatusPending,
			PurchaseOrder: &models.PurchaseOrder{
				InventoryID: &inventoryID,
			},
		}

		mockRepo.On("GetByIDFull", ctx, formID).Return(form, nil).Once()
		mockFinalizationRepo.On("GetLastSuccessful", ctx).Return(&models.RevenueExpenseFinalization{
			FinalizedDate: finalizedDate,
		}, nil).Once()
		mockRepo.On("GenerateNextFormNumber", ctx, finalizedDate, inventoryID).Return("20240114-1-1", nil).Once()
		mockRepo.On("Update", ctx, mock.Anything).Return(nil).Once()

		err := service.ApprovePaymentReceiptForm(ctx, formID)
		assert.NoError(t, err)
	})

	t.Run("should fail if already approved", func(t *testing.T) {
		mockRepo, _, service := setup(t)
		form := &models.PaymentReceiptForm{
			Base:   models.Base{ID: formID},
			Status: models.PaymentReceiptFormStatusApproved,
		}

		mockRepo.On("GetByIDFull", ctx, formID).Return(form, nil).Once()

		err := service.ApprovePaymentReceiptForm(ctx, formID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already approved")
	})

	t.Run("should fail if rejected", func(t *testing.T) {
		mockRepo, _, service := setup(t)
		form := &models.PaymentReceiptForm{
			Base:   models.Base{ID: formID},
			Status: models.PaymentReceiptFormStatusRejected,
		}

		mockRepo.On("GetByIDFull", ctx, formID).Return(form, nil).Once()

		err := service.ApprovePaymentReceiptForm(ctx, formID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "rejected")
	})

	t.Run("should retry on duplicate key error", func(t *testing.T) {
		mockRepo, mockFinalizationRepo, service := setup(t)
		form := &models.PaymentReceiptForm{
			Base:   models.Base{ID: formID},
			Status: models.PaymentReceiptFormStatusPending,
			Date:   time.Now(),
			PurchaseOrder: &models.PurchaseOrder{
				InventoryID: &inventoryID,
			},
		}

		mockRepo.On("GetByIDFull", ctx, formID).Return(form, nil).Once()
		mockFinalizationRepo.On("GetLastSuccessful", ctx).Return(nil, nil).Once()

		// First attempt fails with duplicate key
		mockRepo.On("GenerateNextFormNumber", ctx, form.Date, inventoryID).Return("20240115-1-1", nil).Once()
		dupErr := pkg.NewAppError(pkg.ErrorCodeInternal, "duplicate key value violates unique constraint", nil)
		mockRepo.On("Update", ctx, mock.Anything).Return(dupErr).Once()

		// Second attempt succeeds
		mockRepo.On("GenerateNextFormNumber", ctx, form.Date, inventoryID).Return("20240115-1-2", nil).Once()
		mockRepo.On("Update", ctx, mock.MatchedBy(func(f *models.PaymentReceiptForm) bool {
			return *f.FormNumber == "20240115-1-2"
		})).Return(nil).Once()

		err := service.ApprovePaymentReceiptForm(ctx, formID)
		assert.NoError(t, err)

		mockRepo.AssertNumberOfCalls(t, "GetByIDFull", 1)
		mockRepo.AssertNumberOfCalls(t, "GenerateNextFormNumber", 2)
		mockRepo.AssertNumberOfCalls(t, "Update", 2)
	})
}
