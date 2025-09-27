package services

import (
	"import-export-backend/internal/models"
	"import-export-backend/internal/repository"
)

type ProductService interface {
	CreateProduct(product *models.Product) error
	GetProductByID(id uint) (*models.Product, error)
	UpdateProduct(product *models.Product) error
	DeleteProduct(id uint) error
	RestoreProduct(id uint) error
	ListProducts(limit, offset int, sortBy, sortOrder string) ([]models.Product, error)
	GetProductsBySupplier(supplierID uint) ([]models.Product, error)
	SearchProducts(query string, sortBy, sortOrder string) ([]models.Product, error)
	SearchProductsWithPagination(query string, limit, offset int, sortBy, sortOrder string) ([]models.Product, error)
	CountProducts() (int64, error)
	CountSearchProducts(query string) (int64, error)
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

func (s *productService) CreateProduct(product *models.Product) error {
	err := s.productRepo.Create(product)
	if err != nil {
		return err
	}

	// Create inventory record for the product
	inventory := &models.Inventory{
		ProductID:    product.ID,
		Quantity:     0,
		ReorderLevel: 0,
	}
	return s.inventoryRepo.Create(inventory)
}

func (s *productService) GetProductByID(id uint) (*models.Product, error) {
	return s.productRepo.GetByID(id)
}

func (s *productService) UpdateProduct(product *models.Product) error {
	return s.productRepo.Update(product)
}

func (s *productService) DeleteProduct(id uint) error {
	return s.productRepo.Delete(id)
}

func (s *productService) RestoreProduct(id uint) error {
	return s.productRepo.Restore(id)
}

func (s *productService) ListProducts(limit, offset int, sortBy, sortOrder string) ([]models.Product, error) {
	return s.productRepo.List(limit, offset, sortBy, sortOrder)
}

func (s *productService) GetProductsBySupplier(supplierID uint) ([]models.Product, error) {
	return s.productRepo.GetBySupplier(supplierID)
}

func (s *productService) SearchProducts(query string, sortBy, sortOrder string) ([]models.Product, error) {
	return s.productRepo.Search(query, sortBy, sortOrder)
}

func (s *productService) SearchProductsWithPagination(query string, limit, offset int, sortBy, sortOrder string) ([]models.Product, error) {
	return s.productRepo.SearchWithPagination(query, limit, offset, sortBy, sortOrder)
}

func (s *productService) CountProducts() (int64, error) {
	return s.productRepo.Count()
}

func (s *productService) CountSearchProducts(query string) (int64, error) {
	return s.productRepo.CountSearch(query)
}
