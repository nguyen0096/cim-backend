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
	ListPaymentReceiptForms(ctx context.Context, req *dto.PaymentReceiptFormListRequest) ([]models.PaymentReceiptForm, int64, error)
	SubmitPaymentReceiptForm(ctx context.Context, form *models.PaymentReceiptForm) error
	ApprovePaymentReceiptForm(ctx context.Context, id uint) error
	RejectPaymentReceiptForm(ctx context.Context, id uint) error
	DeletePaymentReceiptForm(ctx context.Context, id uint) error
	SearchPaymentReceiptForms(ctx context.Context, query string, req *dto.PaymentReceiptFormListRequest) ([]models.PaymentReceiptForm, int64, error)
	LatestPendingPaymentReceiptFormStream(ctx context.Context, purchaseOrderID uint) (*models.PaymentReceiptForm, error)
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
func (s *paymentReceiptFormService) ListPaymentReceiptForms(ctx context.Context, req *dto.PaymentReceiptFormListRequest) ([]models.PaymentReceiptForm, int64, error) {
	forms, total, err := s.paymentReceiptFormRepo.List(ctx, req)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list payment receipt forms: %w", err)
	}
	return forms, total, nil
}

// SubmitPaymentReceiptForm updates a payment receipt form
func (s *paymentReceiptFormService) SubmitPaymentReceiptForm(ctx context.Context, form *models.PaymentReceiptForm) error {
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

	form.Status = models.PaymentReceiptFormStatusSubmitted

	if err := s.paymentReceiptFormRepo.Update(ctx, form); err != nil {
		return fmt.Errorf("failed to update payment receipt form: %w", err)
	}

	return nil
}

// ApprovePaymentReceiptForm approves a payment receipt form
func (s *paymentReceiptFormService) ApprovePaymentReceiptForm(ctx context.Context, id uint) error {
	// Get the form to check if it exists and current status
	form, err := s.paymentReceiptFormRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to approve payment receipt form: %w", err)
	}

	// Validate that the form can be approved
	if form.Status == models.PaymentReceiptFormStatusApproved {
		return pkg.NewAppError(pkg.ErrorCodeValidation, "Payment receipt form is already approved", nil)
	}
	if form.Status == models.PaymentReceiptFormStatusRejected {
		return pkg.NewAppError(pkg.ErrorCodeValidation, "Cannot approve a rejected payment receipt form", nil)
	}

	// Update status to approved
	if err := s.paymentReceiptFormRepo.UpdateStatus(ctx, id, models.PaymentReceiptFormStatusApproved); err != nil {
		return fmt.Errorf("failed to approve payment receipt form: %w", err)
	}

	return nil
}

// RejectPaymentReceiptForm rejects a payment receipt form
func (s *paymentReceiptFormService) RejectPaymentReceiptForm(ctx context.Context, id uint) error {
	// Get the form to check if it exists and current status
	form, err := s.paymentReceiptFormRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to reject payment receipt form: %w", err)
	}

	// Validate that the form can be rejected
	if form.Status == models.PaymentReceiptFormStatusRejected {
		return pkg.NewAppError(pkg.ErrorCodeValidation, "Payment receipt form is already rejected", nil)
	}
	if form.Status == models.PaymentReceiptFormStatusApproved {
		return pkg.NewAppError(pkg.ErrorCodeValidation, "Cannot reject an approved payment receipt form", nil)
	}

	// Update status to rejected
	if err := s.paymentReceiptFormRepo.UpdateStatus(ctx, id, models.PaymentReceiptFormStatusRejected); err != nil {
		return fmt.Errorf("failed to reject payment receipt form: %w", err)
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
func (s *paymentReceiptFormService) SearchPaymentReceiptForms(ctx context.Context, query string, req *dto.PaymentReceiptFormListRequest) ([]models.PaymentReceiptForm, int64, error) {
	forms, total, err := s.paymentReceiptFormRepo.Search(ctx, query, req)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search payment receipt forms: %w", err)
	}
	return forms, total, nil
}

// LatestPendingPaymentReceiptFormStream retrieves the latest payment receipt form in pending status
func (s *paymentReceiptFormService) LatestPendingPaymentReceiptFormStream(ctx context.Context, purchaseOrderID uint) (*models.PaymentReceiptForm, error) {
	form, err := s.paymentReceiptFormRepo.GetLatestPaymentReceiptForm(ctx, purchaseOrderID, models.PaymentReceiptFormStatusPending)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest pending payment receipt form: %w", err)
	}
	return form, nil
}
