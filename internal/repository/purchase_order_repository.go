package repository

import (
	"context"
	"fmt"
	"import-export-backend/internal/models"
	"time"

	"gorm.io/gorm"
)

//go:generate mockery --name=PurchaseOrderRepository --structname=PurchaseOrderRepository --output=./repositorymocks --outpkg=repositorymocks
type PurchaseOrderRepository interface {
	Create(ctx context.Context, purchaseOrder *models.PurchaseOrder) error
	GetByID(id uint) (*models.PurchaseOrder, error)
	Update(ctx context.Context, purchaseOrder *models.PurchaseOrder) error
	Delete(id uint) error
	List(ctx context.Context, params models.ListParams) ([]models.PurchaseOrder, int64, error)
	GetByStatus(status string) ([]models.PurchaseOrder, error)
	UpdateStatus(ctx context.Context, purchaseOrderID uint, status models.PurchaseOrderStatus) error
	UpdatePurchaseOrderItemStatus(ctx context.Context, purchaseOrderID, itemID uint, status models.PurchaseOrderItemStatus) error
	AnyDeliveringItem(ctx context.Context, purchaseOrderID uint) bool
}

type purchaseOrderRepository struct {
	db *gorm.DB
}

func NewPurchaseOrderRepository(db *gorm.DB) PurchaseOrderRepository {
	return &purchaseOrderRepository{db: db}
}

func (r *purchaseOrderRepository) Create(ctx context.Context, purchaseOrder *models.PurchaseOrder) error {
	return r.db.WithContext(ctx).Create(purchaseOrder).Error
}

func (r *purchaseOrderRepository) GetByID(id uint) (*models.PurchaseOrder, error) {
	var purchaseOrder models.PurchaseOrder
	err := r.db.Preload("Items").Preload("Items.Product").First(&purchaseOrder, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &purchaseOrder, nil
}

func (r *purchaseOrderRepository) Update(ctx context.Context, purchaseOrder *models.PurchaseOrder) error {
	return r.db.WithContext(ctx).Save(purchaseOrder).Error
}

func (r *purchaseOrderRepository) Delete(id uint) error {
	return r.db.Delete(&models.PurchaseOrder{}, "id = ?", id).Error
}

func (r *purchaseOrderRepository) List(ctx context.Context, params models.ListParams) ([]models.PurchaseOrder, int64, error) {
	var purchaseOrders []models.PurchaseOrder
	var total int64

	// Build base query for both count and data
	baseQuery := r.db.WithContext(ctx).Model(&models.PurchaseOrder{})

	// Apply search filter
	if params.Search != "" {
		baseQuery = baseQuery.Where("order_number ILIKE ? OR notes ILIKE ?", "%"+params.Search+"%", "%"+params.Search+"%")
	}

	// Apply date range filter
	if params.StartDate != "" || params.EndDate != "" {
		if params.StartDate != "" {
			startTime, err := time.Parse("2006-01-02", params.StartDate)
			if err != nil {
				return nil, 0, fmt.Errorf("invalid start_date format, expected YYYY-MM-DD: %w", err)
			}
			baseQuery = baseQuery.Where("created_at >= ?", startTime)
		}
		if params.EndDate != "" {
			endTime, err := time.Parse("2006-01-02", params.EndDate)
			if err != nil {
				return nil, 0, fmt.Errorf("invalid end_date format, expected YYYY-MM-DD: %w", err)
			}
			// Add one day to include the entire end date
			endTime = endTime.Add(24 * time.Hour)
			baseQuery = baseQuery.Where("created_at < ?", endTime)
		}
	}

	// Apply status filter
	if params.Status != "" {
		baseQuery = baseQuery.Where("status = ?", params.Status)
	}

	// Build query for data with preloads
	baseQuery = baseQuery.Preload("Items.Product.Supplier")

	// Apply same search filter for data
	if params.Search != "" {
		baseQuery = baseQuery.Where("order_number ILIKE ? OR notes ILIKE ?", "%"+params.Search+"%", "%"+params.Search+"%")
	}

	// Get total count first
	err := baseQuery.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// Apply sorting
	if params.Sort != "" {
		orderDirection := "ASC"
		if params.Order == "desc" {
			orderDirection = "DESC"
		}

		switch params.Sort {
		case "order_number", "status", "total_amount", "created_at", "updated_at":
			baseQuery = baseQuery.Order(params.Sort + " " + orderDirection)
		default:
			baseQuery = baseQuery.Order("created_at DESC") // Default sorting
		}
	} else {
		baseQuery = baseQuery.Order("created_at DESC") // Default sorting
	}

	err = baseQuery.Limit(params.Limit).Offset(params.GetOffset()).
		Find(&purchaseOrders).Error
	if err != nil {
		return nil, 0, fmt.Errorf("failed to fetch purchase orders: %w", err)
	}
	return purchaseOrders, total, err
}

func (r *purchaseOrderRepository) GetByStatus(status string) ([]models.PurchaseOrder, error) {
	var purchaseOrders []models.PurchaseOrder
	err := r.db.Preload("Items").Preload("Items.Product").Where("status = ?", status).Find(&purchaseOrders).Error
	return purchaseOrders, err
}

func (r *purchaseOrderRepository) UpdateStatus(ctx context.Context, purchaseOrderID uint, status models.PurchaseOrderStatus) error {
	return r.db.WithContext(ctx).Model(&models.PurchaseOrder{}).Where("id = ?", purchaseOrderID).Update("status", status).Error
}

// UpdatePurchaseOrderItemStatus updates the status of a specific purchase order item
func (r *purchaseOrderRepository) UpdatePurchaseOrderItemStatus(ctx context.Context, purchaseOrderID, itemID uint, status models.PurchaseOrderItemStatus) error {
	// First verify that the item belongs to the purchase order
	var item models.PurchaseOrderItem
	err := r.db.WithContext(ctx).Where("id = ? AND purchase_order_id = ?", itemID, purchaseOrderID).First(&item).Error
	if err != nil {
		return fmt.Errorf("failed to find purchase order item: %w", err)
	}

	// Update the status
	return r.db.WithContext(ctx).Model(&item).Update("status", status).Error
}

// AnyDeliveringItem checks if all items in a purchase order are delivered
func (r *purchaseOrderRepository) AnyDeliveringItem(ctx context.Context, purchaseOrderID uint) bool {
	// Get count of delivered items
	err := r.db.WithContext(ctx).Model(&models.PurchaseOrderItem{}).
		Where("purchase_order_id = ? AND status = ?", purchaseOrderID, models.PurchaseOrderItemStatusDelivering).
		First(&models.PurchaseOrderItem{}).Error
	return err == nil
}
