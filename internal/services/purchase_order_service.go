package services

import (
	"context"
	"crypto/rand"
	"fmt"
	"import-export-backend/internal/models"
	"import-export-backend/internal/repository"
	"time"
)

//go:generate mockery --name=PurchaseOrderService --structname=PurchaseOrderService --output=./servicemocks --outpkg=servicemocks
type PurchaseOrderService interface {
	CreatePurchaseOrder(ctx context.Context, purchaseOrder *models.PurchaseOrder) error
	GetPurchaseOrderByID(id uint) (*models.PurchaseOrder, error)
	UpdatePurchaseOrder(ctx context.Context, purchaseOrder *models.PurchaseOrder) error
	DeletePurchaseOrder(id uint) error
	ListPurchaseOrders(ctx context.Context, params models.PaginationParams) (*models.PaginationResult[models.PurchaseOrder], error)
	GetPurchaseOrdersByStatus(status string) ([]models.PurchaseOrder, error)
	UpdatePurchaseOrderStatus(ctx context.Context, id uint, status string) error
	ReceivePurchaseOrder(ctx context.Context, id uint) error
	UpdatePurchaseOrderItemStatus(ctx context.Context, purchaseOrderID, itemID uint, status models.PurchaseOrderItemStatus) error
}

type purchaseOrderService struct {
	purchaseOrderRepo repository.PurchaseOrderRepository
	inventoryService  InventoryService
}

func NewPurchaseOrderService(purchaseOrderRepo repository.PurchaseOrderRepository, inventoryService InventoryService) PurchaseOrderService {
	return &purchaseOrderService{
		purchaseOrderRepo: purchaseOrderRepo,
		inventoryService:  inventoryService,
	}
}

// generatePurchaseOrderNumber generates a unique purchase order number
func (s *purchaseOrderService) generatePurchaseOrderNumber() (string, error) {
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

	// Format: PO-YYMMDD-HHMMSS-XX
	return fmt.Sprintf("PO-%s-%s",
		now.Format("060102-150405"),
		string(suffix)), nil
}

// CreatePurchaseOrder creates a new purchase order with auto-generated order number
func (s *purchaseOrderService) CreatePurchaseOrder(ctx context.Context, purchaseOrder *models.PurchaseOrder) error {
	// Generate order number if not provided
	if purchaseOrder.OrderNumber == "" {
		orderNumber, err := s.generatePurchaseOrderNumber()
		if err != nil {
			return fmt.Errorf("failed to generate purchase order number: %w", err)
		}
		purchaseOrder.OrderNumber = orderNumber
	}

	// Set status to order_placed for new purchase orders
	purchaseOrder.Status = models.PurchaseOrderStatusOrderPlaced

	return s.purchaseOrderRepo.Create(ctx, purchaseOrder)
}

func (s *purchaseOrderService) GetPurchaseOrderByID(id uint) (*models.PurchaseOrder, error) {
	purchaseOrder, err := s.purchaseOrderRepo.GetByID(id)
	if err != nil {
		return nil, err
	}

	// Calculate total amount based on items
	purchaseOrder.TotalAmount = purchaseOrder.CalculateTotalAmount()

	return purchaseOrder, nil
}

func (s *purchaseOrderService) UpdatePurchaseOrder(ctx context.Context, purchaseOrder *models.PurchaseOrder) error {
	return s.purchaseOrderRepo.Update(ctx, purchaseOrder)
}

func (s *purchaseOrderService) DeletePurchaseOrder(id uint) error {
	return s.purchaseOrderRepo.Delete(id)
}

// ListPurchaseOrders retrieves purchase orders with search and pagination
func (s *purchaseOrderService) ListPurchaseOrders(ctx context.Context, params models.PaginationParams) (*models.PaginationResult[models.PurchaseOrder], error) {
	// Validate and set defaults for pagination parameters
	params.ValidateAndSetDefaults()

	// Get data and count from repository
	purchaseOrders, total, err := s.purchaseOrderRepo.List(ctx, params)
	if err != nil {
		return nil, err
	}

	// Calculate total amount for each purchase order based on items
	for i := range purchaseOrders {
		purchaseOrders[i].TotalAmount = purchaseOrders[i].CalculateTotalAmount()
	}

	// Create pagination result
	result := models.NewPaginationResult(purchaseOrders, total, params.Page, params.Limit)
	return result, nil
}

func (s *purchaseOrderService) GetPurchaseOrdersByStatus(status string) ([]models.PurchaseOrder, error) {
	purchaseOrders, err := s.purchaseOrderRepo.GetByStatus(status)
	if err != nil {
		return nil, err
	}

	// Calculate total amount for each purchase order based on items
	for i := range purchaseOrders {
		purchaseOrders[i].TotalAmount = purchaseOrders[i].CalculateTotalAmount()
	}

	return purchaseOrders, nil
}

// UpdatePurchaseOrderStatus updates the status of a purchase order
func (s *purchaseOrderService) UpdatePurchaseOrderStatus(ctx context.Context, id uint, status string) error {
	purchaseOrder, err := s.purchaseOrderRepo.GetByID(id)
	if err != nil {
		return fmt.Errorf("failed to get purchase order: %w", err)
	}
	purchaseOrder.Status = models.PurchaseOrderStatus(status)
	return s.purchaseOrderRepo.Update(ctx, purchaseOrder)
}

func (s *purchaseOrderService) ReceivePurchaseOrder(ctx context.Context, id uint) error {
	purchaseOrder, err := s.purchaseOrderRepo.GetByID(id)
	if err != nil {
		return err
	}

	// Update status to received
	purchaseOrder.Status = models.PurchaseOrderStatusCompleted
	if err := s.purchaseOrderRepo.Update(ctx, purchaseOrder); err != nil {
		return fmt.Errorf("failed to update purchase order: %w", err)
	}

	// Add inventory for each item
	for _, item := range purchaseOrder.Items {
		if err := s.inventoryService.AddInventory(
			ctx,
			*item.ProductID,
			item.Quantity,
			purchaseOrder.ID,
			"purchase_order",
			"Received from purchase order",
		); err != nil {
			return err
		}
	}

	return nil
}

// UpdatePurchaseOrderItemStatus updates the status of a purchase order item
func (s *purchaseOrderService) UpdatePurchaseOrderItemStatus(ctx context.Context, purchaseOrderID, itemID uint, status models.PurchaseOrderItemStatus) error {
	// Validate status
	if status != models.PurchaseOrderItemStatusDelivering && status != models.PurchaseOrderItemStatusDelivered {
		return fmt.Errorf("invalid status: %s", status)
	}

	return s.purchaseOrderRepo.UpdatePurchaseOrderItemStatus(ctx, purchaseOrderID, itemID, status)
}
