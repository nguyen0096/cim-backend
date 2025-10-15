package repository

import (
	"cim-backend/internal/models"
	"context"

	"gorm.io/gorm"
)

//go:generate mockery --name=ProductRepository --structname=ProductRepository --output=../mocks/repositories --outpkg=repositorymocks
type ProductRepository interface {
	Create(ctx context.Context, product *models.Product) error
	BulkCreate(ctx context.Context, products []models.Product) error
	GetByID(ctx context.Context, id uint) (*models.Product, error)
	Update(ctx context.Context, product *models.Product) error
	UpdateStatus(ctx context.Context, id uint, status string) error
	Delete(ctx context.Context, id uint) error
	Restore(ctx context.Context, id uint) error
	GetBySupplier(ctx context.Context, supplierID uint) ([]models.Product, error)
	Search(ctx context.Context, query string, sortBy, sortOrder string) ([]models.Product, error)
	SearchWithPagination(ctx context.Context, query string, limit, offset int, sortBy, sortOrder, status string) ([]models.Product, error)
	Count(ctx context.Context, status string) (int64, error)
	CountSearch(ctx context.Context, query string, status string) (int64, error)

	// v1
	List(ctx context.Context, limit, offset int, sortBy, sortOrder, status string) ([]models.Product, error)
}

type productRepository struct {
	db *gorm.DB
}

func NewProductRepository(db *gorm.DB) ProductRepository {
	return &productRepository{db: db}
}

func (r *productRepository) Create(ctx context.Context, product *models.Product) error {
	return r.db.WithContext(ctx).Create(product).Error
}

// BulkCreate inserts multiple products in a single transaction
func (r *productRepository) BulkCreate(ctx context.Context, products []models.Product) error {
	if len(products) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&products).Error
}

func (r *productRepository) GetByID(ctx context.Context, id uint) (*models.Product, error) {
	var product models.Product
	err := r.db.WithContext(ctx).Preload("Suppliers").Preload("InventoryItems.Inventory").First(&product, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &product, nil
}

func (r *productRepository) Update(ctx context.Context, product *models.Product) error {
	return r.db.WithContext(ctx).Save(product).Error
}

// UpdateStatus updates the status of a product
func (r *productRepository) UpdateStatus(ctx context.Context, id uint, status string) error {
	return r.db.WithContext(ctx).Model(&models.Product{}).Where("id = ?", id).Update("status", status).Error
}

func (r *productRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.Product{}, "id = ?", id).Error
}

func (r *productRepository) Restore(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Unscoped().Model(&models.Product{}).Where("id = ?", id).Update("deleted_at", nil).Error
}

func (r *productRepository) List(ctx context.Context, limit, offset int, sortBy, sortOrder, status string) ([]models.Product, error) {
	var products []models.Product
	query := r.db.WithContext(ctx).
		Preload("Suppliers").
		Preload("InventoryItems")

	// Apply status filter
	if status != "" {
		query = query.Where("status = ?", status)
	}

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

func (r *productRepository) GetBySupplier(ctx context.Context, supplierID uint) ([]models.Product, error) {
	var products []models.Product
	err := r.db.WithContext(ctx).Preload("Suppliers").Preload("InventoryItems").
		Joins("JOIN product_suppliers ON products.id = product_suppliers.product_id").
		Where("product_suppliers.supplier_id = ?", supplierID).
		Find(&products).Error
	return products, err
}

func (r *productRepository) Search(ctx context.Context, query string, sortBy, sortOrder string) ([]models.Product, error) {
	var products []models.Product
	dbQuery := r.db.WithContext(ctx).Preload("Suppliers").Preload("InventoryItems").Where("name ILIKE ? OR product_type ILIKE ?", "%"+query+"%", "%"+query+"%")

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

func (r *productRepository) SearchWithPagination(ctx context.Context, query string, limit, offset int, sortBy, sortOrder, status string) ([]models.Product, error) {
	var products []models.Product
	dbQuery := r.db.WithContext(ctx).Preload("Suppliers").Preload("InventoryItems").Where("name ILIKE ? OR product_type ILIKE ?", "%"+query+"%", "%"+query+"%")

	// Apply status filter
	if status != "" {
		dbQuery = dbQuery.Where("status = ?", status)
	}

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

func (r *productRepository) Count(ctx context.Context, status string) (int64, error) {
	var count int64
	query := r.db.WithContext(ctx).Model(&models.Product{})

	// Apply status filter
	if status != "" {
		query = query.Where("status = ?", status)
	}

	err := query.Count(&count).Error
	return count, err
}

func (r *productRepository) CountSearch(ctx context.Context, query string, status string) (int64, error) {
	var count int64
	dbQuery := r.db.WithContext(ctx).Model(&models.Product{}).Where("name ILIKE ? OR product_type ILIKE ?", "%"+query+"%", "%"+query+"%")

	// Apply status filter
	if status != "" {
		dbQuery = dbQuery.Where("status = ?", status)
	}

	err := dbQuery.Count(&count).Error
	return count, err
}
