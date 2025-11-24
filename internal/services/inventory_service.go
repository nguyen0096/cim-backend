package services

import (
	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
	"context"
	"encoding/json"
	"fmt"

	"github.com/labstack/gommon/log"
	"github.com/shopspring/decimal"
)

//go:generate mockery --name=InventoryService --structname=InventoryService --output=../mocks/servicemocks --outpkg=servicemocks
type InventoryService interface {
	CreateInventory(ctx context.Context, inventory *models.Inventory) error
	GetInventoryByID(ctx context.Context, id uint) (*models.Inventory, error)
	UpdateInventory(ctx context.Context, inventory *models.Inventory) error
	DeleteInventory(ctx context.Context, id uint) error
	ListInventory(ctx context.Context, limit, offset int) ([]models.Inventory, error)
	AddInventory(ctx context.Context, productID uint, quantity decimal.Decimal, referenceID uint, referenceType, notes string) error
	RemoveInventory(ctx context.Context, productID uint, quantity decimal.Decimal, referenceID uint, referenceType, notes string) error

	// v1
	GetLastPurchasePrices(ctx context.Context, supplierID uint) (dto.LastPurchasePriceMap, error)
	ListSubmissions(ctx context.Context, params models.ListParams, approvalStatuses []string, inventoryID uint, submissionTypes []string) ([]dto.SubmissionResponse, int64, error)
	CreateReconcileSubmission(ctx context.Context, req dto.ReconcileInventoryRequest) (*models.InventorySubmission, error)
	CreateDisposeSubmission(ctx context.Context, req dto.DisposeInventoryRequest) (*models.InventorySubmission, error)
	CreateTransferSubmission(ctx context.Context, req dto.TransferInventoryRequest) (*models.InventorySubmission, error)
	ProcessSubmission(ctx context.Context, req dto.SubmissionApprovalRequest) (*models.InventorySubmission, error)
	UpdateSubmission(ctx context.Context, req dto.UpdateSubmissionRequest) (*dto.SubmissionResponse, error)
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

func (s *inventoryService) AddInventory(ctx context.Context, productID uint, quantity decimal.Decimal, referenceID uint, referenceType, notes string) error {
	// Create transaction record
	return s.inventoryRepo.AddInventory(ctx, productID, quantity, referenceID, referenceType)
}

func (s *inventoryService) RemoveInventory(ctx context.Context, productID uint, quantity decimal.Decimal, referenceID uint, referenceType, notes string) error {
	return s.inventoryRepo.RemoveInventory(ctx, productID, quantity, referenceID, referenceType)
}

// consumeHandler is a function that creates a new transaction for consuming inventory
type consumeHandler func(item *models.InventoryItem, consumeTxn *models.InventoryTransaction, quantity decimal.Decimal) []*models.InventoryTransaction

// consumeFIFO is the common logic for consuming inventory items
// It accepts a map of item IDs to quantities to consumeFIFO and a transaction creator function.
// FIFO (First In, First Out) is used to consume inventory items.
func (s *inventoryService) consumeFIFO(
	ctx context.Context,
	ps *processingState,
	activeItems []*models.InventoryItem,
	itemConsumeQuantity map[uint]decimal.Decimal,
	consumeHandler consumeHandler,
) (
	ivtrItemChanges []*models.InventoryItemChange,
	txns []*models.InventoryTransaction,
	err error,
) {
	// validate consume quantity
	for itemID, consumeQty := range itemConsumeQuantity {
		var item *models.InventoryItem
		for _, i := range activeItems {
			if i.ID == itemID {
				item = i
				break
			}
		}
		if item == nil {
			_ = ps.addError(pkg.ErrInventoryItemNotFound(ctx, itemID))
			continue
		}

		if consumeQty.GreaterThan(item.Quantity) {
			_ = ps.addError(pkg.ErrConsumeFIFOFailed(
				fmt.Sprintf("quantity to consume %s exceeds available quantity %s for inventory item %d",
					consumeQty.String(), item.Quantity.String(), itemID)))
			continue
		}
	}
	if ps.hasAnyErrors() {
		return nil, nil, ps.addError(fmt.Errorf("failed to validate consume FIFO"))
	}

	for _, item := range activeItems {
		consumeQty, ok := itemConsumeQuantity[item.ID]
		if !ok || consumeQty.IsZero() {
			continue
		}

		toConsume := consumeQty
		var consumingTxnID uint
		txnCount := len(item.ConsumableTransactions)
		idx := 0

		for toConsume.GreaterThan(decimal.Zero) && idx < txnCount {
			txn := item.ConsumableTransactions[idx]
			txnUnconsumedQty := txn.Quantity.Sub(txn.ConsumedQuantity)

			if txnUnconsumedQty.IsZero() {
				idx++
				continue
			}

			// Create consumption transaction using the provided creator function
			var consumeQtyForTxn decimal.Decimal
			if toConsume.LessThan(txnUnconsumedQty) {
				consumeQtyForTxn = toConsume
			} else {
				consumeQtyForTxn = txnUnconsumedQty
			}
			newTxns := consumeHandler(item, txn, consumeQtyForTxn)
			txns = append(txns, newTxns...)

			txn.ConsumedQuantity = txn.ConsumedQuantity.Add(consumeQtyForTxn)
			txns = append(txns, txn)

			consumingTxnID = txn.ID
			toConsume = toConsume.Sub(consumeQtyForTxn)
			idx++
		}

		ivtrItemChanges = append(ivtrItemChanges, &models.InventoryItemChange{
			InventoryItem:    item,
			OriginalQuantity: item.Quantity,
		})

		item.Quantity = item.Quantity.Sub(consumeQty)
		item.ConsumingTransactionID = consumingTxnID
	}

	return ivtrItemChanges, txns, nil
}

// reconcileInventory reconciles inventory by creating sell transactions for the difference between previous and actual quantities
func (s *inventoryService) reconcileInventory(
	ctx context.Context,
	ps *processingState,
	req dto.ReconcileInventoryRequest,
) error {
	activeItems, err := s.getActiveInventoryItems(ctx, req.InventoryID, req.GetItemIDs())
	if err != nil {
		return ps.addError(fmt.Errorf("failed to get active inventory items: %w", err))
	}
	activeItemMap := s.buildItemMap(activeItems)

	itemConsumeQuantity := make(map[uint]decimal.Decimal)
	for _, reqItem := range req.Items {
		if reqItem.Quantity == nil {
			_ = ps.addError(pkg.ErrInvalidRequestBody(fmt.Errorf("actual quantity is required for inventory item %d", reqItem.InventoryItemID)))
			continue
		}

		item, exists := activeItemMap[reqItem.InventoryItemID]
		if !exists {
			_ = ps.addError(pkg.ErrInventoryItemNotFound(ctx, reqItem.InventoryItemID))
			continue
		}

		// for reconcile, actual quantity is an absolute value, so we need optimistic locking
		if !item.Quantity.Equal(reqItem.PrevQuantity) {
			_ = ps.addError(pkg.ErrOptimisticLockConflict(ctx, "inventory item", reqItem.InventoryItemID, reqItem.PrevQuantity, item.Quantity))
			continue
		}

		itemConsumeQuantity[reqItem.InventoryItemID] = item.Quantity.Sub(*reqItem.Quantity)
	}
	if ps.hasAnyErrors() {
		return ps.addError(pkg.ErrReconcileValidationFailed("validation failed for reconcile request"))
	}

	consumeHandler := func(item *models.InventoryItem, consumeTxn *models.InventoryTransaction, quantity decimal.Decimal) []*models.InventoryTransaction {
		return []*models.InventoryTransaction{
			{
				InventoryItemID:      item.ID,
				TransactionType:      models.InventoryTransactionTypeSell,
				Price:                consumeTxn.Price,
				Quantity:             quantity,
				CounterTransactionID: &consumeTxn.ID,
			},
		}
	}

	ivtrItemChanges, txns, err := s.consumeFIFO(ctx, ps, activeItems, itemConsumeQuantity, consumeHandler)
	if err != nil {
		return ps.addError(fmt.Errorf("failed to consume FIFO: %w", err))
	}

	if err := s.inventoryItemRepo.SaveInventoryItemChanges(ctx, ivtrItemChanges, txns); err != nil {
		return ps.addError(fmt.Errorf("failed to save inventory item changes: %w", err))
	}
	return nil
}

// CreateReconcileSubmission creates a submission for reconciling inventory
func (s *inventoryService) CreateReconcileSubmission(ctx context.Context, req dto.ReconcileInventoryRequest) (*models.InventorySubmission, error) {
	activeItems, err := s.getActiveInventoryItems(ctx, req.InventoryID, req.GetItemIDs())
	if err != nil {
		return nil, fmt.Errorf("failed to get active inventory items: %w", err)
	}
	activeItemMap := s.buildItemMap(activeItems)

	for _, reqItem := range req.Items {
		if reqItem.Quantity == nil {
			return nil, pkg.ErrInvalidRequestBody(fmt.Errorf("actual quantity is required for inventory item %d", reqItem.InventoryItemID))
		}

		item, exists := activeItemMap[reqItem.InventoryItemID]
		if !exists {
			return nil, pkg.ErrInventoryItemNotFound(ctx, reqItem.InventoryItemID)
		}

		// Optimistic locking: validate that current quantity matches what frontend saw
		if !item.Quantity.Equal(reqItem.PrevQuantity) {
			return nil, pkg.ErrOptimisticLockConflict(ctx, "inventory item", reqItem.InventoryItemID, reqItem.PrevQuantity, item.Quantity)
		}

		// Validate that actual quantity doesn't exceed previous quantity
		if reqItem.Quantity.GreaterThan(item.Quantity) {
			return nil, pkg.NewAppError(pkg.ErrorCodeValidation,
				fmt.Sprintf("actual quantity %d exceeds previous quantity %d for inventory item %d",
					*reqItem.Quantity, reqItem.PrevQuantity, reqItem.InventoryItemID), nil)
		}
	}

	payloadBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	submission := &models.InventorySubmission{
		InventoryID:      req.InventoryID,
		SubmissionType:   models.InventorySubmissionTypeReconcile,
		ProcessingStatus: models.InventorySubmissionStatusPending,
		Payload:          json.RawMessage(payloadBytes),
	}

	if err := s.inventorySubmissionRepo.Create(ctx, submission); err != nil {
		return nil, fmt.Errorf("failed to create reconcile submission: %w", err)
	}
	return submission, nil
}

// disposeInventory disposes inventory by creating disposal transactions for specified quantities
func (s *inventoryService) disposeInventory(
	ctx context.Context,
	ps *processingState,
	req dto.DisposeInventoryRequest,
) error {
	activeItems, err := s.getActiveInventoryItems(ctx, req.InventoryID, req.GetItemIDs())
	if err != nil {
		return ps.addError(fmt.Errorf("failed to get active inventory items: %w", err))
	}

	itemConsumeQuantity := make(map[uint]decimal.Decimal)
	for _, reqItem := range req.Items {
		if reqItem.Quantity == nil {
			_ = ps.addError(pkg.ErrInvalidRequestBody(fmt.Errorf("quantity is required for inventory item %d", reqItem.InventoryItemID)))
			continue
		}
		itemConsumeQuantity[reqItem.InventoryItemID] = *reqItem.Quantity
	}
	if ps.hasAnyErrors() {
		return ps.addError(pkg.ErrDisposeValidationFailed("validation failed for dispose request"))
	}

	// Create disposal transaction creator
	disposeTransactionCreator := func(item *models.InventoryItem, consumeTxn *models.InventoryTransaction, quantity decimal.Decimal) []*models.InventoryTransaction {
		return []*models.InventoryTransaction{
			{
				InventoryItemID:      item.ID,
				TransactionType:      models.InventoryTransactionTypeDisposal,
				Price:                consumeTxn.Price,
				Quantity:             quantity,
				CounterTransactionID: &consumeTxn.ID,
			},
		}
	}

	ivtrItemChanges, txns, err := s.consumeFIFO(ctx, ps, activeItems, itemConsumeQuantity, disposeTransactionCreator)
	if err != nil {
		return ps.addError(fmt.Errorf("failed to consume FIFO: %w", err))
	}

	if err := s.inventoryItemRepo.SaveInventoryItemChanges(ctx, ivtrItemChanges, txns); err != nil {
		return ps.addError(fmt.Errorf("failed to save inventory item changes: %w", err))
	}
	return nil
}

// CreateDisposeSubmission creates a submission for disposing inventory
func (s *inventoryService) CreateDisposeSubmission(ctx context.Context, req dto.DisposeInventoryRequest) (*models.InventorySubmission, error) {
	activeItems, err := s.getActiveInventoryItems(ctx, req.InventoryID, req.GetItemIDs())
	if err != nil {
		return nil, fmt.Errorf("failed to get active inventory items: %w", err)
	}
	activeItemMap := s.buildItemMap(activeItems)

	for _, reqItem := range req.Items {
		if reqItem.Quantity == nil {
			return nil, pkg.ErrInvalidRequestBody(fmt.Errorf("quantity is required for inventory item %d", reqItem.InventoryItemID))
		}

		// Validate that inventory item exists
		item, exists := activeItemMap[reqItem.InventoryItemID]
		if !exists {
			return nil, pkg.NewAppError(pkg.ErrorCodeNotFound,
				fmt.Sprintf("inventory item %d not found", reqItem.InventoryItemID), nil)
		}

		// Validate that dispose quantity doesn't exceed current quantity
		if reqItem.Quantity.GreaterThan(item.Quantity) {
			return nil, pkg.NewAppError(pkg.ErrorCodeValidation,
				fmt.Sprintf("dispose quantity %d exceeds available quantity %d for product %s",
					*reqItem.Quantity, item.Quantity, item.Product.Name), nil)
		}
	}

	payloadBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	submission := &models.InventorySubmission{
		InventoryID:      req.InventoryID,
		SubmissionType:   models.InventorySubmissionTypeDispose,
		ProcessingStatus: models.InventorySubmissionStatusPending,
		Payload:          json.RawMessage(payloadBytes),
	}

	if err := s.inventorySubmissionRepo.Create(ctx, submission); err != nil {
		return nil, fmt.Errorf("failed to create dispose submission: %w", err)
	}
	return submission, nil
}

// formatSubmissionItems builds simplified item summaries from payload
func formatSubmissionItems(
	items []dto.QuantityItem,
	itemsMap map[uint]*models.InventoryItem,
) ([]dto.QuantityItem, error) {
	summaries := make([]dto.QuantityItem, 0, len(items))
	for _, item := range items {
		inventoryItem, exists := itemsMap[item.InventoryItemID]
		if !exists {
			continue
		}

		if inventoryItem.Product == nil {
			return nil, fmt.Errorf("inventory item %d has no product", item.InventoryItemID)
		}

		summaries = append(summaries, dto.QuantityItem{
			InventoryItemID: item.InventoryItemID,
			ProductName:     inventoryItem.Product.Name,
			Quantity:        item.Quantity,
			PrevQuantity:    item.PrevQuantity,
			CurrentQuantity: inventoryItem.Quantity,
		})
	}
	return summaries, nil
}

func formatWarnings(
	submission models.InventorySubmission,
	items []dto.QuantityItem,
	itemsMap map[uint]*models.InventoryItem,
) []string {
	if items == nil || itemsMap == nil {
		return []string{}
	}

	// Only show warnings for pending submissions
	if submission.ApprovalStatus != models.InventorySubmissionApprovalStatusPending {
		return nil
	}

	warnings := make([]string, 0)

	for _, item := range items {
		// Skip if inventory item doesn't exist
		inventoryItem, exists := itemsMap[item.InventoryItemID]
		if !exists {
			continue
		}

		// Skip if inventory item or product is nil
		if inventoryItem == nil || inventoryItem.Product == nil {
			continue
		}

		// Skip if product name is empty
		if inventoryItem.Product.Name == "" {
			continue
		}

		// Skip if quantity is nil
		if item.Quantity == nil {
			continue
		}

		// Generate warnings based on submission type
		switch submission.SubmissionType {
		case models.InventorySubmissionTypeDispose, models.InventorySubmissionTypeTransfer:
			// Check if requested quantity exceeds available quantity
			if item.Quantity.GreaterThan(inventoryItem.Quantity) {
				operation := "hủy"
				if submission.SubmissionType == models.InventorySubmissionTypeTransfer {
					operation = "chuyển kho"
				}

				warning := fmt.Sprintf("Sản phẩm %s không đủ số lượng khả dụng (hiện tại: %s) để %s %s",
					inventoryItem.Product.Name, inventoryItem.Quantity, operation, *item.Quantity)
				warnings = append(warnings, warning)
			}
		case models.InventorySubmissionTypeReconcile:
			// Check if prev_quantity is not equal to current item quantity
			if item.PrevQuantity != inventoryItem.Quantity {
				warning := fmt.Sprintf("Số lượng sản phẩm %s đã thay đổi. Số lượng tại thời điểm tạo yêu cầu là %s. Số lượng hiện tại là %s.",
					inventoryItem.Product.Name,
					item.PrevQuantity,
					inventoryItem.Quantity)
				warnings = append(warnings, warning)
			}
		}
	}

	return warnings
}

// ProcessSubmission approves or rejects a pending inventory submission
func (s *inventoryService) ProcessSubmission(ctx context.Context, req dto.SubmissionApprovalRequest) (*models.InventorySubmission, error) {
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
	var approvalStatus models.SubmissionApprovalStatus
	switch models.InventorySubmissionAction(req.Action) {
	case models.InventorySubmissionActionApprove:
		approvalStatus = models.InventorySubmissionApprovalStatusApproved
	case models.InventorySubmissionActionReject:
		approvalStatus = models.InventorySubmissionApprovalStatusRejected
	default:
		return nil, pkg.NewAppError(pkg.ErrorCodeValidation,
			fmt.Sprintf("invalid action: %s", req.Action), nil)
	}

	// Step 1: Persist approval status and reason first
	if err := s.inventorySubmissionRepo.UpdateApprovalStatus(ctx, submission.ID, approvalStatus, req.Reason); err != nil {
		return nil, fmt.Errorf("failed to update approval status: %w", err)
	}
	submission.ApprovalStatus = approvalStatus
	submission.Reason = req.Reason

	// Step 2: Handle submission based on approval status
	switch approvalStatus {
	case models.InventorySubmissionApprovalStatusApproved:
		s.processSubmission(ctx, submission)
	case models.InventorySubmissionApprovalStatusRejected:
		// Set processing status to canceled when rejected
		if err := s.inventorySubmissionRepo.UpdateProcessingStatus(ctx, submission.ID, models.InventorySubmissionStatusCanceled); err != nil {
			return nil, fmt.Errorf("failed to update processing status to canceled: %w", err)
		}
		submission.ProcessingStatus = models.InventorySubmissionStatusCanceled
	case models.InventorySubmissionApprovalStatusPending:
		// No action needed for pending status
	}

	return submission, nil
}

// processSubmission executes the actual inventory operation based on submission type
func (s *inventoryService) processSubmission(ctx context.Context, submission *models.InventorySubmission) {
	ps := newProcessingState(s, submission)
	defer ps.end(ctx)

	switch submission.SubmissionType {
	case models.InventorySubmissionTypeReconcile:
		var req dto.ReconcileInventoryRequest
		if err := json.Unmarshal(submission.Payload, &req); err != nil {
			_ = ps.addError(fmt.Errorf("failed to unmarshal reconcile payload: %w", err))
			return
		}
		_ = s.reconcileInventory(ctx, ps, req)
	case models.InventorySubmissionTypeDispose:
		var req dto.DisposeInventoryRequest
		if err := json.Unmarshal(submission.Payload, &req); err != nil {
			_ = ps.addError(fmt.Errorf("failed to unmarshal dispose payload: %w", err))
			return
		}
		_ = s.disposeInventory(ctx, ps, req)
	case models.InventorySubmissionTypeTransfer:
		var req dto.TransferInventoryRequest
		if err := json.Unmarshal(submission.Payload, &req); err != nil {
			_ = ps.addError(fmt.Errorf("failed to unmarshal transfer payload: %w", err))
			return
		}
		_ = s.transferInventory(ctx, ps, req)
	default:
		_ = ps.addError(pkg.NewAppError(pkg.ErrorCodeValidation,
			fmt.Sprintf("unknown submission type: %s", submission.SubmissionType), nil))
		return
	}
}

// GetLastPurchasePrices retrieves the most recent purchase transaction prices for each product_id + supplier_id combination
func (s *inventoryService) GetLastPurchasePrices(ctx context.Context, supplierID uint) (dto.LastPurchasePriceMap, error) {
	prices, err := s.inventoryRepo.GetLastPurchasePrices(ctx, supplierID, 2)
	if err != nil {
		return nil, fmt.Errorf("failed to get last purchase prices: %w", err)
	}

	// Transform array into nested map: product_id -> supplier_id -> []PriceHistory
	priceMap := make(dto.LastPurchasePriceMap)
	for _, price := range prices {
		if priceMap[price.ProductID] == nil {
			priceMap[price.ProductID] = make(map[uint][]dto.PriceHistory)
		}

		priceHistory := dto.PriceHistory{
			Price:        price.LastPrice,
			PurchaseDate: price.LastPurchaseDate,
		}

		priceMap[price.ProductID][price.SupplierID] = append(
			priceMap[price.ProductID][price.SupplierID],
			priceHistory,
		)
	}

	return priceMap, nil
}

// CreateTransferSubmission creates a submission for transferring inventory items between inventories
func (s *inventoryService) CreateTransferSubmission(ctx context.Context, req dto.TransferInventoryRequest) (*models.InventorySubmission, error) {
	if req.SourceInventoryID == req.DestinationInventoryID {
		return nil, pkg.NewAppError(pkg.ErrorCodeValidation, "source and destination inventories must be different", nil)
	}

	sourceInventory, err := s.inventoryRepo.GetByID(ctx, req.SourceInventoryID)
	if err != nil {
		return nil, fmt.Errorf("failed to get source inventory: %w", err)
	}
	if sourceInventory == nil {
		return nil, pkg.NewAppError(pkg.ErrorCodeNotFound, "source inventory not found", nil)
	}

	destInventory, err := s.inventoryRepo.GetByID(ctx, req.DestinationInventoryID)
	if err != nil {
		return nil, fmt.Errorf("failed to get destination inventory: %w", err)
	}
	if destInventory == nil {
		return nil, pkg.NewAppError(pkg.ErrorCodeNotFound, "destination inventory not found", nil)
	}

	srcItems, err := s.getActiveInventoryItems(ctx, req.SourceInventoryID, req.GetItemIDs())
	if err != nil {
		return nil, fmt.Errorf("failed to get active inventory items: %w", err)
	}
	srcItemMap := s.buildItemMap(srcItems)

	for _, reqItem := range req.Items {
		if reqItem.Quantity == nil {
			return nil, pkg.ErrInvalidRequestBody(fmt.Errorf("quantity is required for inventory item %d", reqItem.InventoryItemID))
		}

		item, exists := srcItemMap[reqItem.InventoryItemID]
		if !exists {
			return nil, pkg.NewAppError(pkg.ErrorCodeNotFound,
				fmt.Sprintf("inventory item %d not found", reqItem.InventoryItemID), nil)
		}

		if reqItem.Quantity.GreaterThan(item.Quantity) {
			return nil, pkg.NewAppError(pkg.ErrorCodeValidation,
				fmt.Sprintf("transfer quantity %d exceeds available quantity %d for product %s",
					*reqItem.Quantity, item.Quantity, item.Product.Name), nil)
		}
	}

	payloadBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create inventory submission with pending status
	submission := &models.InventorySubmission{
		InventoryID:      req.SourceInventoryID,
		SubmissionType:   models.InventorySubmissionTypeTransfer,
		ProcessingStatus: models.InventorySubmissionStatusPending,
		Payload:          json.RawMessage(payloadBytes),
	}

	if err := s.inventorySubmissionRepo.Create(ctx, submission); err != nil {
		return nil, fmt.Errorf("failed to create transfer submission: %w", err)
	}
	return submission, nil
}

// transferInventory transfers inventory items from source to destination inventory
func (s *inventoryService) transferInventory(
	ctx context.Context,
	ps *processingState,
	req dto.TransferInventoryRequest,
) error {
	srcItems, err := s.getActiveInventoryItems(ctx, req.SourceInventoryID, req.GetItemIDs())
	if err != nil {
		return ps.addError(fmt.Errorf("failed to get active inventory items: %w", err))
	}
	srcItemMap := s.buildItemMap(srcItems)

	itemConsumeQuantity := make(map[uint]decimal.Decimal)
	productIDs := make([]uint, 0, len(req.Items))
	for _, reqItem := range req.Items {
		if reqItem.Quantity == nil {
			_ = ps.addError(pkg.ErrInvalidRequestBody(fmt.Errorf("actual quantity is required for inventory item %d", reqItem.InventoryItemID)))
			continue
		}

		item, exists := srcItemMap[reqItem.InventoryItemID]
		if !exists {
			_ = ps.addError(pkg.ErrInventoryItemNotFound(ctx, reqItem.InventoryItemID))
			continue
		}

		if reqItem.Quantity.GreaterThan(item.Quantity) {
			_ = ps.addError(pkg.ErrTransferValidationFailed(fmt.Sprintf("transfer quantity %d exceeds available quantity %d for inventory item %d",
				reqItem.Quantity, item.Quantity, reqItem.InventoryItemID)))
			continue
		}

		productIDs = append(productIDs, item.ProductID)
		itemConsumeQuantity[reqItem.InventoryItemID] = *reqItem.Quantity
	}
	if ps.hasAnyErrors() {
		return ps.addError(pkg.ErrTransferValidationFailed("validation failed for transfer request"))
	}

	destItems, err := s.getActiveInventoryItemsByProductIDs(ctx, req.DestinationInventoryID, productIDs)
	if err != nil && !pkg.IsErrorCode(err, pkg.ErrorCodeNotFound) {
		return ps.addError(fmt.Errorf("failed to get active destination inventory items: %w", err))
	}
	destItemMap := s.buildProductIDMap(destItems) // product_id -> inventory_item

	destIvtrItemChanges := make(map[uint]*models.InventoryItemChange, 0) // product_id -> change
	transferTransactionCreator := func(item *models.InventoryItem, consumeTxn *models.InventoryTransaction, quantity decimal.Decimal) []*models.InventoryTransaction {
		txns := []*models.InventoryTransaction{
			{
				InventoryItemID:      item.ID,
				TransactionType:      models.InventoryTransactionTypeTransferOut,
				Price:                consumeTxn.Price,
				Quantity:             quantity,
				CounterTransactionID: &consumeTxn.ID,
			},
			{
				TransactionType:      models.InventoryTransactionTypeTransferIn,
				Price:                consumeTxn.Price,
				Quantity:             quantity,
				CounterTransactionID: &consumeTxn.ID,
			},
		}

		// if destination change is created, only need to update quantity
		change, ok := destIvtrItemChanges[item.ProductID]
		if ok {
			change.Quantity = change.Quantity.Add(quantity)
			s.linkTxnWithInventoryItem(txns[1], change.InventoryItem)
			return txns
		}

		// check existing destination item so that we can create new
		// or udpate existing inventory item
		destItem, ok := destItemMap[item.ProductID]
		if !ok {
			destItem = &models.InventoryItem{
				InventoryID: req.DestinationInventoryID,
				ProductID:   item.ProductID,
				Status:      models.InventoryItemStatusActive,
			}
		}

		// create new destination change
		change = &models.InventoryItemChange{
			InventoryItem:    destItem,
			OriginalQuantity: destItem.Quantity,
		}
		destIvtrItemChanges[item.ProductID] = change

		change.Quantity = change.Quantity.Add(quantity)
		s.linkTxnWithInventoryItem(txns[1], change.InventoryItem)
		return txns
	}

	srcIvtrItemChanges, txns, err := s.consumeFIFO(ctx, ps, srcItems, itemConsumeQuantity, transferTransactionCreator)
	if err != nil {
		return ps.addError(fmt.Errorf("failed to consume FIFO: %w", err))
	}

	// combine src and dest ivtr item changes
	ivtrItemChanges := make([]*models.InventoryItemChange, 0, len(srcIvtrItemChanges)+len(destIvtrItemChanges))
	ivtrItemChanges = append(ivtrItemChanges, srcIvtrItemChanges...)
	for _, change := range destIvtrItemChanges {
		ivtrItemChanges = append(ivtrItemChanges, change)
	}

	if err := s.inventoryItemRepo.SaveInventoryItemChanges(ctx, ivtrItemChanges, txns); err != nil {
		return ps.addError(fmt.Errorf("failed to save inventory item changes: %w", err))
	}
	return nil
}

func (s *inventoryService) linkTxnWithInventoryItem(txn *models.InventoryTransaction, inventoryItem *models.InventoryItem) {
	if inventoryItem.ID == 0 {
		inventoryItem.ID = txn.InventoryItemID
	}
	txn.InventoryItem = inventoryItem
}

// getActiveInventoryItems gets active inventory items by item IDs and validates them.
func (s *inventoryService) getActiveInventoryItems(
	ctx context.Context,
	inventoryID uint,
	itemIDs []uint,
) ([]*models.InventoryItem, error) {
	activeItems, err := s.inventoryItemRepo.GetActiveInventoryItems(ctx, inventoryID, itemIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get active inventory items: %w", err)
	}

	if len(activeItems) == 0 {
		return nil, pkg.NewAppError(pkg.ErrorCodeNotFound, "no active inventory items found", nil)
	}

	for _, item := range activeItems {
		if err := item.ValidateActivePurchaseTransactions(); err != nil {
			return nil, fmt.Errorf("validation failed for inventory item %d: %w", item.ID, err)
		}
	}

	return activeItems, nil
}

func (s *inventoryService) getActiveInventoryItemsByProductIDs(
	ctx context.Context,
	inventoryID uint,
	productIDs []uint,
) ([]*models.InventoryItem, error) {
	activeItems, err := s.inventoryItemRepo.GetActiveInventoryItemsByProductIDs(ctx, inventoryID, productIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get active inventory items by product IDs: %w", err)
	}

	if len(activeItems) == 0 {
		return nil, pkg.NewAppError(pkg.ErrorCodeNotFound, "no active inventory items found", nil)
	}

	for _, item := range activeItems {
		if err := item.ValidateActivePurchaseTransactions(); err != nil {
			return nil, fmt.Errorf("validation failed for inventory item %d: %w", item.ID, err)
		}
	}

	return activeItems, nil
}

// buildItemMap builds a map of inventory items by item ID.
func (s *inventoryService) buildItemMap(items []*models.InventoryItem) map[uint]*models.InventoryItem {
	itemMap := make(map[uint]*models.InventoryItem)
	for _, item := range items {
		if item.ID == 0 {
			continue
		}
		itemMap[item.ID] = item
	}
	return itemMap
}

func (s *inventoryService) buildProductIDMap(items []*models.InventoryItem) map[uint]*models.InventoryItem {
	productIDMap := make(map[uint]*models.InventoryItem)
	for _, item := range items {
		if item.ProductID == 0 {
			continue
		}
		productIDMap[item.ProductID] = item
	}
	return productIDMap
}

func newProcessingState(s *inventoryService, submission *models.InventorySubmission) *processingState {
	return &processingState{
		svc:        s,
		submission: submission,
		errors:     make([]error, 0),
	}
}

// processingState represents a job.
type processingState struct {
	svc        *inventoryService
	submission *models.InventorySubmission
	errors     []error
}

func (s *processingState) hasAnyErrors() bool {
	return len(s.errors) > 0
}

func (s *processingState) addError(err error) error {
	s.errors = append(s.errors, err)
	return err
}

func (s *processingState) end(ctx context.Context) {
	if s.hasAnyErrors() {
		if err := s.svc.inventorySubmissionRepo.FailSubmissionProcessingWithErrors(ctx, s.submission.ID, s.errors); err != nil {
			log.Error("failed to fail submission processing with errors: %w", err)
		}
		return
	}

	if err := s.svc.inventorySubmissionRepo.UpdateProcessingStatus(ctx, s.submission.ID, models.InventorySubmissionStatusCompleted); err != nil {
		log.Error("failed to update submission status: %w", err)
	}
}

// ListSubmissions retrieves submissions with pagination and filtering
func (s *inventoryService) ListSubmissions(ctx context.Context, params models.ListParams, approvalStatuses []string, inventoryID uint, submissionTypes []string) ([]dto.SubmissionResponse, int64, error) {
	// Get submissions from repository
	submissions, total, err := s.inventorySubmissionRepo.ListSubmissions(ctx, params, inventoryID, approvalStatuses, submissionTypes)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list submissions: %w", err)
	}

	if len(submissions) == 0 {
		return []dto.SubmissionResponse{}, 0, nil
	}

	submissionItemMap := make(map[uint][]dto.QuantityItem)
	itemIDs := make([]uint, 0)
	for _, submission := range submissions {
		var genericPayload struct {
			Items []dto.QuantityItem `json:"items"`
		}
		if err := json.Unmarshal(submission.Payload, &genericPayload); err != nil {
			continue
		}
		submissionItemMap[submission.ID] = genericPayload.Items
		itemIDs = append(itemIDs, models.GetIDs(genericPayload.Items)...)
	}

	inventoryItems, err := s.inventoryItemRepo.GetByIDs(ctx, itemIDs)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get inventory items: %w", err)
	}
	inventoryItemMap := s.buildItemMap(inventoryItems)

	responses := make([]dto.SubmissionResponse, len(submissions))
	for i, submission := range submissions {
		items := submissionItemMap[submission.ID]

		items, err := formatSubmissionItems(items, inventoryItemMap)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to format submission items: %w", err)
		}

		responses[i] = dto.SubmissionResponse{
			ID:             submission.ID,
			InventoryID:    submission.InventoryID,
			Inventory:      submission.Inventory,
			SubmissionType: submission.SubmissionType,
			Status:         submission.ProcessingStatus,
			ApprovalStatus: submission.ApprovalStatus,
			Errors:         s.formatProcessingErrors(submission.Error),
			Warnings:       formatWarnings(submission, items, inventoryItemMap),
			Items:          items,
			Reason:         submission.Reason,
			CreatedBy:      submission.CreatedBy,
			CreatedAt:      submission.CreatedAt.Format(pkg.DateTimeFormat),
			UpdatedBy:      submission.UpdatedBy,
			UpdatedAt:      submission.UpdatedAt.Format(pkg.DateTimeFormat),
		}
	}

	return responses, total, nil
}

func (s *inventoryService) formatProcessingErrors(jsonStr json.RawMessage) json.RawMessage {
	if len(jsonStr) == 0 {
		return nil
	}

	// Unmarshal to []map[string]any
	var errors []map[string]interface{}
	if err := json.Unmarshal(jsonStr, &errors); err != nil {
		// If unmarshaling fails, return a single internal error
		internalError := map[string]interface{}{
			"code":    pkg.ErrorCodeInternal.String(),
			"message": "Internal server error",
		}
		result, _ := json.Marshal([]map[string]interface{}{internalError})
		return json.RawMessage(result)
	}

	var formattedErrors []map[string]interface{}
	hasInternalError := false

	for _, err := range errors {
		// Check if "code" is present - if so, it's an AppError
		if _, exists := err["code"]; exists {
			// It's an AppError, remove "cause" field for user display
			cleanError := map[string]interface{}{
				"code":    err["code"],
				"message": err["message"],
			}
			formattedErrors = append(formattedErrors, cleanError)
		} else {
			// No "code" present, it's an internal server error
			hasInternalError = true
		}
	}

	// If there are any internal errors, add a single internal error at the end
	if hasInternalError {
		internalError := map[string]interface{}{
			"code":    pkg.ErrorCodeInternal.String(),
			"message": "Internal server error",
		}
		formattedErrors = append(formattedErrors, internalError)
	}

	if len(formattedErrors) == 0 {
		return nil
	}

	result, err := json.Marshal(formattedErrors)
	if err != nil {
		// If marshaling fails, return a single internal error
		internalError := map[string]interface{}{
			"code":    pkg.ErrorCodeInternal.String(),
			"message": "Internal server error",
		}
		result, _ := json.Marshal([]map[string]interface{}{internalError})
		return json.RawMessage(result)
	}

	return json.RawMessage(result)
}

// UpdateSubmission updates the items in a pending submission
func (s *inventoryService) UpdateSubmission(ctx context.Context, req dto.UpdateSubmissionRequest) (*dto.SubmissionResponse, error) {
	// Get the submission
	submission, err := s.inventorySubmissionRepo.GetByID(ctx, req.SubmissionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get submission: %w", err)
	}

	// Verify submission approval is pending - only check approval status
	if submission.ApprovalStatus != models.InventorySubmissionApprovalStatusPending {
		return nil, pkg.NewAppError(pkg.ErrorCodeValidation,
			fmt.Sprintf("cannot update submission with approval status: %s. Only pending submissions can be updated", submission.ApprovalStatus), nil)
	}

	// Validate based on submission type
	switch submission.SubmissionType {
	case models.InventorySubmissionTypeReconcile:
		if err := s.validateReconcileUpdate(ctx, submission.InventoryID, req.Items); err != nil {
			return nil, err
		}
	case models.InventorySubmissionTypeDispose:
		if err := s.validateDisposeUpdate(ctx, submission.InventoryID, req.Items); err != nil {
			return nil, err
		}
	case models.InventorySubmissionTypeTransfer:
		if err := s.validateTransferUpdate(ctx, submission, req.Items); err != nil {
			return nil, err
		}
	default:
		return nil, pkg.NewAppError(pkg.ErrorCodeValidation,
			fmt.Sprintf("unknown submission type: %s", submission.SubmissionType), nil)
	}

	// Build updated payload based on submission type
	var payloadBytes []byte
	switch submission.SubmissionType {
	case models.InventorySubmissionTypeReconcile:
		updatedReq := dto.ReconcileInventoryRequest{
			InventoryID: submission.InventoryID,
			Items:       req.Items,
		}
		payloadBytes, err = json.Marshal(updatedReq)
	case models.InventorySubmissionTypeDispose:
		updatedReq := dto.DisposeInventoryRequest{
			InventoryID: submission.InventoryID,
			Items:       req.Items,
		}
		payloadBytes, err = json.Marshal(updatedReq)
	case models.InventorySubmissionTypeTransfer:
		// For transfer, we need to extract the source and destination inventory IDs from the original payload
		var originalReq dto.TransferInventoryRequest
		if unmarshalErr := json.Unmarshal(submission.Payload, &originalReq); unmarshalErr != nil {
			return nil, fmt.Errorf("failed to unmarshal original transfer payload: %w", unmarshalErr)
		}
		updatedReq := dto.TransferInventoryRequest{
			SourceInventoryID:      originalReq.SourceInventoryID,
			DestinationInventoryID: originalReq.DestinationInventoryID,
			Items:                  req.Items,
		}
		payloadBytes, err = json.Marshal(updatedReq)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to marshal updated payload: %w", err)
	}

	// Update the submission payload
	if err := s.inventorySubmissionRepo.UpdateSubmissionPayload(ctx, submission.ID, payloadBytes); err != nil {
		return nil, fmt.Errorf("failed to update submission payload: %w", err)
	}

	// Update the submission object with the new payload
	submission.Payload = json.RawMessage(payloadBytes)

	// Get inventory items for formatting response
	itemIDs := make([]uint, 0, len(req.Items))
	for _, item := range req.Items {
		itemIDs = append(itemIDs, item.InventoryItemID)
	}

	inventoryItems, err := s.inventoryItemRepo.GetByIDs(ctx, itemIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get inventory items: %w", err)
	}
	inventoryItemMap := s.buildItemMap(inventoryItems)

	// Load inventory for response
	inventory, err := s.inventoryRepo.GetByID(ctx, submission.InventoryID)
	if err != nil {
		return nil, fmt.Errorf("failed to get inventory: %w", err)
	}
	submission.Inventory = inventory

	items, err := formatSubmissionItems(req.Items, inventoryItemMap)
	if err != nil {
		return nil, fmt.Errorf("failed to format submission items: %w", err)
	}

	// Build response
	response := &dto.SubmissionResponse{
		ID:             submission.ID,
		InventoryID:    submission.InventoryID,
		Inventory:      submission.Inventory,
		SubmissionType: submission.SubmissionType,
		Status:         submission.ProcessingStatus,
		ApprovalStatus: submission.ApprovalStatus,
		Items:          items,
		Warnings:       formatWarnings(*submission, req.Items, inventoryItemMap),
		Reason:         submission.Reason,
		CreatedBy:      submission.CreatedBy,
		CreatedAt:      submission.CreatedAt.Format(pkg.DateTimeFormat),
		UpdatedBy:      submission.UpdatedBy,
		UpdatedAt:      submission.UpdatedAt.Format(pkg.DateTimeFormat),
	}

	return response, nil
}

// validateReconcileUpdate validates items for reconcile submission update
func (s *inventoryService) validateReconcileUpdate(ctx context.Context, inventoryID uint, items []dto.QuantityItem) error {
	activeItems, err := s.getActiveInventoryItems(ctx, inventoryID, models.GetIDs(items))
	if err != nil {
		return fmt.Errorf("failed to get active inventory items: %w", err)
	}
	activeItemMap := s.buildItemMap(activeItems)

	for _, reqItem := range items {
		if reqItem.Quantity == nil {
			return pkg.ErrInvalidRequestBody(fmt.Errorf("actual quantity is required for inventory item %d", reqItem.InventoryItemID))
		}

		item, exists := activeItemMap[reqItem.InventoryItemID]
		if !exists {
			return pkg.ErrInventoryItemNotFound(ctx, reqItem.InventoryItemID)
		}

		// Optimistic locking: validate that current quantity matches what frontend saw
		if item.Quantity != reqItem.PrevQuantity {
			return pkg.ErrOptimisticLockConflict(ctx, "inventory item", reqItem.InventoryItemID, reqItem.PrevQuantity, item.Quantity)
		}

		// Validate that actual quantity doesn't exceed previous quantity
		if reqItem.Quantity.GreaterThan(item.Quantity) {
			return pkg.NewAppError(pkg.ErrorCodeValidation,
				fmt.Sprintf("actual quantity %d exceeds previous quantity %d for inventory item %d",
					*reqItem.Quantity, reqItem.PrevQuantity, reqItem.InventoryItemID), nil)
		}
	}

	return nil
}

// validateDisposeUpdate validates items for dispose submission update
func (s *inventoryService) validateDisposeUpdate(ctx context.Context, inventoryID uint, items []dto.QuantityItem) error {
	activeItems, err := s.getActiveInventoryItems(ctx, inventoryID, models.GetIDs(items))
	if err != nil {
		return fmt.Errorf("failed to get active inventory items: %w", err)
	}
	activeItemMap := s.buildItemMap(activeItems)

	for _, reqItem := range items {
		if reqItem.Quantity == nil {
			return pkg.ErrInvalidRequestBody(fmt.Errorf("quantity is required for inventory item %d", reqItem.InventoryItemID))
		}

		item, exists := activeItemMap[reqItem.InventoryItemID]
		if !exists {
			return pkg.NewAppError(pkg.ErrorCodeNotFound,
				fmt.Sprintf("inventory item %d not found", reqItem.InventoryItemID), nil)
		}

		// Validate that dispose quantity doesn't exceed current quantity
		if reqItem.Quantity.GreaterThan(item.Quantity) {
			return pkg.NewAppError(pkg.ErrorCodeValidation,
				fmt.Sprintf("dispose quantity %d exceeds available quantity %d for product %s",
					*reqItem.Quantity, item.Quantity, item.Product.Name), nil)
		}
	}

	return nil
}

// validateTransferUpdate validates items for transfer submission update
func (s *inventoryService) validateTransferUpdate(ctx context.Context, submission *models.InventorySubmission, items []dto.QuantityItem) error {
	// Unmarshal original payload to get source inventory ID
	var originalReq dto.TransferInventoryRequest
	if err := json.Unmarshal(submission.Payload, &originalReq); err != nil {
		return fmt.Errorf("failed to unmarshal transfer payload: %w", err)
	}

	activeItems, err := s.getActiveInventoryItems(ctx, originalReq.SourceInventoryID, models.GetIDs(items))
	if err != nil {
		return fmt.Errorf("failed to get active inventory items: %w", err)
	}
	activeItemMap := s.buildItemMap(activeItems)

	for _, reqItem := range items {
		if reqItem.Quantity == nil {
			return pkg.ErrInvalidRequestBody(fmt.Errorf("quantity is required for inventory item %d", reqItem.InventoryItemID))
		}

		item, exists := activeItemMap[reqItem.InventoryItemID]
		if !exists {
			return pkg.NewAppError(pkg.ErrorCodeNotFound,
				fmt.Sprintf("inventory item %d not found", reqItem.InventoryItemID), nil)
		}

		// Validate that transfer quantity doesn't exceed current quantity
		if reqItem.Quantity.GreaterThan(item.Quantity) {
			return pkg.NewAppError(pkg.ErrorCodeValidation,
				fmt.Sprintf("transfer quantity %d exceeds available quantity %d for product %s",
					*reqItem.Quantity, item.Quantity, item.Product.Name), nil)
		}
	}

	return nil
}
