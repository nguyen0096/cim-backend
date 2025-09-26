package services

import (
	"context"
	"crypto/rand"
	"fmt"
	"import-export-backend/internal/models"
	"import-export-backend/internal/repository"
	"time"

	"github.com/google/uuid"
)

type PurchaseOrderService interface {
	CreatePurchaseOrder(ctx context.Context, purchaseOrder *models.PurchaseOrder) error
	GetPurchaseOrderByID(id uuid.UUID) (*models.PurchaseOrder, error)
	UpdatePurchaseOrder(purchaseOrder *models.PurchaseOrder) error
	DeletePurchaseOrder(id uuid.UUID) error
	ListPurchaseOrders(ctx context.Context, params models.PaginationParams) (*models.PaginationResult[models.PurchaseOrder], error)
	GetPurchaseOrdersByStatus(status string) ([]models.PurchaseOrder, error)
	UpdatePurchaseOrderStatus(id uuid.UUID, status string) error
	ReceivePurchaseOrder(id uuid.UUID, userID string) error
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

func (s *purchaseOrderService) GetPurchaseOrderByID(id uuid.UUID) (*models.PurchaseOrder, error) {
	return s.purchaseOrderRepo.GetByID(id)
}

func (s *purchaseOrderService) UpdatePurchaseOrder(purchaseOrder *models.PurchaseOrder) error {
	return s.purchaseOrderRepo.Update(purchaseOrder)
}

func (s *purchaseOrderService) DeletePurchaseOrder(id uuid.UUID) error {
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

	// Create pagination result
	result := models.NewPaginationResult(purchaseOrders, total, params.Page, params.Limit)
	return result, nil
}

func (s *purchaseOrderService) GetPurchaseOrdersByStatus(status string) ([]models.PurchaseOrder, error) {
	return s.purchaseOrderRepo.GetByStatus(status)
}

// UpdatePurchaseOrderStatus updates the status of a purchase order
func (s *purchaseOrderService) UpdatePurchaseOrderStatus(id uuid.UUID, status string) error {
	purchaseOrder, err := s.purchaseOrderRepo.GetByID(id)
	if err != nil {
		return err
	}
	purchaseOrder.Status = models.PurchaseOrderStatus(status)
	return s.purchaseOrderRepo.Update(purchaseOrder)
}

func (s *purchaseOrderService) ReceivePurchaseOrder(id uuid.UUID, userID string) error {
	purchaseOrder, err := s.purchaseOrderRepo.GetByID(id)
	if err != nil {
		return err
	}

	// Update status to received
	purchaseOrder.Status = models.PurchaseOrderStatusCompleted
	if err := s.purchaseOrderRepo.Update(purchaseOrder); err != nil {
		return err
	}

	// Add inventory for each item
	for _, item := range purchaseOrder.Items {
		if err := s.inventoryService.AddInventory(
			*item.ProductID,
			item.Quantity,
			*purchaseOrder.ID,
			"purchase_order",
			"Received from purchase order",
			userID,
		); err != nil {
			return err
		}
	}

	return nil
}
