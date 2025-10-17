package services

import (
	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
	"context"
	"fmt"
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
	ReconcileInventory(ctx context.Context, req dto.ReconcileInventoryRequest) ([]*models.InventoryItem, error)
	DisposeInventory(ctx context.Context, req dto.DisposeInventoryRequest) ([]*models.InventoryItem, error)
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

// transactionCreator is a function that creates a new transaction for consuming inventory
type transactionCreator func(item *models.InventoryItem, purchaseTxn *models.InventoryTransaction, quantity int) *models.InventoryTransaction

// consumeFIFO is the common logic for consuming inventory items
// It accepts a map of item IDs to quantities to consumeFIFO and a transaction creator function.
// FIFO (First In, First Out) is used to consume inventory items.
func (s *inventoryService) consumeFIFO(
	ctx context.Context,
	itemIDs []uint,
	quantitiesToConsume map[uint]int,
	createTransaction transactionCreator,
) ([]*models.InventoryItem, error) {
	// Step 1: Query data and validate
	activeItems, err := s.inventoryItemRepo.GetActiveInventoryItems(ctx, itemIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get active inventory items: %w", err)
	}

	if len(activeItems) == 0 {
		return nil, pkg.NewAppError(pkg.ErrorCodeNotFound, "no active inventory items found", nil)
	}

	// Step 2: Validation - validate transaction quantities against inventory item quantities
	for _, item := range activeItems {
		if err := item.ValidateActivePurchaseTransactions(); err != nil {
			return nil, fmt.Errorf("validation failed for inventory item %d: %w", item.ID, err)
		}
	}

	// Step 3: Validate quantities to consume
	for itemID, qtyToConsume := range quantitiesToConsume {
		var item *models.InventoryItem
		for _, i := range activeItems {
			if i.ID == itemID {
				item = i
				break
			}
		}
		if item == nil {
			return nil, pkg.NewAppError(pkg.ErrorCodeNotFound,
				fmt.Sprintf("inventory item %d not found in active items", itemID), nil)
		}

		if qtyToConsume > item.Quantity {
			return nil, pkg.NewAppError(pkg.ErrorCodeValidation,
				fmt.Sprintf("quantity to consume %d exceeds available quantity %d for inventory item %d",
					qtyToConsume, item.Quantity, itemID), nil)
		}
	}

	// Step 4: Create consumption transactions for each inventory item
	var newTransactions []*models.InventoryTransaction
	var updateItems []*repository.PersistReconciliationItem
	var updateTransactions []*models.InventoryTransaction

	for _, item := range activeItems {
		qtyToConsume, exists := quantitiesToConsume[item.ID]
		if !exists {
			return nil, pkg.NewAppError(pkg.ErrorCodeValidation,
				fmt.Sprintf("no quantity specified for inventory item %d", item.ID), nil)
		}

		if qtyToConsume == 0 {
			continue
		}

		totalToConsume := qtyToConsume
		var consumingTxnID uint
		txnCount := len(item.ActivePurchaseTransactions)
		idx := 0

		for totalToConsume > 0 && idx < txnCount {
			txn := item.ActivePurchaseTransactions[idx]
			txnUnconsumedQty := txn.Quantity - txn.ConsumedQuantity

			if txnUnconsumedQty == 0 {
				idx++
				continue
			}

			// Create consumption transaction using the provided creator function
			consumeQty := min(totalToConsume, txnUnconsumedQty)
			transaction := createTransaction(item, txn, consumeQty)
			newTransactions = append(newTransactions, transaction)

			txn.ConsumedQuantity += consumeQty
			updateTransactions = append(updateTransactions, txn)

			consumingTxnID = txn.ID
			totalToConsume -= consumeQty
			idx++
		}

		updateItems = append(updateItems, &repository.PersistReconciliationItem{
			InventoryItem:    item,
			OriginalQuantity: item.Quantity,
		})

		item.Quantity -= qtyToConsume
		item.ConsumingTransactionID = consumingTxnID
	}

	// Step 5: Persist data
	if err := s.inventoryItemRepo.PersistConsumption(ctx, updateItems, updateTransactions, newTransactions); err != nil {
		return nil, fmt.Errorf("failed to persist consumption transactions and update inventory items: %w", err)
	}

	ivtrItems := make([]*models.InventoryItem, len(updateItems))
	for i, item := range updateItems {
		ivtrItems[i] = item.InventoryItem
	}
	return ivtrItems, nil
}

// ReconcileInventory reconciles inventory by creating sell transactions for the difference between previous and actual quantities
func (s *inventoryService) ReconcileInventory(ctx context.Context, req dto.ReconcileInventoryRequest) ([]*models.InventoryItem, error) {
	// Prepare item IDs
	itemIDs := make([]uint, len(req.Items))
	for i, item := range req.Items {
		itemIDs[i] = item.InventoryItemID
	}

	// Get active items to calculate quantities to consume
	activeItems, err := s.inventoryItemRepo.GetActiveInventoryItems(ctx, itemIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get active inventory items: %w", err)
	}

	// Validate prevQuantity matches current quantity and calculate quantities to consume
	quantitiesToConsume := make(map[uint]int)
	for _, reqItem := range req.Items {
		if reqItem.ActualQuantity == nil {
			return nil, pkg.ErrInvalidRequestBody(fmt.Errorf("actual quantity is required for inventory item %d", reqItem.InventoryItemID))
		}

		var item *models.InventoryItem
		for _, activeItem := range activeItems {
			if activeItem.ID == reqItem.InventoryItemID {
				item = activeItem
				break
			}
		}
		if item == nil {
			return nil, pkg.NewAppError(pkg.ErrorCodeNotFound,
				fmt.Sprintf("inventory item %d not found", reqItem.InventoryItemID), nil)
		}

		// Optimistic locking: validate that current quantity matches what frontend saw
		if item.Quantity != reqItem.PrevQuantity {
			return nil, pkg.ErrOptimisticLockConflict("inventory item", reqItem.InventoryItemID, reqItem.PrevQuantity, item.Quantity)
		}

		// Validate that actual quantity doesn't exceed previous quantity
		if *reqItem.ActualQuantity > reqItem.PrevQuantity {
			return nil, pkg.NewAppError(pkg.ErrorCodeValidation,
				fmt.Sprintf("actual quantity %d exceeds previous quantity %d for inventory item %d",
					reqItem.ActualQuantity, reqItem.PrevQuantity, reqItem.InventoryItemID), nil)
		}

		// Calculate consume quantity: prevQuantity - actualQuantity
		quantitiesToConsume[reqItem.InventoryItemID] = reqItem.PrevQuantity - *reqItem.ActualQuantity
	}

	// Create sell transaction creator
	sellTransactionCreator := func(item *models.InventoryItem, purchaseTxn *models.InventoryTransaction, quantity int) *models.InventoryTransaction {
		return &models.InventoryTransaction{
			InventoryItemID:      item.ID,
			TransactionType:      models.InventoryTransactionTypeSell,
			Price:                purchaseTxn.Price,
			Quantity:             quantity,
			CounterTransactionID: &purchaseTxn.ID,
		}
	}

	return s.consumeFIFO(ctx, itemIDs, quantitiesToConsume, sellTransactionCreator)
}

// DisposeInventory disposes inventory by creating disposal transactions for specified quantities
func (s *inventoryService) DisposeInventory(ctx context.Context, req dto.DisposeInventoryRequest) ([]*models.InventoryItem, error) {
	// Prepare item IDs
	itemIDs := make([]uint, len(req.Items))
	for i, item := range req.Items {
		itemIDs[i] = item.InventoryItemID
	}

	// Get active items to validate prevQuantity
	activeItems, err := s.inventoryItemRepo.GetActiveInventoryItems(ctx, itemIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get active inventory items: %w", err)
	}

	// Validate prevQuantity matches current quantity and prepare quantities to consume
	quantitiesToConsume := make(map[uint]int)
	for _, reqItem := range req.Items {
		var item *models.InventoryItem
		for _, activeItem := range activeItems {
			if activeItem.ID == reqItem.InventoryItemID {
				item = activeItem
				break
			}
		}
		if item == nil {
			return nil, pkg.NewAppError(pkg.ErrorCodeNotFound,
				fmt.Sprintf("inventory item %d not found", reqItem.InventoryItemID), nil)
		}

		// Optimistic locking: validate that current quantity matches what frontend saw
		if item.Quantity != reqItem.PrevQuantity {
			return nil, pkg.ErrOptimisticLockConflict("inventory item", reqItem.InventoryItemID, reqItem.PrevQuantity, item.Quantity)
		}

		// Validate that dispose quantity doesn't exceed previous quantity
		if reqItem.Quantity > reqItem.PrevQuantity {
			return nil, pkg.NewAppError(pkg.ErrorCodeValidation,
				fmt.Sprintf("dispose quantity %d exceeds previous quantity %d for inventory item %d",
					reqItem.Quantity, reqItem.PrevQuantity, reqItem.InventoryItemID), nil)
		}

		// For dispose, quantity is already the consume quantity
		quantitiesToConsume[reqItem.InventoryItemID] = reqItem.Quantity
	}

	// Create disposal transaction creator
	disposeTransactionCreator := func(item *models.InventoryItem, purchaseTxn *models.InventoryTransaction, quantity int) *models.InventoryTransaction {
		return &models.InventoryTransaction{
			InventoryItemID:      item.ID,
			TransactionType:      models.InventoryTransactionTypeDisposal,
			Price:                purchaseTxn.Price,
			Quantity:             quantity,
			CounterTransactionID: &purchaseTxn.ID,
		}
	}

	return s.consumeFIFO(ctx, itemIDs, quantitiesToConsume, disposeTransactionCreator)
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
