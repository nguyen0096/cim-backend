package services

import (
	"import-export-backend/internal/models"
	"import-export-backend/internal/repository"

	"github.com/google/uuid"
)

type SupplierService interface {
	CreateSupplier(supplier *models.Supplier) error
	GetSupplierByID(id uuid.UUID) (*models.Supplier, error)
	UpdateSupplier(supplier *models.Supplier) error
	DeleteSupplier(id uuid.UUID) error
	ListSuppliers(limit, offset int) ([]models.Supplier, error)
	SearchSuppliers(query string) ([]models.Supplier, error)
	SearchSuppliersWithPagination(query string, limit, offset int) ([]models.Supplier, error)
	CountSuppliers() (int64, error)
	CountSearchSuppliers(query string) (int64, error)
}

type supplierService struct {
	supplierRepo repository.SupplierRepository
}

func NewSupplierService(supplierRepo repository.SupplierRepository) SupplierService {
	return &supplierService{
		supplierRepo: supplierRepo,
	}
}

func (s *supplierService) CreateSupplier(supplier *models.Supplier) error {
	return s.supplierRepo.Create(supplier)
}

func (s *supplierService) GetSupplierByID(id uuid.UUID) (*models.Supplier, error) {
	return s.supplierRepo.GetByID(id)
}

func (s *supplierService) UpdateSupplier(supplier *models.Supplier) error {
	return s.supplierRepo.Update(supplier)
}

func (s *supplierService) DeleteSupplier(id uuid.UUID) error {
	return s.supplierRepo.Delete(id)
}

func (s *supplierService) ListSuppliers(limit, offset int) ([]models.Supplier, error) {
	return s.supplierRepo.List(limit, offset)
}

func (s *supplierService) SearchSuppliers(query string) ([]models.Supplier, error) {
	return s.supplierRepo.Search(query)
}

func (s *supplierService) SearchSuppliersWithPagination(query string, limit, offset int) ([]models.Supplier, error) {
	return s.supplierRepo.SearchWithPagination(query, limit, offset)
}

func (s *supplierService) CountSuppliers() (int64, error) {
	return s.supplierRepo.Count()
}

func (s *supplierService) CountSearchSuppliers(query string) (int64, error) {
	return s.supplierRepo.CountSearch(query)
}
