package repository

import (
	"context"
	"fmt"

	"cim-backend/internal/models"
	"cim-backend/pkg"

	"gorm.io/gorm"
)

type PaymentReceiptFormRepository interface {
	Create(ctx context.Context, form *models.PaymentReceiptForm) error
	GetByID(ctx context.Context, id uint) (*models.PaymentReceiptForm, error)
	List(ctx context.Context, params models.ListParams) ([]models.PaymentReceiptForm, int64, error)
	Update(ctx context.Context, form *models.PaymentReceiptForm) error
	Delete(ctx context.Context, id uint) error
	Search(ctx context.Context, query string, params models.ListParams) ([]models.PaymentReceiptForm, int64, error)
	GetLatestPendingForm(ctx context.Context) (*models.PaymentReceiptForm, error)
}

type paymentReceiptFormRepository struct {
	db *gorm.DB
}

// NewPaymentReceiptFormRepository creates a new payment receipt form repository
func NewPaymentReceiptFormRepository(db *gorm.DB) PaymentReceiptFormRepository {
	return &paymentReceiptFormRepository{
		db: db,
	}
}

// Create creates a new payment receipt form
func (r *paymentReceiptFormRepository) Create(ctx context.Context, form *models.PaymentReceiptForm) error {
	if err := r.db.WithContext(ctx).Create(form).Error; err != nil {
		return pkg.NewAppError(pkg.ErrorCodeInternal, "Failed to create payment receipt form", err)
	}
	return nil
}

// GetByID retrieves a payment receipt form by ID
func (r *paymentReceiptFormRepository) GetByID(ctx context.Context, id uint) (*models.PaymentReceiptForm, error) {
	var form models.PaymentReceiptForm
	if err := r.db.WithContext(ctx).First(&form, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, pkg.NewAppError(pkg.ErrorCodeNotFound, "Payment receipt form not found", nil)
		}
		return nil, fmt.Errorf("failed to get payment receipt form: %w", err)
	}
	return &form, nil
}

// List retrieves a paginated list of payment receipt forms
func (r *paymentReceiptFormRepository) List(ctx context.Context, params models.ListParams) ([]models.PaymentReceiptForm, int64, error) {
	var forms []models.PaymentReceiptForm
	var total int64

	query := r.db.WithContext(ctx).Model(&models.PaymentReceiptForm{})

	// Count total records
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count payment receipt forms: %w", err)
	}

	// Apply pagination and sorting
	offset := params.GetOffset()
	orderBy := fmt.Sprintf("%s %s", params.Sort, params.Order)

	if err := query.Order(orderBy).Offset(offset).Limit(params.Limit).Find(&forms).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list payment receipt forms: %w", err)
	}

	return forms, total, nil
}

// Update updates a payment receipt form
func (r *paymentReceiptFormRepository) Update(ctx context.Context, form *models.PaymentReceiptForm) error {
	if err := r.db.WithContext(ctx).Save(form).Error; err != nil {
		return fmt.Errorf("failed to update payment receipt form: %w", err)
	}
	return nil
}

// Delete soft deletes a payment receipt form
func (r *paymentReceiptFormRepository) Delete(ctx context.Context, id uint) error {
	if err := r.db.WithContext(ctx).Delete(&models.PaymentReceiptForm{}, id).Error; err != nil {
		return fmt.Errorf("failed to delete payment receipt form: %w", err)
	}
	return nil
}

// Search searches payment receipt forms with pagination
func (r *paymentReceiptFormRepository) Search(ctx context.Context, query string, params models.ListParams) ([]models.PaymentReceiptForm, int64, error) {
	var forms []models.PaymentReceiptForm
	var total int64

	searchQuery := r.db.WithContext(ctx).Model(&models.PaymentReceiptForm{}).
		Where("full_name ILIKE ? OR department ILIKE ? OR details ILIKE ? OR location ILIKE ?",
			"%"+query+"%", "%"+query+"%", "%"+query+"%", "%"+query+"%")

	// Count total records
	if err := searchQuery.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count search results: %w", err)
	}

	// Apply pagination and sorting
	offset := params.GetOffset()
	orderBy := fmt.Sprintf("%s %s", params.Sort, params.Order)

	if err := searchQuery.Order(orderBy).Offset(offset).Limit(params.Limit).Find(&forms).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to search payment receipt forms: %w", err)
	}

	return forms, total, nil
}

// GetLatestPendingForm retrieves the latest payment receipt form in pending status
func (r *paymentReceiptFormRepository) GetLatestPendingForm(ctx context.Context) (*models.PaymentReceiptForm, error) {
	var form models.PaymentReceiptForm
	if err := r.db.WithContext(ctx).
		Where("status = ?", models.PaymentReceiptFormStatusPending).
		Order("created_at DESC").
		First(&form).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get latest pending payment receipt form: %w", err)
	}
	return &form, nil
}
