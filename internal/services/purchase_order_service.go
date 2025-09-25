package services

import (
	"import-export-backend/internal/models"
	"import-export-backend/internal/repository"

	"github.com/google/uuid"
)

type PurchaseOrderService interface {
	CreatePurchaseOrder(purchaseOrder *models.PurchaseOrder) error
	GetPurchaseOrderByID(id uuid.UUID) (*models.PurchaseOrder, error)
	UpdatePurchaseOrder(purchaseOrder *models.PurchaseOrder) error
	DeletePurchaseOrder(id uuid.UUID) error
	ListPurchaseOrders(limit, offset int) ([]models.PurchaseOrder, error)
	GetPurchaseOrdersByStatus(status string) ([]models.PurchaseOrder, error)
	UpdatePurchaseOrderStatus(id uuid.UUID, status string) error
	ReceivePurchaseOrder(id uuid.UUID, userID string) error
	CountPurchaseOrders() (int64, error)
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

func (s *purchaseOrderService) CreatePurchaseOrder(purchaseOrder *models.PurchaseOrder) error {
	return s.purchaseOrderRepo.Create(purchaseOrder)
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

func (s *purchaseOrderService) ListPurchaseOrders(limit, offset int) ([]models.PurchaseOrder, error) {
	return s.purchaseOrderRepo.List(limit, offset)
}

func (s *purchaseOrderService) GetPurchaseOrdersByStatus(status string) ([]models.PurchaseOrder, error) {
	return s.purchaseOrderRepo.GetByStatus(status)
}

func (s *purchaseOrderService) UpdatePurchaseOrderStatus(id uuid.UUID, status string) error {
	purchaseOrder, err := s.purchaseOrderRepo.GetByID(id)
	if err != nil {
		return err
	}
	purchaseOrder.Status = status
	return s.purchaseOrderRepo.Update(purchaseOrder)
}

func (s *purchaseOrderService) ReceivePurchaseOrder(id uuid.UUID, userID string) error {
	purchaseOrder, err := s.purchaseOrderRepo.GetByID(id)
	if err != nil {
		return err
	}

	// Update status to received
	purchaseOrder.Status = "received"
	if err := s.purchaseOrderRepo.Update(purchaseOrder); err != nil {
		return err
	}

	// Add inventory for each item
	for _, item := range purchaseOrder.Items {
		if err := s.inventoryService.AddInventory(
			item.ProductID,
			item.Quantity,
			purchaseOrder.ID,
			"purchase_order",
			"Received from purchase order",
			userID,
		); err != nil {
			return err
		}
	}

	return nil
}

func (s *purchaseOrderService) CountPurchaseOrders() (int64, error) {
	return s.purchaseOrderRepo.Count()
}
