package services

import (
	"context"
	"import-export-backend/internal/models"
	"import-export-backend/internal/repository"
)

type ProductService interface {
	CreateProduct(ctx context.Context, product *models.Product) error
	GetProductByID(ctx context.Context, id uint) (*models.Product, error)
	UpdateProduct(ctx context.Context, product *models.Product) error
	UpdateProductStatus(ctx context.Context, id uint, status string) error
	DeleteProduct(ctx context.Context, id uint) error
	RestoreProduct(ctx context.Context, id uint) error
	ListProducts(ctx context.Context, limit, offset int, sortBy, sortOrder, status string) ([]models.Product, error)
	GetProductsBySupplier(ctx context.Context, supplierID uint) ([]models.Product, error)
	SearchProducts(ctx context.Context, query string, sortBy, sortOrder string) ([]models.Product, error)
	SearchProductsWithPagination(ctx context.Context, query string, limit, offset int, sortBy, sortOrder, status string) ([]models.Product, error)
	CountProducts(ctx context.Context, status string) (int64, error)
	CountSearchProducts(ctx context.Context, query string, status string) (int64, error)
}

type productService struct {
	productRepo   repository.ProductRepository
	inventoryRepo repository.InventoryRepository
}

func NewProductService(productRepo repository.ProductRepository, inventoryRepo repository.InventoryRepository) ProductService {
	return &productService{
		productRepo:   productRepo,
		inventoryRepo: inventoryRepo,
	}
}

func (s *productService) CreateProduct(ctx context.Context, product *models.Product) error {
	err := s.productRepo.Create(ctx, product)
	if err != nil {
		return err
	}

	// Create inventory record for the product
	inventory := &models.Inventory{
		ProductID:    product.ID,
		Quantity:     0,
		ReorderLevel: 0,
	}
	return s.inventoryRepo.Create(ctx, inventory)
}

func (s *productService) GetProductByID(ctx context.Context, id uint) (*models.Product, error) {
	return s.productRepo.GetByID(ctx, id)
}

func (s *productService) UpdateProduct(ctx context.Context, product *models.Product) error {
	return s.productRepo.Update(ctx, product)
}

// UpdateProductStatus updates the status of a product
func (s *productService) UpdateProductStatus(ctx context.Context, id uint, status string) error {
	return s.productRepo.UpdateStatus(ctx, id, status)
}

func (s *productService) DeleteProduct(ctx context.Context, id uint) error {
	return s.productRepo.Delete(ctx, id)
}

func (s *productService) RestoreProduct(ctx context.Context, id uint) error {
	return s.productRepo.Restore(ctx, id)
}

func (s *productService) ListProducts(ctx context.Context, limit, offset int, sortBy, sortOrder, status string) ([]models.Product, error) {
	return s.productRepo.List(ctx, limit, offset, sortBy, sortOrder, status)
}

func (s *productService) GetProductsBySupplier(ctx context.Context, supplierID uint) ([]models.Product, error) {
	return s.productRepo.GetBySupplier(ctx, supplierID)
}

func (s *productService) SearchProducts(ctx context.Context, query string, sortBy, sortOrder string) ([]models.Product, error) {
	return s.productRepo.Search(ctx, query, sortBy, sortOrder)
}

func (s *productService) SearchProductsWithPagination(ctx context.Context, query string, limit, offset int, sortBy, sortOrder, status string) ([]models.Product, error) {
	return s.productRepo.SearchWithPagination(ctx, query, limit, offset, sortBy, sortOrder, status)
}

func (s *productService) CountProducts(ctx context.Context, status string) (int64, error) {
	return s.productRepo.Count(ctx, status)
}

func (s *productService) CountSearchProducts(ctx context.Context, query string, status string) (int64, error) {
	return s.productRepo.CountSearch(ctx, query, status)
}
