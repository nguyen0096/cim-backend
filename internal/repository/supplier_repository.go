package repository

import (
	"context"
	"import-export-backend/internal/models"

	"gorm.io/gorm"
)

type SupplierRepository interface {
	Create(ctx context.Context, supplier *models.Supplier) error
	GetByID(ctx context.Context, id uint) (*models.Supplier, error)
	Update(ctx context.Context, supplier *models.Supplier) error
	Delete(ctx context.Context, id uint) error
	Restore(ctx context.Context, id uint) error
	List(ctx context.Context, limit, offset int, sortBy, sortOrder string) ([]models.Supplier, error)
	Search(ctx context.Context, query string, sortBy, sortOrder string) ([]models.Supplier, error)
	SearchWithPagination(ctx context.Context, query string, limit, offset int, sortBy, sortOrder string) ([]models.Supplier, error)
	Count(ctx context.Context) (int64, error)
	CountSearch(ctx context.Context, query string) (int64, error)
}

type supplierRepository struct {
	db *gorm.DB
}

func NewSupplierRepository(db *gorm.DB) SupplierRepository {
	return &supplierRepository{db: db}
}

func (r *supplierRepository) Create(ctx context.Context, supplier *models.Supplier) error {
	return r.db.WithContext(ctx).Create(supplier).Error
}

func (r *supplierRepository) GetByID(ctx context.Context, id uint) (*models.Supplier, error) {
	var supplier models.Supplier
	err := r.db.WithContext(ctx).First(&supplier, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &supplier, nil
}

func (r *supplierRepository) Update(ctx context.Context, supplier *models.Supplier) error {
	return r.db.WithContext(ctx).Save(supplier).Error
}

func (r *supplierRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.Supplier{}, "id = ?", id).Error
}

func (r *supplierRepository) Restore(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Unscoped().Model(&models.Supplier{}).Where("id = ?", id).Update("deleted_at", nil).Error
}

func (r *supplierRepository) List(ctx context.Context, limit, offset int, sortBy, sortOrder string) ([]models.Supplier, error) {
	var suppliers []models.Supplier
	query := r.db.WithContext(ctx)

	// Apply sorting
	if sortBy != "" {
		if sortOrder == "" {
			sortOrder = "asc"
		}
		query = query.Order(sortBy + " " + sortOrder)
	} else {
		query = query.Order("created_at desc")
	}

	err := query.Limit(limit).Offset(offset).Find(&suppliers).Error
	return suppliers, err
}

func (r *supplierRepository) Search(ctx context.Context, query string, sortBy, sortOrder string) ([]models.Supplier, error) {
	var suppliers []models.Supplier
	dbQuery := r.db.WithContext(ctx).Where("name ILIKE ? OR contact_phone ILIKE ? OR contact_email ILIKE ?",
		"%"+query+"%", "%"+query+"%", "%"+query+"%")

	// Apply sorting
	if sortBy != "" {
		if sortOrder == "" {
			sortOrder = "asc"
		}
		dbQuery = dbQuery.Order(sortBy + " " + sortOrder)
	} else {
		dbQuery = dbQuery.Order("created_at desc")
	}

	err := dbQuery.Find(&suppliers).Error
	return suppliers, err
}

func (r *supplierRepository) SearchWithPagination(ctx context.Context, query string, limit, offset int, sortBy, sortOrder string) ([]models.Supplier, error) {
	var suppliers []models.Supplier
	dbQuery := r.db.WithContext(ctx).Where("name ILIKE ? OR contact_phone ILIKE ? OR contact_email ILIKE ?",
		"%"+query+"%", "%"+query+"%", "%"+query+"%")

	// Apply sorting
	if sortBy != "" {
		if sortOrder == "" {
			sortOrder = "asc"
		}
		dbQuery = dbQuery.Order(sortBy + " " + sortOrder)
	} else {
		dbQuery = dbQuery.Order("created_at desc")
	}

	err := dbQuery.Limit(limit).Offset(offset).Find(&suppliers).Error
	return suppliers, err
}

func (r *supplierRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.Supplier{}).Count(&count).Error
	return count, err
}

func (r *supplierRepository) CountSearch(ctx context.Context, query string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.Supplier{}).Where("name ILIKE ? OR contact_phone ILIKE ? OR contact_email ILIKE ?",
		"%"+query+"%", "%"+query+"%", "%"+query+"%").Count(&count).Error
	return count, err
}
