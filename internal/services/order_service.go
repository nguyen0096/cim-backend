package services

import (
	"context"
	"import-export-backend/internal/models"
	"import-export-backend/internal/repository"
)

type OrderService interface {
	CreateOrder(order *models.Order) error
	GetOrderByID(id uint) (*models.Order, error)
	UpdateOrder(order *models.Order) error
	DeleteOrder(id uint) error
	ListOrders(limit, offset int) ([]models.Order, error)
	GetOrdersByStatus(status string) ([]models.Order, error)
	GetOrdersByCustomer(customerEmail string) ([]models.Order, error)
	UpdateOrderStatus(id uint, status string) error
	CompleteOrder(ctx context.Context, id uint) error
	CountOrders() (int64, error)
}

type orderService struct {
	orderRepo        repository.OrderRepository
	inventoryService InventoryService
}

func NewOrderService(orderRepo repository.OrderRepository, inventoryService InventoryService) OrderService {
	return &orderService{
		orderRepo:        orderRepo,
		inventoryService: inventoryService,
	}
}

func (s *orderService) CreateOrder(order *models.Order) error {
	return s.orderRepo.Create(order)
}

func (s *orderService) GetOrderByID(id uint) (*models.Order, error) {
	return s.orderRepo.GetByID(id)
}

func (s *orderService) UpdateOrder(order *models.Order) error {
	return s.orderRepo.Update(order)
}

func (s *orderService) DeleteOrder(id uint) error {
	return s.orderRepo.Delete(id)
}

func (s *orderService) ListOrders(limit, offset int) ([]models.Order, error) {
	return s.orderRepo.List(limit, offset)
}

func (s *orderService) GetOrdersByStatus(status string) ([]models.Order, error) {
	return s.orderRepo.GetByStatus(status)
}

func (s *orderService) GetOrdersByCustomer(customerEmail string) ([]models.Order, error) {
	return s.orderRepo.GetByCustomer(customerEmail)
}

func (s *orderService) UpdateOrderStatus(id uint, status string) error {
	order, err := s.orderRepo.GetByID(id)
	if err != nil {
		return err
	}
	order.Status = status
	return s.orderRepo.Update(order)
}

func (s *orderService) CompleteOrder(ctx context.Context, id uint) error {
	order, err := s.orderRepo.GetByID(id)
	if err != nil {
		return err
	}

	// Update status to completed
	order.Status = "completed"
	if err := s.orderRepo.Update(order); err != nil {
		return err
	}

	// Remove inventory for each item
	for _, item := range order.Items {
		if err := s.inventoryService.RemoveInventory(
			ctx,
			item.ProductID,
			item.Quantity,
			order.ID,
			"order",
			"Order completed",
		); err != nil {
			return err
		}
	}

	return nil
}

func (s *orderService) CountOrders() (int64, error) {
	return s.orderRepo.Count()
}
