package repository

import (
	"import-export-backend/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SupplierRepository interface {
	Create(supplier *models.Supplier) error
	GetByID(id uuid.UUID) (*models.Supplier, error)
	Update(supplier *models.Supplier) error
	Delete(id uuid.UUID) error
	List(limit, offset int) ([]models.Supplier, error)
	Search(query string) ([]models.Supplier, error)
	Count() (int64, error)
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

func (r *supplierRepository) GetByID(id uuid.UUID) (*models.Supplier, error) {
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

func (r *supplierRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&models.Supplier{}, "id = ?", id).Error
}

func (r *supplierRepository) List(limit, offset int) ([]models.Supplier, error) {
	var suppliers []models.Supplier
	err := r.db.Limit(limit).Offset(offset).Find(&suppliers).Error
	return suppliers, err
}

func (r *supplierRepository) Search(query string) ([]models.Supplier, error) {
	var suppliers []models.Supplier
	err := r.db.Where("name ILIKE ?", "%"+query+"%").Find(&suppliers).Error
	return suppliers, err
}

func (r *supplierRepository) Count() (int64, error) {
	var count int64
	err := r.db.Model(&models.Supplier{}).Count(&count).Error
	return count, err
}
