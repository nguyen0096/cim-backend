package services

import (
	"context"
	"fmt"
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
	SearchPaymentReceiptForms(ctx context.Context, query string, req *dto.PaymentReceiptFormListRequest) ([]models.PaymentReceiptForm, int64, error)
	LatestPendingPaymentReceiptFormStream(ctx context.Context, purchaseOrderID uint) (*models.PaymentReceiptForm, error)
}

type paymentReceiptFormService struct {
	paymentReceiptFormRepo repository.PaymentReceiptFormRepository
	db                     *gorm.DB
}

// NewPaymentReceiptFormService creates a new payment receipt form service
func NewPaymentReceiptFormService(paymentReceiptFormRepo repository.PaymentReceiptFormRepository, db *gorm.DB) PaymentReceiptFormService {
	return &paymentReceiptFormService{
		paymentReceiptFormRepo: paymentReceiptFormRepo,
		db:                     db,
	}
}

func (s *paymentReceiptFormService) buildFormNumber(date time.Time, inventoryID uint, number int64) string {
	return fmt.Sprintf("%s-%d-%d", date.Format("20060102"), inventoryID, number)
}

// generateNextFormNumber generates the next available form number in date-increment format
func (s *paymentReceiptFormService) generateNextFormNumber(ctx context.Context, date time.Time, inventoryID uint) (string, error) {
	dateString := date.Format("20060102")

	// Use a transaction with retry logic to handle race conditions
	var formNumber string
	maxRetries := 3

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Count all existing forms (including soft-deleted) to get the next increment number
		var count int64
		err := s.db.WithContext(ctx).
			Unscoped().
			Model(&models.PaymentReceiptForm{}).
			Where("form_number LIKE ?", fmt.Sprintf("%s-%d-%%", dateString, inventoryID)).
			Count(&count).Error

		if err != nil {
			return "", fmt.Errorf("failed to count existing forms: %w", err)
		}

		// Generate form number with next increment
		increment := count + 1
		formNumber = s.buildFormNumber(date, inventoryID, increment)

		// Verify this form number doesn't exist
		var existingCount int64
		err = s.db.WithContext(ctx).
			Unscoped().
			Model(&models.PaymentReceiptForm{}).
			Where("form_number = ?", formNumber).
			Count(&existingCount).Error

		if err != nil {
			return "", fmt.Errorf("failed to verify form number uniqueness: %w", err)
		}

		// If form number is unique, return it
		if existingCount == 0 {
			return formNumber, nil
		}

		// If we reach here, there's a race condition, retry
		if attempt == maxRetries-1 {
			return "", fmt.Errorf("failed to generate unique form number after %d attempts", maxRetries)
		}

		// Small delay before retry
		time.Sleep(time.Millisecond * 10 * time.Duration(attempt+1))
	}

	return formNumber, nil
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

// prepareFormForSubmission prepares a form for submission by generating form number and setting status
func (s *paymentReceiptFormService) prepareFormForSubmission(ctx context.Context, form *models.PaymentReceiptForm, inventoryID uint) error {
	// Only generate a new form number if one doesn't already exist
	if form.FormNumber == nil || *form.FormNumber == "" {
		formNumber, err := s.generateNextFormNumber(ctx, form.Date, inventoryID)
		if err != nil {
			return fmt.Errorf("failed to generate form number: %w", err)
		}
		form.FormNumber = &formNumber
	}

	form.Status = models.PaymentReceiptFormStatusSubmitted
	return nil
}

// SubmitPaymentReceiptForm updates a payment receipt form (used by bot_form role)
func (s *paymentReceiptFormService) SubmitPaymentReceiptForm(ctx context.Context, form *models.PaymentReceiptForm) error {
	// Validate required fields
	if err := s.validatePaymentReceiptFormFields(form); err != nil {
		return err
	}

	// Prepare form for submission
	if err := s.prepareFormForSubmission(ctx, form, *form.PurchaseOrder.InventoryID); err != nil {
		return err
	}

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

	// Get inventory ID from existing form's purchase order
	inventoryID := *existingForm.PurchaseOrder.InventoryID

	// Prepare form for submission (still needs approval)
	if err := s.prepareFormForSubmission(ctx, form, inventoryID); err != nil {
		return err
	}

	// Update the form
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
