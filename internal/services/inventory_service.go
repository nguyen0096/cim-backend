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
	CreateReconcileSubmission(ctx context.Context, req dto.ReconcileInventoryRequest) (*models.InventorySubmission, error)
	CreateDisposeSubmission(ctx context.Context, req dto.DisposeInventoryRequest) (*models.InventorySubmission, error)
	GetPendingSubmissions(ctx context.Context, inventoryID uint) ([]dto.PendingSubmissionResponse, error)
	ApproveSubmission(ctx context.Context, req dto.ProcessSubmissionRequest) (*models.InventorySubmission, error)
	GetLastPurchasePrices(ctx context.Context) (dto.LastPurchasePriceMap, error)
}

type inventoryService struct {
	inventoryRepo           repository.InventoryRepository
	inventoryItemRepo       repository.InventoryItemRepository
	inventorySubmissionRepo repository.InventorySubmissionRepository
	productRepo             repository.ProductRepository
}

func NewInventoryService(
	inventoryRepo repository.InventoryRepository,
	inventoryItemRepo repository.InventoryItemRepository,
	inventorySubmissionRepo repository.InventorySubmissionRepository,
	productRepo repository.ProductRepository,
) InventoryService {
	return &inventoryService{
		inventoryRepo:           inventoryRepo,
		productRepo:             productRepo,
		inventoryItemRepo:       inventoryItemRepo,
		inventorySubmissionRepo: inventorySubmissionRepo,
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

// reconcileInventory reconciles inventory by creating sell transactions for the difference between previous and actual quantities
func (s *inventoryService) reconcileInventory(ctx context.Context, req dto.ReconcileInventoryRequest) error {
	// Prepare item IDs
	itemIDs := make([]uint, len(req.Items))
	for i, item := range req.Items {
		itemIDs[i] = item.InventoryItemID
	}

	// Get active items to calculate quantities to consume
	activeItems, err := s.inventoryItemRepo.GetActiveInventoryItems(ctx, itemIDs)
	if err != nil {
		return fmt.Errorf("failed to get active inventory items: %w", err)
	}

	// Validate prevQuantity matches current quantity and calculate quantities to consume
	quantitiesToConsume := make(map[uint]int)
	for _, reqItem := range req.Items {
		if reqItem.ActualQuantity == nil {
			return pkg.ErrInvalidRequestBody(fmt.Errorf("actual quantity is required for inventory item %d", reqItem.InventoryItemID))
		}

		var item *models.InventoryItem
		for _, activeItem := range activeItems {
			if activeItem.ID == reqItem.InventoryItemID {
				item = activeItem
				break
			}
		}
		if item == nil {
			return pkg.NewAppError(pkg.ErrorCodeNotFound,
				fmt.Sprintf("inventory item %d not found", reqItem.InventoryItemID), nil)
		}

		// Optimistic locking: validate that current quantity matches what frontend saw
		if item.Quantity != reqItem.PrevQuantity {
			return pkg.ErrOptimisticLockConflict("inventory item", reqItem.InventoryItemID, reqItem.PrevQuantity, item.Quantity)
		}

		// Validate that actual quantity doesn't exceed previous quantity
		if *reqItem.ActualQuantity > reqItem.PrevQuantity {
			return pkg.NewAppError(pkg.ErrorCodeValidation,
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

	_, err = s.consumeFIFO(ctx, itemIDs, quantitiesToConsume, sellTransactionCreator)
	if err != nil {
		return fmt.Errorf("failed to consume FIFO: %w", err)
	}

	return nil
}

// CreateReconcileSubmission creates a submission for reconciling inventory
func (s *inventoryService) CreateReconcileSubmission(ctx context.Context, req dto.ReconcileInventoryRequest) (*models.InventorySubmission, error) {
	// Prepare item IDs
	itemIDs := make([]uint, len(req.Items))
	for i, item := range req.Items {
		itemIDs[i] = item.InventoryItemID
	}

	// Get active items to validate the request
	activeItems, err := s.inventoryItemRepo.GetActiveInventoryItems(ctx, itemIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get active inventory items: %w", err)
	}

	// Validate prevQuantity matches current quantity
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
	}

	// Clear prevQuantity from items before persisting (used only for validation)
	submissionItems := make([]dto.ReconcileItem, len(req.Items))
	for i, item := range req.Items {
		submissionItems[i] = dto.ReconcileItem{
			InventoryItemID: item.InventoryItemID,
			ActualQuantity:  item.ActualQuantity,
			PrevQuantity:    0, // Clear prevQuantity, not needed in DB
		}
	}

	// Create inventory submission with pending status
	submission := &models.InventorySubmission{
		InventoryID:      req.InventoryID,
		SubmissionType:   models.InventorySubmissionTypeReconcile,
		ProcessingStatus: models.InventorySubmissionStatusPending,
		Payload: dto.ReconcileSubmissionPayload{
			Items: submissionItems,
		},
	}

	if err := s.inventorySubmissionRepo.Create(ctx, submission); err != nil {
		return nil, fmt.Errorf("failed to create reconcile submission: %w", err)
	}

	return submission, nil
}

// disposeInventory disposes inventory by creating disposal transactions for specified quantities
func (s *inventoryService) disposeInventory(ctx context.Context, req dto.DisposeInventoryRequest) error {
	// Prepare item IDs
	itemIDs := make([]uint, len(req.Items))
	for i, item := range req.Items {
		itemIDs[i] = item.InventoryItemID
	}

	// Get active items to validate prevQuantity
	activeItems, err := s.inventoryItemRepo.GetActiveInventoryItems(ctx, itemIDs)
	if err != nil {
		return fmt.Errorf("failed to get active inventory items: %w", err)
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
			return pkg.NewAppError(pkg.ErrorCodeNotFound,
				fmt.Sprintf("inventory item %d not found", reqItem.InventoryItemID), nil)
		}

		// Validate that dispose quantity doesn't exceed previous quantity
		if reqItem.Quantity > reqItem.PrevQuantity {
			return pkg.NewAppError(pkg.ErrorCodeValidation,
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

	_, err = s.consumeFIFO(ctx, itemIDs, quantitiesToConsume, disposeTransactionCreator)
	if err != nil {
		return fmt.Errorf("failed to consume FIFO: %w", err)
	}

	return nil
}

// CreateDisposeSubmission creates a submission for disposing inventory
func (s *inventoryService) CreateDisposeSubmission(ctx context.Context, req dto.DisposeInventoryRequest) (*models.InventorySubmission, error) {
	// Prepare item IDs
	itemIDs := make([]uint, len(req.Items))
	for i, item := range req.Items {
		itemIDs[i] = item.InventoryItemID
	}

	// Get active items to validate the request
	activeItems, err := s.inventoryItemRepo.GetActiveInventoryItems(ctx, itemIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get active inventory items: %w", err)
	}

	// Validate prevQuantity matches current quantity
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
	}

	// Clear prevQuantity from items before persisting (used only for validation)
	submissionItems := make([]dto.DisposeItem, len(req.Items))
	for i, item := range req.Items {
		submissionItems[i] = dto.DisposeItem{
			InventoryItemID: item.InventoryItemID,
			Quantity:        item.Quantity,
			PrevQuantity:    0, // Clear prevQuantity, not needed in DB
		}
	}

	// Create inventory submission with pending status
	submission := &models.InventorySubmission{
		InventoryID:      req.InventoryID,
		SubmissionType:   models.InventorySubmissionTypeDispose,
		ProcessingStatus: models.InventorySubmissionStatusPending,
		Payload: dto.DisposeSubmissionPayload{
			Items: submissionItems,
		},
	}

	if err := s.inventorySubmissionRepo.Create(ctx, submission); err != nil {
		return nil, fmt.Errorf("failed to create dispose submission: %w", err)
	}

	return submission, nil
}

// extractInventoryItemIDsFromPayload extracts inventory item IDs from a payload's items array
func extractInventoryItemIDsFromPayload(payload any) []uint {
	var itemIDs []uint
	if items, ok := payload.(map[string]interface{})["items"].([]interface{}); ok {
		for _, item := range items {
			if itemMap, ok := item.(map[string]interface{}); ok {
				if itemID, ok := itemMap["inventory_item_id"].(float64); ok {
					itemIDs = append(itemIDs, uint(itemID))
				}
			}
		}
	}
	return itemIDs
}

// buildSimplifiedItems builds simplified item summaries from payload
func buildSimplifiedItems(payload any, itemsMap map[uint]*models.InventoryItem) []dto.PendingSubmissionItemSummary {
	payloadMap, ok := payload.(map[string]interface{})
	if !ok {
		return []dto.PendingSubmissionItemSummary{}
	}

	items, ok := payloadMap["items"].([]interface{})
	if !ok {
		return []dto.PendingSubmissionItemSummary{}
	}

	summaries := make([]dto.PendingSubmissionItemSummary, 0, len(items))
	for _, item := range items {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		inventoryItemID, ok := itemMap["inventory_item_id"].(float64)
		if !ok {
			continue
		}

		inventoryItem, exists := itemsMap[uint(inventoryItemID)]
		if !exists || inventoryItem.Product == nil {
			continue
		}

		// Extract quantity based on submission type (actual_quantity for reconcile, quantity for dispose)
		var quantity int
		if actualQty, ok := itemMap["actual_quantity"].(float64); ok {
			quantity = int(actualQty)
		} else if qty, ok := itemMap["quantity"].(float64); ok {
			quantity = int(qty)
		}

		summaries = append(summaries, dto.PendingSubmissionItemSummary{
			ProductName: inventoryItem.Product.Name,
			Quantity:    quantity,
		})
	}

	return summaries
}

// GetPendingSubmissions retrieves all pending submissions for an inventory and populates inventory items
func (s *inventoryService) GetPendingSubmissions(ctx context.Context, inventoryID uint) ([]dto.PendingSubmissionResponse, error) {
	submissions, err := s.inventorySubmissionRepo.GetPendingSubmissions(ctx, inventoryID)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending submissions: %w", err)
	}

	if len(submissions) == 0 {
		return []dto.PendingSubmissionResponse{}, nil
	}

	// Extract all inventory item IDs from all submissions
	itemIDsSet := make(map[uint]struct{})
	for _, submission := range submissions {
		itemIDs := extractInventoryItemIDsFromPayload(submission.Payload)
		for _, id := range itemIDs {
			itemIDsSet[id] = struct{}{}
		}
	}

	// Convert set to slice
	itemIDs := make([]uint, 0, len(itemIDsSet))
	for id := range itemIDsSet {
		itemIDs = append(itemIDs, id)
	}

	// Query all inventory items with products in a single call
	itemsMap := make(map[uint]*models.InventoryItem)
	if len(itemIDs) > 0 {
		inventoryItems, err := s.inventoryItemRepo.GetByIDs(ctx, itemIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to get inventory items: %w", err)
		}

		// Create a map for O(1) lookup
		for _, item := range inventoryItems {
			itemsMap[item.ID] = item
		}
	}

	responses := make([]dto.PendingSubmissionResponse, len(submissions))
	for i, submission := range submissions {
		responses[i] = dto.PendingSubmissionResponse{
			ID:             submission.ID,
			InventoryID:    submission.InventoryID,
			SubmissionType: string(submission.SubmissionType),
			Status:         string(submission.ProcessingStatus),
			Items:          buildSimplifiedItems(submission.Payload, itemsMap),
			CreatedBy:      submission.CreatedBy,
			CreatedAt:      submission.CreatedAt.Format(pkg.DateTimeFormat),
			UpdatedBy:      submission.UpdatedBy,
			UpdatedAt:      submission.UpdatedAt.Format(pkg.DateTimeFormat),
		}
	}

	return responses, nil
}

// ApproveSubmission approves or rejects a pending inventory submission
func (s *inventoryService) ApproveSubmission(ctx context.Context, req dto.ProcessSubmissionRequest) (*models.InventorySubmission, error) {
	// Check permissions
	if !pkg.HasPermission(ctx, pkg.RBACResourceInventorySubmissions, pkg.RBACActionApprove) {
		return nil, pkg.NewAppError(pkg.ErrorCodeForbidden, "user does not have permission to approve inventory submissions", nil)
	}

	// Get submission
	submission, err := s.inventorySubmissionRepo.GetByID(ctx, req.SubmissionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get submission: %w", err)
	}

	// Verify submission approval is pending
	if submission.ApprovalStatus != models.InventorySubmissionApprovalStatusPending {
		return nil, pkg.NewAppError(pkg.ErrorCodeValidation,
			fmt.Sprintf("submission approval is not pending, current approval status: %s", submission.ApprovalStatus), nil)
	}

	// Determine approval status based on action
	var approvalStatus models.InventorySubmissionApprovalStatus
	switch models.InventorySubmissionAction(req.Action) {
	case models.InventorySubmissionActionApprove:
		approvalStatus = models.InventorySubmissionApprovalStatusApproved
	case models.InventorySubmissionActionReject:
		approvalStatus = models.InventorySubmissionApprovalStatusRejected
	default:
		return nil, pkg.NewAppError(pkg.ErrorCodeValidation,
			fmt.Sprintf("invalid action: %s", req.Action), nil)
	}

	// Step 1: Persist approval status first
	if err := s.inventorySubmissionRepo.UpdateApprovalStatus(ctx, submission.ID, approvalStatus); err != nil {
		return nil, fmt.Errorf("failed to update approval status: %w", err)
	}
	submission.ApprovalStatus = approvalStatus

	// Step 2: Process submission only if approved
	if approvalStatus == models.InventorySubmissionApprovalStatusApproved {
		if err := s.processSubmission(ctx, submission); err != nil {
			// Mark processing as failed if execution fails
			if updateErr := s.inventorySubmissionRepo.UpdateProcessingStatus(ctx, submission.ID, models.InventorySubmissionStatusFailed); updateErr != nil {
				return nil, fmt.Errorf("failed to process submission and update processing status: %w, update error: %v", err, updateErr)
			}
			submission.ProcessingStatus = models.InventorySubmissionStatusFailed
			return nil, fmt.Errorf("failed to process submission: %w", err)
		}

		// Update processing status to completed
		if err := s.inventorySubmissionRepo.UpdateProcessingStatus(ctx, submission.ID, models.InventorySubmissionStatusCompleted); err != nil {
			return nil, fmt.Errorf("failed to update processing status to completed: %w", err)
		}
		submission.ProcessingStatus = models.InventorySubmissionStatusCompleted
	}

	return submission, nil
}

// processSubmission executes the actual inventory operation based on submission type
func (s *inventoryService) processSubmission(ctx context.Context, submission *models.InventorySubmission) error {
	switch submission.SubmissionType {
	case models.InventorySubmissionTypeReconcile:
		items := submission.Payload.(dto.ReconcileSubmissionPayload).Items
		return s.reconcileInventory(ctx, dto.ReconcileInventoryRequest{
			InventoryID: submission.InventoryID,
			Items:       items,
		})
	case models.InventorySubmissionTypeDispose:
		items := submission.Payload.(dto.DisposeSubmissionPayload).Items
		return s.disposeInventory(ctx, dto.DisposeInventoryRequest{
			InventoryID: submission.InventoryID,
			Items:       items,
		})
	default:
		return pkg.NewAppError(pkg.ErrorCodeValidation,
			fmt.Sprintf("unknown submission type: %s", submission.SubmissionType), nil)
	}
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
