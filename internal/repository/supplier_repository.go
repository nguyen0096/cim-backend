package repository

import (
	"import-export-backend/internal/models"

	"gorm.io/gorm"
)

type SupplierRepository interface {
	Create(supplier *models.Supplier) error
	GetByID(id uint) (*models.Supplier, error)
	Update(supplier *models.Supplier) error
	Delete(id uint) error
	Restore(id uint) error
	List(limit, offset int, sortBy, sortOrder string) ([]models.Supplier, error)
	Search(query string, sortBy, sortOrder string) ([]models.Supplier, error)
	SearchWithPagination(query string, limit, offset int, sortBy, sortOrder string) ([]models.Supplier, error)
	Count() (int64, error)
	CountSearch(query string) (int64, error)
}

type supplierRepository struct {
	db *gorm.DB
}

func NewSupplierRepository(db *gorm.DB) SupplierRepository {
	return &supplierRepository{db: db}
}

func (r *supplierRepository) Create(supplier *models.Supplier) error {
	return r.db.Create(supplier).Error
}

func (r *supplierRepository) GetByID(id uint) (*models.Supplier, error) {
	var supplier models.Supplier
	err := r.db.First(&supplier, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &supplier, nil
}

func (r *supplierRepository) Update(supplier *models.Supplier) error {
	return r.db.Save(supplier).Error
}

func (r *supplierRepository) Delete(id uint) error {
	return r.db.Delete(&models.Supplier{}, "id = ?", id).Error
}

func (r *supplierRepository) Restore(id uint) error {
	return r.db.Unscoped().Model(&models.Supplier{}).Where("id = ?", id).Update("deleted_at", nil).Error
}

func (r *supplierRepository) List(limit, offset int, sortBy, sortOrder string) ([]models.Supplier, error) {
	var suppliers []models.Supplier
	query := r.db

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

func (r *supplierRepository) Search(query string, sortBy, sortOrder string) ([]models.Supplier, error) {
	var suppliers []models.Supplier
	dbQuery := r.db.Where("name ILIKE ? OR contact_phone ILIKE ? OR contact_email ILIKE ?",
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

func (r *supplierRepository) SearchWithPagination(query string, limit, offset int, sortBy, sortOrder string) ([]models.Supplier, error) {
	var suppliers []models.Supplier
	dbQuery := r.db.Where("name ILIKE ? OR contact_phone ILIKE ? OR contact_email ILIKE ?",
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

func (r *supplierRepository) Count() (int64, error) {
	var count int64
	err := r.db.Model(&models.Supplier{}).Count(&count).Error
	return count, err
}

func (r *supplierRepository) CountSearch(query string) (int64, error) {
	var count int64
	err := r.db.Model(&models.Supplier{}).Where("name ILIKE ? OR contact_phone ILIKE ? OR contact_email ILIKE ?",
		"%"+query+"%", "%"+query+"%", "%"+query+"%").Count(&count).Error
	return count, err
}
