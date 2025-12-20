package services

import (
	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/pkg"
	"cim-backend/pkg/log"
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

//go:generate mockery --name=SaleOrderService --structname=SaleOrderService --output=../mocks/servicemocks --outpkg=servicemocks
type SaleOrderService interface {
	CreateSaleOrder(ctx context.Context, saleOrder *models.SaleOrder) error
	UpdateSaleOrder(ctx context.Context, id uint, saleOrder *models.SaleOrder) (*models.SaleOrder, error)
	UpdateSaleOrderStatus(ctx context.Context, id uint, status models.SaleOrderStatus) error
	GetSaleOrderByID(ctx context.Context, id uint) (*models.SaleOrder, error)
	ListSaleOrders(ctx context.Context, params models.ListParams, tag *int) (*models.PaginationResult[models.SaleOrder], error)
}

type saleOrderService struct {
	saleOrderRepo repository.SaleOrderRepository
	db            *gorm.DB
}

func NewSaleOrderService(
	saleOrderRepo repository.SaleOrderRepository,
	db *gorm.DB,
) SaleOrderService {
	return &saleOrderService{
		saleOrderRepo: saleOrderRepo,
		db:            db,
	}
}

// generateSaleOrderNumber generates a sale order number similar to purchase order format
func (s *saleOrderService) generateSaleOrderNumber() (string, error) {
	now := time.Now()

	// Generate 2-character random alphanumeric suffix
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	suffix := make([]byte, 2)
	if _, err := rand.Read(suffix); err != nil {
		return "", fmt.Errorf("failed to generate random suffix: %w", err)
	}

	for i := range suffix {
		suffix[i] = charset[suffix[i]%byte(len(charset))]
	}

	// Format: SO-YYMMDD-HHMMSS-XX
	return fmt.Sprintf("SO-%s-%s",
		now.Format("060102-150405"),
		string(suffix)), nil
}

// generateCustomerID generates a ULID for customer ID
func (s *saleOrderService) generateCustomerID() string {
	// Using UUID v4 as ULID alternative (26 chars when base32 encoded, but UUID is simpler)
	// For now, using UUID which is 36 chars, but we can switch to proper ULID library later
	// ULID format: 26 characters (Crockford's Base32)
	// For simplicity, using UUID without dashes (32 chars) or we can use a ULID library
	// Using a simple approach: timestamp + random
	ulid := uuid.New().String()
	// Remove dashes to get 32 chars, but ULID should be 26
	// For now, using first 26 chars of UUID (not perfect ULID but works)
	ulidNoDashes := strings.ReplaceAll(ulid, "-", "")
	if len(ulidNoDashes) > 26 {
		return ulidNoDashes[:26]
	}
	return ulidNoDashes
}

func (s *saleOrderService) CreateSaleOrder(ctx context.Context, saleOrder *models.SaleOrder) error {
	log.WithFields(logrus.Fields{
		"operation": "CreateSaleOrder",
	}).Info("Creating new sale order")

	// Set defaults
	saleOrder.PreviousOrderID = nil
	saleOrder.IsLatest = true

	// Generate customer ID if not provided
	if saleOrder.CustomerID == "" {
		saleOrder.CustomerID = s.generateCustomerID()
	}

	// Generate order number if not provided
	if saleOrder.OrderNumber == "" {
		orderNumber, err := s.generateSaleOrderNumber()
		if err != nil {
			return fmt.Errorf("failed to generate order number: %w", err)
		}
		saleOrder.OrderNumber = orderNumber
	}

	// Validate sale order struct
	if err := pkg.Validator.Struct(saleOrder); err != nil {
		log.WithFields(logrus.Fields{
			"operation": "CreateSaleOrder",
			"error":     err,
		}).Error("Sale order validation failed")
		return pkg.ErrValidation("validation failed", err)
	}

	// Create sale order
	if err := s.saleOrderRepo.Create(ctx, saleOrder); err != nil {
		log.WithFields(logrus.Fields{
			"operation": "CreateSaleOrder",
			"error":     err,
		}).Error("Failed to create sale order")
		return fmt.Errorf("failed to create sale order: %w", err)
	}

	// Calculate total price
	saleOrder.CalculateTotalPrice()

	log.WithFields(logrus.Fields{
		"operation":     "CreateSaleOrder",
		"sale_order_id": saleOrder.ID,
		"order_number":  saleOrder.OrderNumber,
	}).Info("Successfully created sale order")

	return nil
}

func (s *saleOrderService) UpdateSaleOrder(ctx context.Context, id uint, saleOrder *models.SaleOrder) (*models.SaleOrder, error) {
	log.WithFields(logrus.Fields{
		"operation":     "UpdateSaleOrder",
		"sale_order_id": id,
	}).Info("Updating sale order")

	// Get existing sale order
	existing, err := s.saleOrderRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get existing sale order: %w", err)
	}
	if existing == nil {
		return nil, pkg.NewAppError(pkg.ErrorCodeNotFound, "Sale order not found", nil)
	}

	// If status is cancelled|served, just update the existing record without creating a new version
	if saleOrder.Status == models.SaleOrderStatusCancelled || saleOrder.Status == models.SaleOrderStatusServed {
		existing.Status = saleOrder.Status
		existing.Notes = saleOrder.Notes
		existing.Items = saleOrder.Items
		// Tag is kept the same (from existing record)

		// Validate updated sale order
		if err := pkg.Validator.Struct(existing); err != nil {
			return nil, pkg.ErrValidation("validation failed", err)
		}

		// Update existing sale order
		if err := s.saleOrderRepo.Update(ctx, existing); err != nil {
			return nil, fmt.Errorf("failed to update sale order: %w", err)
		}

		// Reload with relationships
		updated, err := s.saleOrderRepo.GetByID(ctx, existing.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get updated sale order: %w", err)
		}

		// Calculate total price
		updated.CalculateTotalPrice()

		log.WithFields(logrus.Fields{
			"operation":     "UpdateSaleOrder",
			"sale_order_id": existing.ID,
			"order_number":  existing.OrderNumber,
			"status":        saleOrder.Status,
		}).Info("Successfully updated sale order")

		return updated, nil
	}

	// For non-cancellation|served updates, create a new version
	// Start transaction
	tx := s.db.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Set existing sale order's isLatest to false
	if err := tx.Model(&models.SaleOrder{}).
		Where("id = ?", id).
		Update("is_latest", false).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update existing sale order isLatest: %w", err)
	}

	// Create new sale order with previousOrderID pointing to existing
	newSaleOrder := &models.SaleOrder{
		PreviousOrderID: &existing.ID,
		IsLatest:        true,
		CustomerID:      existing.CustomerID,  // Keep same customer ID
		Tag:             existing.Tag,         // Keep same tag
		OrderNumber:     existing.OrderNumber, // Must be same as existing
		InventoryID:     existing.InventoryID, // Must be same as existing
		Status:          saleOrder.Status,
		Notes:           saleOrder.Notes,
		Items:           saleOrder.Items,
	}

	// Validate new sale order
	if err := pkg.Validator.Struct(newSaleOrder); err != nil {
		tx.Rollback()
		return nil, pkg.ErrValidation("validation failed", err)
	}

	// Create new sale order
	if err := tx.Create(newSaleOrder).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create new sale order: %w", err)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Reload with relationships
	updated, err := s.saleOrderRepo.GetByID(ctx, newSaleOrder.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated sale order: %w", err)
	}

	// Calculate total price
	updated.CalculateTotalPrice()

	log.WithFields(logrus.Fields{
		"operation":         "UpdateSaleOrder",
		"old_sale_order_id": id,
		"new_sale_order_id": updated.ID,
		"order_number":      updated.OrderNumber,
	}).Info("Successfully updated sale order")

	return updated, nil
}

func (s *saleOrderService) UpdateSaleOrderStatus(ctx context.Context, id uint, status models.SaleOrderStatus) error {
	log.WithFields(logrus.Fields{
		"operation":     "UpdateSaleOrderStatus",
		"sale_order_id": id,
		"status":        status,
	}).Info("Updating sale order status")

	// Verify sale order exists
	existing, err := s.saleOrderRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get sale order: %w", err)
	}
	if existing == nil {
		return pkg.NewAppError(pkg.ErrorCodeNotFound, "Sale order not found", nil)
	}

	// Update status only (no new record created)
	err = s.saleOrderRepo.UpdateStatus(ctx, id, status)
	if err != nil {
		log.WithFields(logrus.Fields{
			"operation":     "UpdateSaleOrderStatus",
			"sale_order_id": id,
			"status":        status,
			"error":         err,
		}).Error("Failed to update sale order status")
		return fmt.Errorf("failed to update sale order status: %w", err)
	}

	log.WithFields(logrus.Fields{
		"operation":     "UpdateSaleOrderStatus",
		"sale_order_id": id,
		"status":        status,
	}).Info("Successfully updated sale order status")

	return nil
}

func (s *saleOrderService) GetSaleOrderByID(ctx context.Context, id uint) (*models.SaleOrder, error) {
	saleOrder, err := s.saleOrderRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get sale order: %w", err)
	}
	if saleOrder == nil {
		return nil, pkg.NewAppError(pkg.ErrorCodeNotFound, "Sale order not found", nil)
	}

	// Calculate total price
	saleOrder.CalculateTotalPrice()

	return saleOrder, nil
}

func (s *saleOrderService) ListSaleOrders(ctx context.Context, params models.ListParams, tag *int) (*models.PaginationResult[models.SaleOrder], error) {
	params.ValidateAndSetDefaults()

	saleOrders, total, err := s.saleOrderRepo.List(ctx, params, tag)
	if err != nil {
		return nil, fmt.Errorf("failed to list sale orders: %w", err)
	}

	// Calculate total price for each sale order
	for i := range saleOrders {
		saleOrders[i].CalculateTotalPrice()
	}

	return models.NewPaginationResult(saleOrders, total, params.Page, params.Limit), nil
}
