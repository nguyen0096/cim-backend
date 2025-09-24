package repository

import (
	"import-export-backend/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PurchaseOrderRepository interface {
	Create(purchaseOrder *models.PurchaseOrder) error
	GetByID(id uuid.UUID) (*models.PurchaseOrder, error)
	Update(purchaseOrder *models.PurchaseOrder) error
	Delete(id uuid.UUID) error
	List(limit, offset int) ([]models.PurchaseOrder, error)
	GetByStatus(status string) ([]models.PurchaseOrder, error)
	GetBySupplier(supplierID uuid.UUID) ([]models.PurchaseOrder, error)
}

type purchaseOrderRepository struct {
	db *gorm.DB
}

func NewPurchaseOrderRepository(db *gorm.DB) PurchaseOrderRepository {
	return &purchaseOrderRepository{db: db}
}

func (r *purchaseOrderRepository) Create(purchaseOrder *models.PurchaseOrder) error {
	return r.db.Create(purchaseOrder).Error
}

func (r *purchaseOrderRepository) GetByID(id uuid.UUID) (*models.PurchaseOrder, error) {
	var purchaseOrder models.PurchaseOrder
	err := r.db.Preload("Supplier").Preload("Items").Preload("Items.Product").First(&purchaseOrder, "id = ?", id).Error
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

func (r *purchaseOrderRepository) List(limit, offset int) ([]models.PurchaseOrder, error) {
	var purchaseOrders []models.PurchaseOrder
	err := r.db.Preload("Supplier").Preload("Items").Preload("Items.Product").Limit(limit).Offset(offset).Find(&purchaseOrders).Error
	return purchaseOrders, err
}

func (r *purchaseOrderRepository) GetByStatus(status string) ([]models.PurchaseOrder, error) {
	var purchaseOrders []models.PurchaseOrder
	err := r.db.Preload("Supplier").Preload("Items").Preload("Items.Product").Where("status = ?", status).Find(&purchaseOrders).Error
	return purchaseOrders, err
}

func (r *purchaseOrderRepository) GetBySupplier(supplierID uuid.UUID) ([]models.PurchaseOrder, error) {
	var purchaseOrders []models.PurchaseOrder
	err := r.db.Preload("Supplier").Preload("Items").Preload("Items.Product").Where("supplier_id = ?", supplierID).Find(&purchaseOrders).Error
	return purchaseOrders, err
}
