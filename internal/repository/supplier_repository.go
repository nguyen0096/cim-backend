package repository

import (
	"cim-backend/internal/models"
	"context"

	"gorm.io/gorm"
)

//go:generate mockery --name=SupplierRepository --structname=SupplierRepository --output=../mocks/repositorymocks --outpkg=repositorymocks
type SupplierRepository interface {
	Create(ctx context.Context, supplier *models.Supplier) error
	GetByID(ctx context.Context, id uint) (*models.Supplier, error)
	GetByName(ctx context.Context, name string) (*models.Supplier, error)
	FindOrCreateByName(ctx context.Context, supplier *models.Supplier) (*models.Supplier, error)
	Update(ctx context.Context, supplier *models.Supplier) error
	UpdateStatus(ctx context.Context, id uint, status string) error
	Delete(ctx context.Context, id uint) error
	Restore(ctx context.Context, id uint) error
	List(ctx context.Context, limit, offset int, sortBy, sortOrder, status string) ([]models.Supplier, error)
	Search(ctx context.Context, query string, sortBy, sortOrder string) ([]models.Supplier, error)
	SearchWithPagination(ctx context.Context, query string, limit, offset int, sortBy, sortOrder, status string) ([]models.Supplier, error)
	Count(ctx context.Context, status string) (int64, error)
	CountSearch(ctx context.Context, query string, status string) (int64, error)
}

type supplierRepository struct {
	*baseRepository
}

func NewSupplierRepository(base BaseRepository) SupplierRepository {
	return &supplierRepository{baseRepository: asBase(base)}
}

func (r *supplierRepository) Create(ctx context.Context, supplier *models.Supplier) error {
	return r.db.WithContext(ctx).Create(supplier).Error
}

func (r *supplierRepository) GetByID(ctx context.Context, id uint) (*models.Supplier, error) {
	var supplier models.Supplier
	err := r.db.WithContext(ctx).Preload("Products").First(&supplier, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &supplier, nil
}

// GetByName retrieves a supplier by name
func (r *supplierRepository) GetByName(ctx context.Context, name string) (*models.Supplier, error) {
	var supplier models.Supplier
	err := r.db.WithContext(ctx).Where("name = ?", name).First(&supplier).Error
	if err != nil {
		return nil, err
	}
	return &supplier, nil
}

// FindOrCreateByName finds an existing supplier by name or creates a new one
func (r *supplierRepository) FindOrCreateByName(ctx context.Context, supplier *models.Supplier) (*models.Supplier, error) {
	var existing models.Supplier
	err := r.db.WithContext(ctx).Where("name = ?", supplier.Name).First(&existing).Error
	if err == nil {
		// Supplier exists, update fields if provided
		if supplier.ContactEmail != "" {
			existing.ContactEmail = supplier.ContactEmail
		}
		if supplier.ContactPhone != "" {
			existing.ContactPhone = supplier.ContactPhone
		}
		if supplier.Address != "" {
			existing.Address = supplier.Address
		}
		if err := r.db.WithContext(ctx).Save(&existing).Error; err != nil {
			return nil, err
		}
		return &existing, nil
	}

	if err == gorm.ErrRecordNotFound {
		// Create new supplier
		if err := r.db.WithContext(ctx).Create(supplier).Error; err != nil {
			return nil, err
		}
		return supplier, nil
	}

	return nil, err
}

func (r *supplierRepository) Update(ctx context.Context, supplier *models.Supplier) error {
	return r.db.WithContext(ctx).Where("id = ?", supplier.ID).Omit("Base").Updates(supplier).Error
}

// UpdateStatus updates the status of a supplier
func (r *supplierRepository) UpdateStatus(ctx context.Context, id uint, status string) error {
	return r.db.WithContext(ctx).Model(&models.Supplier{}).Where("id = ?", id).Update("status", status).Error
}

func (r *supplierRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.Supplier{}, "id = ?", id).Error
}

func (r *supplierRepository) Restore(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Unscoped().Model(&models.Supplier{}).Where("id = ?", id).Update("deleted_at", nil).Error
}

func (r *supplierRepository) List(ctx context.Context, limit, offset int, sortBy, sortOrder, status string) ([]models.Supplier, error) {
	var suppliers []models.Supplier
	query := r.db.WithContext(ctx)

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

	err := query.Limit(limit).Offset(offset).Find(&suppliers).Error
	return suppliers, err
}

func (r *supplierRepository) Search(ctx context.Context, query string, sortBy, sortOrder string) ([]models.Supplier, error) {
	var suppliers []models.Supplier
	dbQuery := r.db.WithContext(ctx).Preload("Products").Where("name ILIKE ? OR contact_phone ILIKE ? OR contact_email ILIKE ?",
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

func (r *supplierRepository) SearchWithPagination(ctx context.Context, query string, limit, offset int, sortBy, sortOrder, status string) ([]models.Supplier, error) {
	var suppliers []models.Supplier
	dbQuery := r.db.WithContext(ctx).Preload("Products").Where("name ILIKE ? OR contact_phone ILIKE ? OR contact_email ILIKE ?",
		"%"+query+"%", "%"+query+"%", "%"+query+"%")

	// Apply status filter
	if status != "" {
		dbQuery = dbQuery.Where("status = ?", status)
	} else {
		dbQuery = dbQuery.Where("status = ?", "active")
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

	err := dbQuery.Limit(limit).Offset(offset).Find(&suppliers).Error
	return suppliers, err
}

func (r *supplierRepository) Count(ctx context.Context, status string) (int64, error) {
	var count int64
	query := r.db.WithContext(ctx).Model(&models.Supplier{})

	// Apply status filter
	if status != "" {
		query = query.Where("status = ?", status)
	} else {
		query = query.Where("status = ?", "active")
	}

	err := query.Count(&count).Error
	return count, err
}

func (r *supplierRepository) CountSearch(ctx context.Context, query string, status string) (int64, error) {
	var count int64
	dbQuery := r.db.WithContext(ctx).Model(&models.Supplier{}).Where("name ILIKE ? OR contact_phone ILIKE ? OR contact_email ILIKE ?",
		"%"+query+"%", "%"+query+"%", "%"+query+"%")

	// Apply status filter
	if status != "" {
		dbQuery = dbQuery.Where("status = ?", status)
	} else {
		dbQuery = dbQuery.Where("status = ?", "active")
	}

	err := dbQuery.Count(&count).Error
	return count, err
}
