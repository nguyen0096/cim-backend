package services

import (
	"context"
	"import-export-backend/internal/models"
	"import-export-backend/internal/repository"
)

type SupplierService interface {
	CreateSupplier(ctx context.Context, supplier *models.Supplier) error
	GetSupplierByID(ctx context.Context, id uint) (*models.Supplier, error)
	UpdateSupplier(ctx context.Context, supplier *models.Supplier) error
	DeleteSupplier(ctx context.Context, id uint) error
	RestoreSupplier(ctx context.Context, id uint) error
	ListSuppliers(ctx context.Context, limit, offset int, sortBy, sortOrder string) ([]models.Supplier, error)
	SearchSuppliers(ctx context.Context, query string, sortBy, sortOrder string) ([]models.Supplier, error)
	SearchSuppliersWithPagination(ctx context.Context, query string, limit, offset int, sortBy, sortOrder string) ([]models.Supplier, error)
	CountSuppliers(ctx context.Context) (int64, error)
	CountSearchSuppliers(ctx context.Context, query string) (int64, error)
}

type supplierService struct {
	supplierRepo repository.SupplierRepository
}

func NewSupplierService(supplierRepo repository.SupplierRepository) SupplierService {
	return &supplierService{
		supplierRepo: supplierRepo,
	}
}

func (s *supplierService) CreateSupplier(ctx context.Context, supplier *models.Supplier) error {
	return s.supplierRepo.Create(ctx, supplier)
}

func (s *supplierService) GetSupplierByID(ctx context.Context, id uint) (*models.Supplier, error) {
	return s.supplierRepo.GetByID(ctx, id)
}

func (s *supplierService) UpdateSupplier(ctx context.Context, supplier *models.Supplier) error {
	return s.supplierRepo.Update(ctx, supplier)
}

func (s *supplierService) DeleteSupplier(ctx context.Context, id uint) error {
	return s.supplierRepo.Delete(ctx, id)
}

func (s *supplierService) RestoreSupplier(ctx context.Context, id uint) error {
	return s.supplierRepo.Restore(ctx, id)
}

func (s *supplierService) ListSuppliers(ctx context.Context, limit, offset int, sortBy, sortOrder string) ([]models.Supplier, error) {
	return s.supplierRepo.List(ctx, limit, offset, sortBy, sortOrder)
}

func (s *supplierService) SearchSuppliers(ctx context.Context, query string, sortBy, sortOrder string) ([]models.Supplier, error) {
	return s.supplierRepo.Search(ctx, query, sortBy, sortOrder)
}

func (s *supplierService) SearchSuppliersWithPagination(ctx context.Context, query string, limit, offset int, sortBy, sortOrder string) ([]models.Supplier, error) {
	return s.supplierRepo.SearchWithPagination(ctx, query, limit, offset, sortBy, sortOrder)
}

func (s *supplierService) CountSuppliers(ctx context.Context) (int64, error) {
	return s.supplierRepo.Count(ctx)
}

func (s *supplierService) CountSearchSuppliers(ctx context.Context, query string) (int64, error) {
	return s.supplierRepo.CountSearch(ctx, query)
}
