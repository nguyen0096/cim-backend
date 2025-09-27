package repository

import (
	"import-export-backend/internal/models"

	"gorm.io/gorm"
)

type OrderRepository interface {
	Create(order *models.Order) error
	GetByID(id uint) (*models.Order, error)
	Update(order *models.Order) error
	Delete(id uint) error
	List(limit, offset int) ([]models.Order, error)
	GetByStatus(status string) ([]models.Order, error)
	GetByCustomer(customerEmail string) ([]models.Order, error)
	Count() (int64, error)
}

type orderRepository struct {
	db *gorm.DB
}

func NewOrderRepository(db *gorm.DB) OrderRepository {
	return &orderRepository{db: db}
}

func (r *orderRepository) Create(order *models.Order) error {
	return r.db.Create(order).Error
}

func (r *orderRepository) GetByID(id uint) (*models.Order, error) {
	var order models.Order
	err := r.db.Preload("Items").Preload("Items.Product").First(&order, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *orderRepository) Update(order *models.Order) error {
	return r.db.Save(order).Error
}

func (r *orderRepository) Delete(id uint) error {
	return r.db.Delete(&models.Order{}, "id = ?", id).Error
}

func (r *orderRepository) List(limit, offset int) ([]models.Order, error) {
	var orders []models.Order
	err := r.db.Preload("Items").Preload("Items.Product").Limit(limit).Offset(offset).Find(&orders).Error
	return orders, err
}

func (r *orderRepository) GetByStatus(status string) ([]models.Order, error) {
	var orders []models.Order
	err := r.db.Preload("Items").Preload("Items.Product").Where("status = ?", status).Find(&orders).Error
	return orders, err
}

func (r *orderRepository) GetByCustomer(customerEmail string) ([]models.Order, error) {
	var orders []models.Order
	err := r.db.Preload("Items").Preload("Items.Product").Where("customer_email = ?", customerEmail).Find(&orders).Error
	return orders, err
}

func (r *orderRepository) Count() (int64, error) {
	var count int64
	err := r.db.Model(&models.Order{}).Count(&count).Error
	return count, err
}
