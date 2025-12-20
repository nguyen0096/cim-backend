package repository

import (
	"cim-backend/internal/models"
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

//go:generate mockery --name=SaleOrderRepository --structname=SaleOrderRepository --output=../mocks/repositorymocks --outpkg=repositorymocks
type SaleOrderRepository interface {
	Create(ctx context.Context, saleOrder *models.SaleOrder) error
	GetByID(ctx context.Context, id uint) (*models.SaleOrder, error)
	Update(ctx context.Context, saleOrder *models.SaleOrder) error
	UpdateIsLatest(ctx context.Context, id uint, isLatest bool) error
	UpdateStatus(ctx context.Context, id uint, status models.SaleOrderStatus) error
	List(ctx context.Context, params models.ListParams, tag *int) ([]models.SaleOrder, int64, error)
}

type saleOrderRepository struct {
	db *gorm.DB
}

func NewSaleOrderRepository(db *gorm.DB) SaleOrderRepository {
	return &saleOrderRepository{db: db}
}

func (r *saleOrderRepository) Create(ctx context.Context, saleOrder *models.SaleOrder) error {
	return r.db.WithContext(ctx).Create(saleOrder).Error
}

func (r *saleOrderRepository) GetByID(ctx context.Context, id uint) (*models.SaleOrder, error) {
	var saleOrder models.SaleOrder
	err := r.db.WithContext(ctx).
		Preload("Items").
		Preload("Items.MenuItems").
		Preload("Items.MenuItems.Products").
		Preload("Inventory").
		Preload("PreviousOrder").
		First(&saleOrder, "id = ?", id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get sale order: %w", err)
	}
	return &saleOrder, nil
}

func (r *saleOrderRepository) Update(ctx context.Context, saleOrder *models.SaleOrder) error {
	return r.db.WithContext(ctx).Save(saleOrder).Error
}

func (r *saleOrderRepository) UpdateIsLatest(ctx context.Context, id uint, isLatest bool) error {
	return r.db.WithContext(ctx).
		Model(&models.SaleOrder{}).
		Where("id = ?", id).
		Update("is_latest", isLatest).Error
}

func (r *saleOrderRepository) UpdateStatus(ctx context.Context, id uint, status models.SaleOrderStatus) error {
	return r.db.WithContext(ctx).
		Model(&models.SaleOrder{}).
		Where("id = ?", id).
		Update("status", status).Error
}

func (r *saleOrderRepository) List(ctx context.Context, params models.ListParams, tag *int) ([]models.SaleOrder, int64, error) {
	var saleOrders []models.SaleOrder
	var total int64

	baseQuery := r.db.WithContext(ctx).Model(&models.SaleOrder{}).
		Preload("Inventory").
		Preload("Items").
		Preload("Items.MenuItems").
		Preload("Items.MenuItems.Products")

	// Apply search filter
	if params.Search != "" {
		searchPattern := "%" + params.Search + "%"
		baseQuery = baseQuery.Where("order_number ILIKE ? OR notes ILIKE ?", searchPattern, searchPattern)
	}

	// Apply tag filter
	if tag != nil {
		baseQuery = baseQuery.Where("tag = ?", *tag)
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
			endTime = endTime.Add(24 * time.Hour)
			baseQuery = baseQuery.Where("created_at < ?", endTime)
		}
	}

	// Handle status filtering
	if params.Status != "" {
		statusStrings := strings.Split(params.Status, ",")
		statuses := make([]models.SaleOrderStatus, 0, len(statusStrings))
		for _, statusStr := range statusStrings {
			trimmedStatus := strings.TrimSpace(statusStr)
			if trimmedStatus != "" {
				statuses = append(statuses, models.SaleOrderStatus(trimmedStatus))
			}
		}
		if len(statuses) > 0 {
			baseQuery = baseQuery.Where("status IN ?", statuses)
		}
	}

	// Get total count first
	err := baseQuery.Count(&total).Error
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count sale orders: %w", err)
	}

	// Apply sorting
	if params.Sort != "" {
		orderDirection := "ASC"
		if params.Order == "desc" {
			orderDirection = "DESC"
		}
		switch params.Sort {
		case "order_number", "status", "created_at", "updated_at", "tag":
			baseQuery = baseQuery.Order(params.Sort + " " + orderDirection)
		default:
			baseQuery = baseQuery.Order("created_at DESC")
		}
	} else {
		baseQuery = baseQuery.Order("created_at DESC")
	}

	// Apply pagination
	err = baseQuery.Limit(params.Limit).Offset(params.GetOffset()).
		Find(&saleOrders).Error
	if err != nil {
		return nil, 0, fmt.Errorf("failed to fetch sale orders: %w", err)
	}

	return saleOrders, total, nil
}
