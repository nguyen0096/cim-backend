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
	Restore(id uuid.UUID) error
	List(limit, offset int, sortBy, sortOrder string) ([]models.Product, error)
	GetBySupplier(supplierID uuid.UUID) ([]models.Product, error)
	Search(query string, sortBy, sortOrder string) ([]models.Product, error)
	SearchWithPagination(query string, limit, offset int, sortBy, sortOrder string) ([]models.Product, error)
	Count() (int64, error)
	CountSearch(query string) (int64, error)
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

func (r *productRepository) Restore(id uuid.UUID) error {
	return r.db.Unscoped().Model(&models.Product{}).Where("id = ?", id).Update("deleted_at", nil).Error
}

func (r *productRepository) List(limit, offset int, sortBy, sortOrder string) ([]models.Product, error) {
	var products []models.Product
	query := r.db.Preload("Supplier").Preload("Inventory")

	// Apply sorting
	if sortBy != "" {
		if sortOrder == "" {
			sortOrder = "asc"
		}
		query = query.Order(sortBy + " " + sortOrder)
	} else {
		query = query.Order("created_at desc")
	}

	err := query.Limit(limit).Offset(offset).Find(&products).Error
	return products, err
}

func (r *productRepository) GetBySupplier(supplierID uuid.UUID) ([]models.Product, error) {
	var products []models.Product
	err := r.db.Preload("Supplier").Preload("Inventory").Where("supplier_id = ?", supplierID).Find(&products).Error
	return products, err
}

func (r *productRepository) Search(query string, sortBy, sortOrder string) ([]models.Product, error) {
	var products []models.Product
	dbQuery := r.db.Preload("Supplier").Preload("Inventory").Where("name ILIKE ?", "%"+query+"%")

	// Apply sorting
	if sortBy != "" {
		if sortOrder == "" {
			sortOrder = "asc"
		}
		dbQuery = dbQuery.Order(sortBy + " " + sortOrder)
	} else {
		dbQuery = dbQuery.Order("created_at desc")
	}

	err := dbQuery.Find(&products).Error
	return products, err
}

func (r *productRepository) SearchWithPagination(query string, limit, offset int, sortBy, sortOrder string) ([]models.Product, error) {
	var products []models.Product
	dbQuery := r.db.Preload("Supplier").Preload("Inventory").Where("name ILIKE ?", "%"+query+"%")

	// Apply sorting
	if sortBy != "" {
		if sortOrder == "" {
			sortOrder = "asc"
		}
		dbQuery = dbQuery.Order(sortBy + " " + sortOrder)
	} else {
		dbQuery = dbQuery.Order("created_at desc")
	}

	err := dbQuery.Limit(limit).Offset(offset).Find(&products).Error
	return products, err
}

func (r *productRepository) Count() (int64, error) {
	var count int64
	err := r.db.Model(&models.Product{}).Count(&count).Error
	return count, err
}

func (r *productRepository) CountSearch(query string) (int64, error) {
	var count int64
	err := r.db.Model(&models.Product{}).Where("name ILIKE ?", "%"+query+"%").Count(&count).Error
	return count, err
}
