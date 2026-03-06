package services

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"

	"gorm.io/gorm"
)

type PaymentReceiptFormService interface {
	CreatePaymentReceiptForm(ctx context.Context, form *models.PaymentReceiptForm) (*models.PaymentReceiptForm, error)
	GetPaymentReceiptForm(ctx context.Context, id uint) (*models.PaymentReceiptForm, error)
	ListPaymentReceiptForms(ctx context.Context, req *dto.PaymentReceiptFormListRequest) ([]models.PaymentReceiptForm, int64, error)
	SubmitPaymentReceiptForm(ctx context.Context, form *models.PaymentReceiptForm) error
	UpdatePaymentReceiptForm(ctx context.Context, form *models.PaymentReceiptForm) error
	ApprovePaymentReceiptForm(ctx context.Context, id uint) error
	RejectPaymentReceiptForm(ctx context.Context, id uint) error
	DeletePaymentReceiptForm(ctx context.Context, id uint) error
	GetLatestPaymentReceiptForms(ctx context.Context, purchaseOrderID uint, status models.PaymentReceiptFormStatus, limit int) ([]*models.PaymentReceiptForm, error)
}

type paymentReceiptFormService struct {
	paymentReceiptFormRepo         repository.PaymentReceiptFormRepository
	db                             *gorm.DB
	settingsService                SettingsService
	revenueExpenseFinalizationRepo repository.RevenueExpenseFinalizationRepository
}

// NewPaymentReceiptFormService creates a new payment receipt form service
func NewPaymentReceiptFormService(
	paymentReceiptFormRepo repository.PaymentReceiptFormRepository,
	db *gorm.DB,
	settingsService SettingsService,
	revenueExpenseFinalizationRepo repository.RevenueExpenseFinalizationRepository,
) PaymentReceiptFormService {
	return &paymentReceiptFormService{
		paymentReceiptFormRepo:         paymentReceiptFormRepo,
		db:                             db,
		settingsService:                settingsService,
		revenueExpenseFinalizationRepo: revenueExpenseFinalizationRepo,
	}
}

// CreatePaymentReceiptForm creates a new payment receipt form
func (s *paymentReceiptFormService) CreatePaymentReceiptForm(ctx context.Context, form *models.PaymentReceiptForm) (*models.PaymentReceiptForm, error) {
	// Convert payload to model
	form.Status = models.PaymentReceiptFormStatusPending

	if err := s.paymentReceiptFormRepo.Create(ctx, form); err != nil {
		return nil, pkg.NewAppError(pkg.ErrorCodeInternal, "Failed to create payment receipt form", err)
	}

	return form, nil
}

// GetPaymentReceiptForm retrieves a payment receipt form by ID
func (s *paymentReceiptFormService) GetPaymentReceiptForm(ctx context.Context, id uint) (*models.PaymentReceiptForm, error) {
	form, err := s.paymentReceiptFormRepo.GetByIDFull(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get payment receipt form: %w", err)
	}
	return form, nil
}

// ListPaymentReceiptForms retrieves a paginated list of payment receipt forms
func (s *paymentReceiptFormService) ListPaymentReceiptForms(ctx context.Context, req *dto.PaymentReceiptFormListRequest) ([]models.PaymentReceiptForm, int64, error) {
	preloads := make([]string, 0, 5)
	if req.FinalizedDate != "" {
		preloads = append(preloads, "PurchaseOrder.Items", "PurchaseOrder.Items.Supplier", "PurchaseOrder.Items.Product")
	}
	forms, total, err := s.paymentReceiptFormRepo.List(ctx, req, preloads...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list payment receipt forms: %w", err)
	}
	return forms, total, nil
}

// validatePaymentReceiptFormFields validates required fields of a payment receipt form
func (s *paymentReceiptFormService) validatePaymentReceiptFormFields(form *models.PaymentReceiptForm) error {
	if form.FullName == "" {
		return pkg.NewAppError(pkg.ErrorCodeValidation, "Full name is required", nil)
	}
	if form.Department == "" {
		return pkg.NewAppError(pkg.ErrorCodeValidation, "Department is required", nil)
	}
	if form.TotalAmount < 0 {
		return pkg.NewAppError(pkg.ErrorCodeValidation, "Total amount must be greater than or equal to 0", nil)
	}
	return nil
}

// SubmitPaymentReceiptForm updates a payment receipt form (used by bot_form role)
func (s *paymentReceiptFormService) SubmitPaymentReceiptForm(ctx context.Context, form *models.PaymentReceiptForm) error {
	// Validate required fields
	if err := s.validatePaymentReceiptFormFields(form); err != nil {
		return err
	}

	form.Status = models.PaymentReceiptFormStatusSubmitted

	// Update the form
	if err := s.paymentReceiptFormRepo.Update(ctx, form); err != nil {
		return fmt.Errorf("failed to update payment receipt form: %w", err)
	}

	return nil
}

// UpdatePaymentReceiptForm updates a payment receipt form (used by admin/accountant)
func (s *paymentReceiptFormService) UpdatePaymentReceiptForm(ctx context.Context, form *models.PaymentReceiptForm) error {
	// Get the existing form to get inventory ID from purchase order
	existingForm, err := s.paymentReceiptFormRepo.GetByIDFull(ctx, form.ID)
	if err != nil {
		return fmt.Errorf("failed to get payment receipt form: %w", err)
	}

	// Only allow editing forms in pending status
	if existingForm.Status != models.PaymentReceiptFormStatusPending {
		return pkg.NewAppError(pkg.ErrorCodeValidation, fmt.Sprintf("Cannot edit a payment receipt form with status '%s'", existingForm.Status), nil)
	}

	// Validate required fields
	if err := s.validatePaymentReceiptFormFields(form); err != nil {
		return err
	}

	// Update the form
	if err := s.paymentReceiptFormRepo.Update(ctx, form); err != nil {
		return fmt.Errorf("failed to update payment receipt form: %w", err)
	}

	return nil
}

// isDuplicateKeyError checks if an error is a PostgreSQL duplicate key violation
func (s *paymentReceiptFormService) isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}

	// Check the error message (which includes AppError message and cause)
	errStr := strings.ToLower(err.Error())
	if strings.Contains(errStr, "duplicate key") || strings.Contains(errStr, "sqlstate 23505") {
		return true
	}

	// Also check if it's an AppError and unwrap to check the cause
	var appErr *pkg.AppError
	if errors.As(err, &appErr) && appErr.Cause != nil {
		causeStr := strings.ToLower(appErr.Cause.Error())
		if strings.Contains(causeStr, "duplicate key") || strings.Contains(causeStr, "sqlstate 23505") {
			return true
		}
	}

	return false
}

// ApprovePaymentReceiptForm approves a payment receipt form
func (s *paymentReceiptFormService) ApprovePaymentReceiptForm(ctx context.Context, id uint) error {
	// Get the form to check if it exists and current status
	form, err := s.paymentReceiptFormRepo.GetByIDFull(ctx, id)
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

	form.Status = models.PaymentReceiptFormStatusApproved

	// Get last successful finalized date from table
	var finalizedDate time.Time
	lastFinalization, err := s.revenueExpenseFinalizationRepo.GetLastSuccessful(ctx)
	if err != nil {
		return fmt.Errorf("failed to get last successful finalized date: %w", err)
	}
	if lastFinalization != nil {
		finalizedDate = lastFinalization.FinalizedDate
	}

	// If no finalized date is found, use form.Date as fallback
	if finalizedDate.IsZero() {
		finalizedDate = form.Date
	}

	// Retry logic to handle concurrent form number generation
	maxRetries := 5
	for attempt := 0; attempt < maxRetries; attempt++ {
		formNumber, err := s.paymentReceiptFormRepo.GenerateNextFormNumber(ctx, finalizedDate, *form.PurchaseOrder.InventoryID)
		if err != nil {
			return fmt.Errorf("failed to generate form number: %w", err)
		}
		form.FormNumber = &formNumber

		// Update the form
		err = s.paymentReceiptFormRepo.Update(ctx, form)
		if err == nil {
			// Success, no need to retry
			return nil
		}

		// Check if it's a duplicate key error
		if s.isDuplicateKeyError(err) {
			// If this is the last attempt, return the error
			if attempt == maxRetries-1 {
				return fmt.Errorf("failed to update payment receipt form after %d retries due to duplicate key conflicts: %w", maxRetries, err)
			}
			// Exponential backoff with jitter to reduce thundering herd
			baseDelay := time.Millisecond * time.Duration(50*(1<<uint(attempt))) // Exponential: 50ms, 100ms, 200ms, 400ms...
			jitter := time.Millisecond * time.Duration(rand.Intn(50))            // Random jitter up to 50ms
			time.Sleep(baseDelay + jitter)
			continue
		}

		// If it's not a duplicate key error, return immediately
		return fmt.Errorf("failed to update payment receipt form: %w", err)
	}

	return nil
}

// RejectPaymentReceiptForm rejects a payment receipt form
func (s *paymentReceiptFormService) RejectPaymentReceiptForm(ctx context.Context, id uint) error {
	// Delete the form from database
	if err := s.paymentReceiptFormRepo.DeletePermanently(ctx, id); err != nil {
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

	if err := s.paymentReceiptFormRepo.DeletePermanently(ctx, id); err != nil {
		return fmt.Errorf("failed to delete payment receipt form: %w", err)
	}

	return nil
}

// GetLatestPaymentReceiptForms retrieves the latest payment receipt forms in pending status
func (s *paymentReceiptFormService) GetLatestPaymentReceiptForms(ctx context.Context, purchaseOrderID uint, status models.PaymentReceiptFormStatus, limit int) ([]*models.PaymentReceiptForm, error) {
	forms, err := s.paymentReceiptFormRepo.GetLatestPaymentReceiptForms(ctx, purchaseOrderID, status, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest pending payment receipt forms: %w", err)
	}
	return forms, nil
}
