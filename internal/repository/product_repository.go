package repository

import (
	"cim-backend/internal/models"
	"context"
	"fmt"

	"gorm.io/gorm"
)

//go:generate mockery --name=ProductRepository --structname=ProductRepository --output=../mocks/repositorymocks --outpkg=repositorymocks
type ProductRepository interface {
	Create(ctx context.Context, product *models.Product) error
	BulkCreate(ctx context.Context, products []models.Product) error
	GetByID(ctx context.Context, id uint) (*models.Product, error)
	GetByName(ctx context.Context, name string) (*models.Product, error)
	GetByNames(ctx context.Context, names []string) ([]models.Product, error)
	Update(ctx context.Context, product *models.Product) error
	UpdateStatus(ctx context.Context, id uint, status string) error
	Delete(ctx context.Context, id uint) error
	Restore(ctx context.Context, id uint) error
	GetBySupplier(ctx context.Context, supplierID uint) ([]models.Product, error)
	Search(ctx context.Context, query string, sortBy, sortOrder string) ([]models.Product, error)
	SearchWithPagination(ctx context.Context, query string, limit, offset int, sortBy, sortOrder, status, productType string, supplierID uint) ([]models.Product, error)
	Count(ctx context.Context, status, productType string, supplierID uint) (int64, error)
	CountSearch(ctx context.Context, query string, status, productType string, supplierID uint) (int64, error)
	AnyProductUsingUnitID(ctx context.Context, unitID uint) (bool, error)

	// v1 - AGENTS MUST CONFIRM BEFORE MODIFYING SECTION BELOW THIS LINE
	List(ctx context.Context, limit, offset int, sortBy, sortOrder, status, productType string, supplierID uint) ([]models.Product, error)
}

type productRepository struct {
	*baseRepository
}

func NewProductRepository(base BaseRepository) ProductRepository {
	return &productRepository{baseRepository: asBase(base)}
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
	err := r.db.WithContext(ctx).
		Preload("Unit").
		Preload("Suppliers").
		Preload("InventoryItems.Inventory").
		First(&product, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &product, nil
}

// GetByName retrieves a product by name (case-insensitive)
func (r *productRepository) GetByName(ctx context.Context, name string) (*models.Product, error) {
	var product models.Product
	err := r.db.WithContext(ctx).
		Where("LOWER(name) = LOWER(?)", name).
		First(&product).Error
	if err != nil {
		return nil, err
	}
	return &product, nil
}

func (r *productRepository) GetByNames(ctx context.Context, names []string) ([]models.Product, error) {
	if len(names) == 0 {
		return nil, nil
	}
	products := []models.Product{}
	if err := r.db.WithContext(ctx).Where("name IN ?", names).Find(&products).Error; err != nil {
		return nil, err
	}
	return products, nil
}

func (r *productRepository) Update(ctx context.Context, product *models.Product) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Load the existing product into transaction context
		var existingProduct models.Product
		if err := tx.First(&existingProduct, product.ID).Error; err != nil {
			return fmt.Errorf("failed to find product: %w", err)
		}

		// Update product fields (excluding Suppliers)
		updateMap := map[string]interface{}{
			"name":         product.Name,
			"description":  product.Description,
			"product_type": product.ProductType,
			"unit_id":      product.UnitID,
			"status":       product.Status,
		}
		// Only update product_image if it's explicitly set (not nil)
		// nil = don't update, empty slice = clear (set to NULL), has data = update
		if product.ProductImage != nil {
			if len(product.ProductImage) == 0 {
				// Empty slice means clear the image (set to NULL)
				updateMap["product_image"] = nil
			} else {
				// Has content, update with image data
				updateMap["product_image"] = product.ProductImage
			}
		}

		if err := tx.Model(&existingProduct).Omit("Suppliers").Updates(updateMap).Error; err != nil {
			return fmt.Errorf("failed to update product: %w", err)
		}

		// Replace suppliers association
		if err := tx.Model(&existingProduct).Association("Suppliers").Replace(product.Suppliers); err != nil {
			return fmt.Errorf("failed to update product suppliers: %w", err)
		}

		return nil
	})
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

func (r *productRepository) List(ctx context.Context, limit, offset int, sortBy, sortOrder, status, productType string, supplierID uint) ([]models.Product, error) {
	var products []models.Product
	query := r.db.WithContext(ctx).
		Preload("Unit").
		Preload("Unit.DerivedUnits").
		Preload("Suppliers").
		Preload("InventoryItems")

	// Apply status filter
	if status != "" {
		query = query.Where("status = ?", status)
	}

	// Apply product type filter
	if productType != "" {
		query = query.Where("product_type = ?", productType)
	}

	// Apply supplier filter
	if supplierID > 0 {
		query = query.Joins("JOIN product_suppliers ON products.id = product_suppliers.product_id").
			Where("product_suppliers.supplier_id = ?", supplierID)
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
	err := r.db.WithContext(ctx).Preload("Unit").Preload("Suppliers").Preload("InventoryItems").
		Joins("JOIN product_suppliers ON products.id = product_suppliers.product_id").
		Where("product_suppliers.supplier_id = ?", supplierID).
		Find(&products).Error
	return products, err
}

func (r *productRepository) Search(ctx context.Context, query string, sortBy, sortOrder string) ([]models.Product, error) {
	var products []models.Product
	dbQuery := r.db.WithContext(ctx).
		Preload("Unit").
		Preload("Unit.DerivedUnits").
		Preload("Suppliers").
		Preload("InventoryItems").
		Where("name ILIKE ? OR product_type ILIKE ?", "%"+query+"%", "%"+query+"%")

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

func (r *productRepository) SearchWithPagination(ctx context.Context, query string, limit, offset int, sortBy, sortOrder, status, productType string, supplierID uint) ([]models.Product, error) {
	var products []models.Product
	dbQuery := r.db.WithContext(ctx).
		Preload("Unit").
		Preload("Suppliers").
		Preload("InventoryItems").
		Where("name ILIKE ? OR product_type ILIKE ?", "%"+query+"%", "%"+query+"%")

	// Apply status filter
	if status != "" {
		dbQuery = dbQuery.Where("status = ?", status)
	}

	// Apply product type filter
	if productType != "" {
		dbQuery = dbQuery.Where("product_type = ?", productType)
	}

	// Apply supplier filter
	if supplierID > 0 {
		dbQuery = dbQuery.Joins("JOIN product_suppliers ON products.id = product_suppliers.product_id").
			Where("product_suppliers.supplier_id = ?", supplierID)
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

func (r *productRepository) Count(ctx context.Context, status, productType string, supplierID uint) (int64, error) {
	var count int64
	query := r.db.WithContext(ctx).Model(&models.Product{})

	// Apply status filter
	if status != "" {
		query = query.Where("status = ?", status)
	}

	// Apply product type filter
	if productType != "" {
		query = query.Where("product_type = ?", productType)
	}

	// Apply supplier filter
	if supplierID > 0 {
		query = query.Joins("JOIN product_suppliers ON products.id = product_suppliers.product_id").
			Where("product_suppliers.supplier_id = ?", supplierID)
	}

	err := query.Count(&count).Error
	return count, err
}

func (r *productRepository) CountSearch(ctx context.Context, query string, status, productType string, supplierID uint) (int64, error) {
	var count int64
	dbQuery := r.db.WithContext(ctx).Model(&models.Product{}).Where("name ILIKE ? OR product_type ILIKE ?", "%"+query+"%", "%"+query+"%")

	// Apply status filter
	if status != "" {
		dbQuery = dbQuery.Where("status = ?", status)
	}

	// Apply product type filter
	if productType != "" {
		dbQuery = dbQuery.Where("product_type = ?", productType)
	}

	// Apply supplier filter
	if supplierID > 0 {
		dbQuery = dbQuery.Joins("JOIN product_suppliers ON products.id = product_suppliers.product_id").
			Where("product_suppliers.supplier_id = ?", supplierID)
	}

	err := dbQuery.Count(&count).Error
	return count, err
}

func (r *productRepository) AnyProductUsingUnitID(ctx context.Context, unitID uint) (bool, error) {
	var product models.Product
	err := r.db.WithContext(ctx).
		Model(&models.Product{}).
		Where("unit_id = ?", unitID).
		First(&product).Error
	if err == nil {
		return true, nil
	}
	if err == gorm.ErrRecordNotFound {
		return false, nil
	}
	return false, fmt.Errorf("failed to check if products exist for unit_id: %w", err)
}
