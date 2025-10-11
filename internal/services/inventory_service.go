package services

import (
	"context"
	"fmt"
	"import-export-backend/internal/models"
	"import-export-backend/internal/repository"
	"import-export-backend/internal/services/dto"
	"import-export-backend/pkg"
)

//go:generate mockery --name=InventoryService --structname=InventoryService --output=./servicemocks --outpkg=servicemocks
type InventoryService interface {
	CreateInventory(ctx context.Context, inventory *models.Inventory) error
	GetInventoryByID(ctx context.Context, id uint) (*models.Inventory, error)
	UpdateInventory(ctx context.Context, inventory *models.Inventory) error
	DeleteInventory(ctx context.Context, id uint) error
	ListInventory(ctx context.Context, limit, offset int) ([]models.Inventory, error)
	AddInventory(ctx context.Context, productID uint, quantity int, referenceID uint, referenceType, notes string) error
	RemoveInventory(ctx context.Context, productID uint, quantity int, referenceID uint, referenceType, notes string) error

	// v1
	ReconcileInventory(ctx context.Context, req dto.ConfirmInventoryRequest) error
	DisposeItems(ctx context.Context, req dto.DisposeItemsRequest) ([]*models.InventoryItem, error)
	GetLastPurchasePrices(ctx context.Context) (dto.LastPurchasePriceMap, error)
}

type inventoryService struct {
	inventoryRepo     repository.InventoryRepository
	inventoryItemRepo repository.InventoryItemRepository
	productRepo       repository.ProductRepository
}

func NewInventoryService(
	inventoryRepo repository.InventoryRepository,
	inventoryItemRepo repository.InventoryItemRepository,
	productRepo repository.ProductRepository,
) InventoryService {
	return &inventoryService{
		inventoryRepo:     inventoryRepo,
		productRepo:       productRepo,
		inventoryItemRepo: inventoryItemRepo,
	}
}

func (s *inventoryService) CreateInventory(ctx context.Context, inventory *models.Inventory) error {
	return s.inventoryRepo.Create(ctx, inventory)
}

func (s *inventoryService) ListInventory(ctx context.Context, limit, offset int) ([]models.Inventory, error) {
	return s.inventoryRepo.List(ctx, limit, offset)
}

func (s *inventoryService) GetInventoryByID(ctx context.Context, id uint) (*models.Inventory, error) {
	return s.inventoryRepo.GetByID(ctx, id)
}

func (s *inventoryService) DeleteInventory(ctx context.Context, id uint) error {
	return s.inventoryRepo.Delete(ctx, id)
}

func (s *inventoryService) UpdateInventory(ctx context.Context, inventory *models.Inventory) error {
	return s.inventoryRepo.Update(ctx, inventory)
}

func (s *inventoryService) AddInventory(ctx context.Context, productID uint, quantity int, referenceID uint, referenceType, notes string) error {
	// Create transaction record
	return s.inventoryRepo.AddInventory(ctx, productID, quantity, referenceID, referenceType)
}

func (s *inventoryService) RemoveInventory(ctx context.Context, productID uint, quantity int, referenceID uint, referenceType, notes string) error {
	return s.inventoryRepo.RemoveInventory(ctx, productID, quantity, referenceID, referenceType)
}

// DisposeInventoryItems disposes multiple inventory items by reducing their quantities
func (s *inventoryService) DisposeItems(ctx context.Context, req dto.DisposeItemsRequest) ([]*models.InventoryItem, error) {
	// Get inventory with preloaded inventory items
	inventory, err := s.inventoryRepo.GetByID(ctx, req.InventoryID)
	if err != nil {
		return nil, fmt.Errorf("failed to get inventory: %w", err)
	}
	if inventory == nil {
		return nil, pkg.NewAppError(pkg.ErrorCodeNotFound, "inventory not found", nil)
	}

	// Build a map of inventory item ID -> inventory item pointer for quick lookup
	itemMap := make(map[uint]*models.InventoryItem)
	for i := range inventory.Items {
		itemMap[inventory.Items[i].ID] = inventory.Items[i]
	}

	var updatedItems []*models.InventoryItem
	var inventoryTransactions []*models.InventoryTransaction

	// Process each item in the disposal request
	for _, disposalItem := range req.Items {
		// Find the inventory item in the map
		item, exists := itemMap[disposalItem.InventoryItemID]
		if !exists {
			return nil, pkg.NewAppError(pkg.ErrorCodeNotFound, fmt.Sprintf("inventory item %d not found", disposalItem.InventoryItemID), nil)
		}

		// Check if there's sufficient quantity to dispose
		if item.Quantity < disposalItem.Quantity {
			return nil, pkg.NewAppError(pkg.ErrorCodeValidation, fmt.Sprintf("insufficient quantity for inventory item %d. Available: %d, Requested: %d", disposalItem.InventoryItemID, item.Quantity, disposalItem.Quantity), nil)
		}

		// Reduce the quantity in memory
		item.Quantity -= disposalItem.Quantity

		// Add to updated items list
		updatedItems = append(updatedItems, item)

		// Create inventory transaction record
		transaction := models.InventoryTransaction{
			InventoryItemID: item.ID,
			TransactionType: models.InventoryTransactionTypeDisposal,
			Quantity:        -disposalItem.Quantity, // Negative for disposal
		}
		inventoryTransactions = append(inventoryTransactions, &transaction)
	}

	// Batch update inventory items and create transactions
	if err := s.inventoryItemRepo.Update(ctx, updatedItems, inventoryTransactions); err != nil {
		return nil, fmt.Errorf("failed to update inventory items and create transactions: %w", err)
	}

	return updatedItems, nil
}

func (s *inventoryService) ReconcileInventory(ctx context.Context, req dto.ConfirmInventoryRequest) error {
	itemIDs := make([]uint, len(req.Items))
	for i, item := range req.Items {
		itemIDs[i] = item.InventoryItemID
	}

	// Step 1: Query data and validate
	activeItems, err := s.inventoryItemRepo.GetActiveItemsByInventoryIDs(ctx, itemIDs)
	if err != nil {
		return fmt.Errorf("failed to get active inventory items: %w", err)
	}

	if len(activeItems) == 0 {
		return pkg.NewAppError(pkg.ErrorCodeNotFound, "no active inventory items found", nil)
	}

	// Validate transaction quantities against inventory item quantities
	for _, item := range activeItems {
		if err := item.ValidateActivePurchaseTransactions(); err != nil {
			return fmt.Errorf("validation failed for inventory item %d: %w", item.ID, err)
		}
	}

	// Step 2: Create sell transactions for each inventory item
	// based on user-provided quantities
	actualQuantities := make(map[uint]int)
	for _, reqItem := range req.Items {
		actualQuantities[reqItem.InventoryItemID] = reqItem.Quantity
	}

	var sellTransactions []*models.InventoryTransaction
	var updatedItems []*models.InventoryItem

	for _, item := range activeItems {
		actualQty, exists := actualQuantities[item.ID]
		if !exists {
			return pkg.NewAppError(pkg.ErrorCodeValidation,
				fmt.Sprintf("no quantity specified for inventory item %d", item.ID), nil)
		}

		// Validate that requested quantity doesn't exceed available quantity
		if actualQty > item.Quantity {
			return pkg.NewAppError(pkg.ErrorCodeValidation,
				fmt.Sprintf("requested quantity %d exceeds available quantity %d for inventory item %d",
					actualQty, item.Quantity, item.ID), nil)
		}

		totalToConsume := item.Quantity - actualQty
		txnCount := len(item.ActivePurchaseTransactions)
		currentConsumingIdx := 0
		for totalToConsume > 0 && currentConsumingIdx < txnCount {
			txn := item.ActivePurchaseTransactions[currentConsumingIdx]

			txnUnconsumedQty := txn.Quantity - txn.ConsumedQuantity
			if txnUnconsumedQty == 0 {
				if currentConsumingIdx == txnCount-1 {
					// no txn left to consume
					break
				}

				// move to next transaction for consuming if there's still txn
				currentConsumingIdx++
				continue
			}

			sellQty := min(totalToConsume, txnUnconsumedQty)
			if sellQty > 0 {
				sellTransaction := &models.InventoryTransaction{
					InventoryItemID:      item.ID,
					TransactionType:      models.InventoryTransactionTypeSell,
					Price:                txn.Price, // Use same price as purchase
					Quantity:             sellQty,
					CounterTransactionID: &txn.ID,
				}
				sellTransactions = append(sellTransactions, sellTransaction)
				txn.ConsumedQuantity += sellQty
			}
		}

		item.Quantity = actualQty
		item.ConsumingTransactionID = item.ActivePurchaseTransactions[currentConsumingIdx].ID
	}

	// Step 3: Persist data
	if err := s.inventoryItemRepo.PersistReconciliation(ctx, updatedItems, sellTransactions); err != nil {
		return fmt.Errorf("failed to create sell transactions and update inventory items: %w", err)
	}
	return nil
}

// GetLastPurchasePrices retrieves the last purchase transaction price for each product_id + supplier_id combination
func (s *inventoryService) GetLastPurchasePrices(ctx context.Context) (dto.LastPurchasePriceMap, error) {
	prices, err := s.inventoryRepo.GetLastPurchasePrices(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get last purchase prices: %w", err)
	}

	// Transform array into nested map: product_id -> supplier_id -> last_price
	priceMap := make(dto.LastPurchasePriceMap)
	for _, price := range prices {
		if priceMap[price.ProductID] == nil {
			priceMap[price.ProductID] = make(map[uint]float64)
		}
		priceMap[price.ProductID][price.SupplierID] = price.LastPrice
	}

	return priceMap, nil
}
