package repository

import (
	"import-export-backend/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ProductRepository interface {
	Create(product *models.Product) error
	GetByID(id uuid.UUID) (*models.Product, error)
	Update(product *models.Product) error
	Delete(id uuid.UUID) error
	List(limit, offset int) ([]models.Product, error)
	GetBySupplier(supplierID uuid.UUID) ([]models.Product, error)
	Search(query string) ([]models.Product, error)
}

type productRepository struct {
	db *gorm.DB
}

func NewProductRepository(db *gorm.DB) ProductRepository {
	return &productRepository{db: db}
}

func (r *productRepository) Create(product *models.Product) error {
	return r.db.Create(product).Error
}

func (r *productRepository) GetByID(id uuid.UUID) (*models.Product, error) {
	var product models.Product
	err := r.db.Preload("Supplier").Preload("Inventory").First(&product, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &product, nil
}

func (r *productRepository) Update(product *models.Product) error {
	return r.db.Save(product).Error
}

func (r *productRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&models.Product{}, "id = ?", id).Error
}

func (r *productRepository) List(limit, offset int) ([]models.Product, error) {
	var products []models.Product
	err := r.db.Preload("Supplier").Preload("Inventory").Limit(limit).Offset(offset).Find(&products).Error
	return products, err
}

func (r *productRepository) GetBySupplier(supplierID uuid.UUID) ([]models.Product, error) {
	var products []models.Product
	err := r.db.Preload("Supplier").Preload("Inventory").Where("supplier_id = ?", supplierID).Find(&products).Error
	return products, err
}

func (r *productRepository) Search(query string) ([]models.Product, error) {
	var products []models.Product
	err := r.db.Preload("Supplier").Preload("Inventory").Where("name ILIKE ? OR sku ILIKE ?", "%"+query+"%", "%"+query+"%").Find(&products).Error
	return products, err
}
