package repository

import (
	"context"
	"import-export-backend/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

//go:generate mockery --name=PurchaseOrderRepository --structname=PurchaseOrderRepository --output=./repositorymocks --outpkg=repositorymocks
type PurchaseOrderRepository interface {
	Create(ctx context.Context, purchaseOrder *models.PurchaseOrder) error
	GetByID(id uuid.UUID) (*models.PurchaseOrder, error)
	Update(purchaseOrder *models.PurchaseOrder) error
	Delete(id uuid.UUID) error
	List(ctx context.Context, params models.PaginationParams) ([]models.PurchaseOrder, int64, error)
	GetByStatus(status string) ([]models.PurchaseOrder, error)
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

func (r *purchaseOrderRepository) GetByID(id uuid.UUID) (*models.PurchaseOrder, error) {
	var purchaseOrder models.PurchaseOrder
	err := r.db.Preload("Items").Preload("Items.Product").First(&purchaseOrder, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &purchaseOrder, nil
}

func (r *purchaseOrderRepository) Update(purchaseOrder *models.PurchaseOrder) error {
	return r.db.Save(purchaseOrder).Error
}

func (r *purchaseOrderRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&models.PurchaseOrder{}, "id = ?", id).Error
}

func (r *purchaseOrderRepository) List(ctx context.Context, params models.PaginationParams) ([]models.PurchaseOrder, int64, error) {
	var purchaseOrders []models.PurchaseOrder
	var total int64

	// Build base query for both count and data
	baseQuery := r.db.WithContext(ctx).Model(&models.PurchaseOrder{})

	// Apply search filter
	if params.Search != "" {
		baseQuery = baseQuery.Where("order_number ILIKE ? OR notes ILIKE ?", "%"+params.Search+"%", "%"+params.Search+"%")
	}

	// Get total count first
	err := baseQuery.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// Build query for data with preloads
	dataQuery := r.db.WithContext(ctx).Preload("Items").Preload("Items.Product")

	// Apply same search filter for data
	if params.Search != "" {
		dataQuery = dataQuery.Where("order_number ILIKE ? OR notes ILIKE ?", "%"+params.Search+"%", "%"+params.Search+"%")
	}

	// Apply sorting
	if params.Sort != "" {
		orderDirection := "ASC"
		if params.Order == "desc" {
			orderDirection = "DESC"
		}

		switch params.Sort {
		case "order_number", "status", "total_amount", "created_at", "updated_at":
			dataQuery = dataQuery.Order(params.Sort + " " + orderDirection)
		default:
			dataQuery = dataQuery.Order("created_at DESC") // Default sorting
		}
	} else {
		dataQuery = dataQuery.Order("created_at DESC") // Default sorting
	}

	err = dataQuery.Limit(params.Limit).Offset(params.GetOffset()).Find(&purchaseOrders).Error
	return purchaseOrders, total, err
}

func (r *purchaseOrderRepository) GetByStatus(status string) ([]models.PurchaseOrder, error) {
	var purchaseOrders []models.PurchaseOrder
	err := r.db.Preload("Items").Preload("Items.Product").Where("status = ?", status).Find(&purchaseOrders).Error
	return purchaseOrders, err
}
