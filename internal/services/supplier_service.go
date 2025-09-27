package services

import (
	"import-export-backend/internal/models"
	"import-export-backend/internal/repository"
)

type SupplierService interface {
	CreateSupplier(supplier *models.Supplier) error
	GetSupplierByID(id uint) (*models.Supplier, error)
	UpdateSupplier(supplier *models.Supplier) error
	DeleteSupplier(id uint) error
	RestoreSupplier(id uint) error
	ListSuppliers(limit, offset int, sortBy, sortOrder string) ([]models.Supplier, error)
	SearchSuppliers(query string, sortBy, sortOrder string) ([]models.Supplier, error)
	SearchSuppliersWithPagination(query string, limit, offset int, sortBy, sortOrder string) ([]models.Supplier, error)
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

func (s *supplierService) GetSupplierByID(id uint) (*models.Supplier, error) {
	return s.supplierRepo.GetByID(id)
}

func (s *supplierService) UpdateSupplier(supplier *models.Supplier) error {
	return s.supplierRepo.Update(supplier)
}

func (s *supplierService) DeleteSupplier(id uint) error {
	return s.supplierRepo.Delete(id)
}

func (s *supplierService) RestoreSupplier(id uint) error {
	return s.supplierRepo.Restore(id)
}

func (s *supplierService) ListSuppliers(limit, offset int, sortBy, sortOrder string) ([]models.Supplier, error) {
	return s.supplierRepo.List(limit, offset, sortBy, sortOrder)
}

func (s *supplierService) SearchSuppliers(query string, sortBy, sortOrder string) ([]models.Supplier, error) {
	return s.supplierRepo.Search(query, sortBy, sortOrder)
}

func (s *supplierService) SearchSuppliersWithPagination(query string, limit, offset int, sortBy, sortOrder string) ([]models.Supplier, error) {
	return s.supplierRepo.SearchWithPagination(query, limit, offset, sortBy, sortOrder)
}

func (s *supplierService) CountSuppliers() (int64, error) {
	return s.supplierRepo.Count()
}

func (s *supplierService) CountSearchSuppliers(query string) (int64, error) {
	return s.supplierRepo.CountSearch(query)
}
