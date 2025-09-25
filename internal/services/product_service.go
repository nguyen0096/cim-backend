package services

import (
	"import-export-backend/internal/models"
	"import-export-backend/internal/repository"

	"github.com/google/uuid"
)

type ProductService interface {
	CreateProduct(product *models.Product) error
	GetProductByID(id uuid.UUID) (*models.Product, error)
	UpdateProduct(product *models.Product) error
	DeleteProduct(id uuid.UUID) error
	ListProducts(limit, offset int) ([]models.Product, error)
	GetProductsBySupplier(supplierID uuid.UUID) ([]models.Product, error)
	SearchProducts(query string) ([]models.Product, error)
	SearchProductsWithPagination(query string, limit, offset int) ([]models.Product, error)
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

func (s *productService) GetProductByID(id uuid.UUID) (*models.Product, error) {
	return s.productRepo.GetByID(id)
}

func (s *productService) UpdateProduct(product *models.Product) error {
	return s.productRepo.Update(product)
}

func (s *productService) DeleteProduct(id uuid.UUID) error {
	return s.productRepo.Delete(id)
}

func (s *productService) ListProducts(limit, offset int) ([]models.Product, error) {
	return s.productRepo.List(limit, offset)
}

func (s *productService) GetProductsBySupplier(supplierID uuid.UUID) ([]models.Product, error) {
	return s.productRepo.GetBySupplier(supplierID)
}

func (s *productService) SearchProducts(query string) ([]models.Product, error) {
	return s.productRepo.Search(query)
}

func (s *productService) SearchProductsWithPagination(query string, limit, offset int) ([]models.Product, error) {
	return s.productRepo.SearchWithPagination(query, limit, offset)
}

func (s *productService) CountProducts() (int64, error) {
	return s.productRepo.Count()
}

func (s *productService) CountSearchProducts(query string) (int64, error) {
	return s.productRepo.CountSearch(query)
}
