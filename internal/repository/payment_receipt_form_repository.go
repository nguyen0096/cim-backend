package repository

import (
	"context"
	"fmt"
	"time"

	"cim-backend/internal/models"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"

	"gorm.io/gorm"
)

//go:generate mockery --name=PaymentReceiptFormRepository --structname=PaymentReceiptFormRepository --output=../mocks/repositorymocks --outpkg=repositorymocks

type PaymentReceiptFormRepository interface {
	Create(ctx context.Context, form *models.PaymentReceiptForm) error
	GetByID(ctx context.Context, id uint) (*models.PaymentReceiptForm, error)
	GetByIDFull(ctx context.Context, id uint) (*models.PaymentReceiptForm, error)
	GetByIDsFull(ctx context.Context, ids []uint) ([]*models.PaymentReceiptForm, error)
	List(ctx context.Context, req *dto.PaymentReceiptFormListRequest, preloads ...string) ([]models.PaymentReceiptForm, int64, error)
	Update(ctx context.Context, form *models.PaymentReceiptForm) error
	Delete(ctx context.Context, id uint) error
	DeletePermanently(ctx context.Context, id uint) error
	GetLatestPaymentReceiptForms(ctx context.Context, purchaseOrderID uint, status models.PaymentReceiptFormStatus, limit int) ([]*models.PaymentReceiptForm, error)
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
	if err := r.db.WithContext(ctx).Model(&models.PaymentReceiptForm{}).Omit("PurchaseOrder").Create(form).Error; err != nil {
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

func (r *paymentReceiptFormRepository) GetByIDFull(ctx context.Context, id uint) (*models.PaymentReceiptForm, error) {
	var form models.PaymentReceiptForm
	if err := r.db.WithContext(ctx).
		Preload("PurchaseOrder").
		Preload("PurchaseOrder.Inventory").
		Preload("PurchaseOrder.Items").
		Preload("PurchaseOrder.Items.Product").
		Preload("PurchaseOrder.Items.Supplier").
		First(&form, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, pkg.NewAppError(pkg.ErrorCodeNotFound, "Payment receipt form not found", nil)
		}
		return nil, fmt.Errorf("failed to get payment receipt form: %w", err)
	}
	return &form, nil
}

// GetByIDsFull retrieves multiple payment receipt forms by IDs with full relationships
func (r *paymentReceiptFormRepository) GetByIDsFull(ctx context.Context, ids []uint) ([]*models.PaymentReceiptForm, error) {
	if len(ids) == 0 {
		return []*models.PaymentReceiptForm{}, nil
	}

	var forms []*models.PaymentReceiptForm
	if err := r.db.WithContext(ctx).
		Preload("PurchaseOrder").
		Preload("PurchaseOrder.Inventory").
		Preload("PurchaseOrder.Items").
		Preload("PurchaseOrder.Items.Product").
		Preload("PurchaseOrder.Items.Supplier").
		Where("id IN ?", ids).
		Find(&forms).Error; err != nil {
		return nil, fmt.Errorf("failed to get payment receipt forms: %w", err)
	}
	return forms, nil
}

// List retrieves a paginated list of payment receipt forms
func (r *paymentReceiptFormRepository) List(ctx context.Context, req *dto.PaymentReceiptFormListRequest, preloads ...string) ([]models.PaymentReceiptForm, int64, error) {
	var forms []models.PaymentReceiptForm
	var total int64

	query := r.db.WithContext(ctx).Model(&models.PaymentReceiptForm{}).Preload("PurchaseOrder").Preload("PurchaseOrder.Inventory")
	for _, preload := range preloads {
		query = query.Preload(preload)
	}
	// Apply purchase order filter if provided
	if req.PurchaseOrderID != 0 {
		query = query.Where("payment_receipt_forms.purchase_order_id = ?", req.PurchaseOrderID)
	}

	// Apply inventory filter if provided
	if req.InventoryID != 0 {
		query = query.Joins("JOIN purchase_orders ON payment_receipt_forms.purchase_order_id = purchase_orders.id").
			Where("purchase_orders.inventory_id = ?", req.InventoryID)
	}

	// Apply status filter if provided
	if len(req.Statuses) > 0 {
		query = query.Where("payment_receipt_forms.status IN ?", req.Statuses)
	}

	// Apply search filter if provided
	if req.Search != "" {
		searchPattern := "%" + req.Search + "%"
		query = query.Where("payment_receipt_forms.full_name ILIKE ? OR payment_receipt_forms.department ILIKE ? OR payment_receipt_forms.details ILIKE ?",
			searchPattern, searchPattern, searchPattern)
	}

	// Apply finalized date filter if provided
	if !req.FinalizedDate.IsZero() {
		finalizedDate := req.FinalizedDate.Truncate(24 * time.Hour).Format("20060102")
		query = query.Where("payment_receipt_forms.form_number LIKE ?", finalizedDate+"-%")
	}

	// Apply date filter if provided
	if req.Date != "" {
		date, err := time.Parse("2006-01-02", req.Date)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid date format, expected YYYY-MM-DD: %w", err)
		}
		startOfDay := date.Truncate(24 * time.Hour)
		endOfDay := startOfDay.Add(24 * time.Hour)
		query = query.Where("payment_receipt_forms.date >= ? AND payment_receipt_forms.date < ?", startOfDay, endOfDay)
	}

	// Count total records (after all filters are applied)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count payment receipt forms: %w", err)
	}

	// Apply pagination and sorting
	offset := req.GetOffset()
	if req.Sort != "" {
		if req.Order == "" {
			req.Order = "asc"
		}
		// Special handling for form_number to sort by numeric parts
		// Form number format: YYYYMMDD-inventoryID-increment
		if req.Sort == "form_number" {
			// Sort by date part, then inventoryID, then increment (all as integers)
			query = query.Order("CAST(SPLIT_PART(payment_receipt_forms.form_number, '-', 1) AS INTEGER) " + req.Order).
				Order("CAST(SPLIT_PART(payment_receipt_forms.form_number, '-', 2) AS INTEGER) " + req.Order).
				Order("CAST(SPLIT_PART(payment_receipt_forms.form_number, '-', 3) AS INTEGER) " + req.Order)
		} else {
			query = query.Order("payment_receipt_forms." + req.Sort + " " + req.Order)
		}
	} else {
		query = query.Order("payment_receipt_forms.created_at desc")
	}

	if err := query.Offset(offset).Limit(req.Limit).Find(&forms).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list payment receipt forms: %w", err)
	}

	return forms, total, nil
}

// Update updates a payment receipt form
func (r *paymentReceiptFormRepository) Update(ctx context.Context, form *models.PaymentReceiptForm) error {
	if err := r.db.WithContext(ctx).
		Model(form).
		Select("full_name", "date", "department", "details", "total_amount", "status", "form_number").
		Where("id = ?", form.ID).
		Updates(form).Error; err != nil {
		return pkg.NewAppError(pkg.ErrorCodeInternal, "Failed to update payment receipt form", err)
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

// Delete permanently deletes a payment receipt form from the database
func (r *paymentReceiptFormRepository) DeletePermanently(ctx context.Context, id uint) error {
	if err := r.db.WithContext(ctx).Unscoped().Delete(&models.PaymentReceiptForm{}, id).Error; err != nil {
		return fmt.Errorf("failed to delete payment receipt form: %w", err)
	}
	return nil
}

// GetLatestPaymentReceiptForms retrieves the latest payment receipt forms in the provided status
func (r *paymentReceiptFormRepository) GetLatestPaymentReceiptForms(ctx context.Context, purchaseOrderID uint, status models.PaymentReceiptFormStatus, limit int) ([]*models.PaymentReceiptForm, error) {
	var forms []*models.PaymentReceiptForm
	query := r.db.WithContext(ctx).
		Preload("PurchaseOrder").
		Preload("PurchaseOrder.Inventory").
		Omit("PurchaseOrder.TotalAmount").
		Where("status = ?", status).
		Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if purchaseOrderID != 0 {
		query = query.Where("purchase_order_id = ?", purchaseOrderID)
	}

	if err := query.Find(&forms).Error; err != nil {
		return nil, fmt.Errorf("failed to get latest payment receipt forms: %w", err)
	}
	return forms, nil
}
