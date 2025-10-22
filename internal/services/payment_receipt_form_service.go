package services

import (
	"context"
	"fmt"

	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
)

type PaymentReceiptFormService interface {
	CreatePaymentReceiptForm(ctx context.Context, payload *dto.PaymentReceiptFormPayload) (*models.PaymentReceiptForm, error)
	GetPaymentReceiptForm(ctx context.Context, id uint) (*models.PaymentReceiptForm, error)
	ListPaymentReceiptForms(ctx context.Context, params models.ListParams) ([]models.PaymentReceiptForm, int64, error)
	UpdatePaymentReceiptForm(ctx context.Context, form *models.PaymentReceiptForm) error
	DeletePaymentReceiptForm(ctx context.Context, id uint) error
	SearchPaymentReceiptForms(ctx context.Context, query string, params models.ListParams) ([]models.PaymentReceiptForm, int64, error)
	GetLatestPendingPaymentReceiptForm(ctx context.Context, purchaseOrderID uint) (*models.PaymentReceiptForm, error)
}

type paymentReceiptFormService struct {
	paymentReceiptFormRepo repository.PaymentReceiptFormRepository
}

// NewPaymentReceiptFormService creates a new payment receipt form service
func NewPaymentReceiptFormService(paymentReceiptFormRepo repository.PaymentReceiptFormRepository) PaymentReceiptFormService {
	return &paymentReceiptFormService{
		paymentReceiptFormRepo: paymentReceiptFormRepo,
	}
}

// CreatePaymentReceiptForm creates a new payment receipt form
func (s *paymentReceiptFormService) CreatePaymentReceiptForm(ctx context.Context, payload *dto.PaymentReceiptFormPayload) (*models.PaymentReceiptForm, error) {
	// Convert payload to model
	payload.Status = models.PaymentReceiptFormStatusPending
	form, err := payload.ToPaymentReceiptForm()
	if err != nil {
		return nil, pkg.NewAppError(pkg.ErrorCodeValidation, "Invalid date format. Use YYYY-MM-DD", nil)
	}

	if err := s.paymentReceiptFormRepo.Create(ctx, form); err != nil {
		return nil, pkg.NewAppError(pkg.ErrorCodeInternal, "Failed to create payment receipt form", err)
	}

	return form, nil
}

// GetPaymentReceiptForm retrieves a payment receipt form by ID
func (s *paymentReceiptFormService) GetPaymentReceiptForm(ctx context.Context, id uint) (*models.PaymentReceiptForm, error) {
	form, err := s.paymentReceiptFormRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get payment receipt form: %w", err)
	}
	return form, nil
}

// ListPaymentReceiptForms retrieves a paginated list of payment receipt forms
func (s *paymentReceiptFormService) ListPaymentReceiptForms(ctx context.Context, params models.ListParams) ([]models.PaymentReceiptForm, int64, error) {
	forms, total, err := s.paymentReceiptFormRepo.List(ctx, params)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list payment receipt forms: %w", err)
	}
	return forms, total, nil
}

// UpdatePaymentReceiptForm updates a payment receipt form
func (s *paymentReceiptFormService) UpdatePaymentReceiptForm(ctx context.Context, form *models.PaymentReceiptForm) error {
	// Validate required fields
	if form.FullName == "" {
		return pkg.NewAppError(pkg.ErrorCodeValidation, "Full name is required", nil)
	}
	if form.Department == "" {
		return pkg.NewAppError(pkg.ErrorCodeValidation, "Department is required", nil)
	}
	if form.TotalAmount <= 0 {
		return pkg.NewAppError(pkg.ErrorCodeValidation, "Total amount must be greater than 0", nil)
	}

	if err := s.paymentReceiptFormRepo.Update(ctx, form); err != nil {
		return fmt.Errorf("failed to update payment receipt form: %w", err)
	}

	return nil
}

// DeletePaymentReceiptForm deletes a payment receipt form
func (s *paymentReceiptFormService) DeletePaymentReceiptForm(ctx context.Context, id uint) error {
	// Check if the form exists
	_, err := s.paymentReceiptFormRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete payment receipt form: %w", err)
	}

	if err := s.paymentReceiptFormRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete payment receipt form: %w", err)
	}

	return nil
}

// SearchPaymentReceiptForms searches payment receipt forms with pagination
func (s *paymentReceiptFormService) SearchPaymentReceiptForms(ctx context.Context, query string, params models.ListParams) ([]models.PaymentReceiptForm, int64, error) {
	forms, total, err := s.paymentReceiptFormRepo.Search(ctx, query, params)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search payment receipt forms: %w", err)
	}
	return forms, total, nil
}

// GetLatestPendingPaymentReceiptForm retrieves the latest payment receipt form in pending status
func (s *paymentReceiptFormService) GetLatestPendingPaymentReceiptForm(ctx context.Context, purchaseOrderID uint) (*models.PaymentReceiptForm, error) {
	form, err := s.paymentReceiptFormRepo.GetLatestPaymentReceiptForm(ctx, purchaseOrderID, models.PaymentReceiptFormStatusPending)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest pending payment receipt form: %w", err)
	}
	return form, nil
}
