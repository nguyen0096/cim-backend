package services

import (
	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/internal/services/dto"
	"cim-backend/internal/services/excel"
	"cim-backend/pkg"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/labstack/gommon/log"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
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

	// v1 - AGENTS MUST CONFIRM BEFORE MODIFYING SECTION BELOW THIS LINE

	GetLastPurchasePrices(ctx context.Context, supplierID uint) (dto.LastPurchasePriceMap, error)
	ListSubmissions(ctx context.Context, params models.ListParams, approvalStatuses []string, inventoryID uint, submissionTypes []string) ([]dto.SubmissionResponse, int64, error)
	// ListReconciliationsAwaitingProcessing returns the cross-inventory queue of
	// reconcile submissions awaiting Start-Processing (issue #88) for the #87
	// list/overview UI. It is gated by the recon_manage permission (403 otherwise,
	// mirroring CloseReconciliation/StartProcessing) and uses a lightweight queue
	// mapper that does NOT synthesize per-row payloads (so the page is a bounded,
	// constant query count — no N+1). reconcileStatuses optionally narrows the
	// open/closed set.
	ListReconciliationsAwaitingProcessing(ctx context.Context, params models.ListParams, reconcileStatuses []string) ([]dto.SubmissionResponse, int64, error)
	InitiateReconcile(ctx context.Context, req dto.InitiateReconcileRequest) (*models.InventorySubmission, error)
	CreateReconcileSubmission(ctx context.Context, req dto.ReconcileInventoryRequest) (*models.InventorySubmission, error)
	CreateDisposeSubmission(ctx context.Context, req dto.DisposeInventoryRequest) (*models.InventorySubmission, error)
	CreateTransferSubmission(ctx context.Context, req dto.TransferInventoryRequest) (*models.InventorySubmission, error)
	ProcessSubmission(ctx context.Context, req dto.SubmissionApprovalRequest) (*models.InventorySubmission, error)
	UpdateSubmission(ctx context.Context, req dto.UpdateSubmissionRequest) (*dto.SubmissionResponse, error)
	GetMonthlyTransactionReport(ctx context.Context, inventoryID uint, month, year int) (*models.TxnReportInventory, error)

	// Staff reconciliation child-item lifecycle (epic #38, Part 4; row + count
	// labels issue #73). Create/Update/List return the FE row response shape
	// (dto.ReconciliationItemResponse) — row label + flattened count lines — rather
	// than the raw model.
	CreateReconciliationItem(ctx context.Context, req dto.CreateReconciliationItemRequest) (*dto.ReconciliationItemResponse, error)
	UpdateReconciliationItem(ctx context.Context, req dto.UpdateReconciliationItemRequest) (*dto.ReconciliationItemResponse, error)
	DeleteReconciliationItem(ctx context.Context, req dto.DeleteReconciliationItemRequest) error
	// ListReconciliationItems returns the live count-session rows of an initiated
	// reconcile, RBAC-scoped (staff: own rows; admin/accountant: all), id-ascending.
	ListReconciliationItems(ctx context.Context, submissionID uint) ([]dto.ReconciliationItemResponse, error)

	// Admin/accountant reconciliation management (epic #38, Part 6 redesign; one
	// recon_manage action, admin+accountant only). Close locks staff out
	// (open->closed); reopen re-opens (closed->open). StartProcessing is the ONE
	// atomic apply transaction: advisory-locked per inventory, runs the event-based
	// drift re-check (roll back + warning payload on drift), and otherwise applies
	// the synthesized reconcile with snapshot-aware consume sizing and finalizes the
	// submission to processed.
	CloseReconciliation(ctx context.Context, submissionID uint) (*models.InventorySubmission, error)
	ReopenReconciliation(ctx context.Context, submissionID uint) (*models.InventorySubmission, error)
	StartProcessing(ctx context.Context, submissionID uint) (*dto.StartProcessingResult, error)

	// SynthesizeSubmissionPayload folds the live (non-deleted) staff child rows of
	// an initiated reconcile into the legacy ReconcileInventoryRequest-shaped
	// payload the apply path consumes, summing counted quantities by
	// inventory_item_id and attaching the snapshot baseline as PrevQuantity. It is
	// pure/read-only synthesis (epic #38, Part 5) and drives the list/detail view,
	// review label and warnings for active reconciles.
	SynthesizeSubmissionPayload(ctx context.Context, submissionID uint) (*dto.SynthesizedReconcile, error)
}

type inventoryService struct {
	inventoryRepo           repository.InventoryRepository
	inventoryItemRepo       repository.InventoryItemRepository
	inventorySubmissionRepo repository.InventorySubmissionRepository
	snapshotRepo            repository.ReconciliationSnapshotRepository
	reconItemRepo           repository.ReconciliationRequestItemRepository
	productRepo             repository.ProductRepository

	fileStorageService FileStorageService
	// baseRepo is the repository-layer transaction root. The reconcile flow does
	// not use a *gorm.DB directly; it asks the base repository to open one
	// transaction and thread it through the context (WithinTx), then calls
	// repository methods with that context so they enlist in the same transaction.
	baseRepo repository.BaseRepository
	// db backs the pre-existing monthly-transaction-report query
	// (getPurchaseOrderItemsLookup) only. New atomic flows must go through
	// baseRepo.WithinTx + repositories rather than reaching for this handle.
	db *gorm.DB
}

func NewInventoryService(
	inventoryRepo repository.InventoryRepository,
	inventoryItemRepo repository.InventoryItemRepository,
	inventorySubmissionRepo repository.InventorySubmissionRepository,
	snapshotRepo repository.ReconciliationSnapshotRepository,
	reconItemRepo repository.ReconciliationRequestItemRepository,
	productRepo repository.ProductRepository,
	fileStorageService FileStorageService,
	baseRepo repository.BaseRepository,
	db *gorm.DB,
) InventoryService {
	return &inventoryService{
		inventoryRepo:           inventoryRepo,
		productRepo:             productRepo,
		inventoryItemRepo:       inventoryItemRepo,
		inventorySubmissionRepo: inventorySubmissionRepo,
		snapshotRepo:            snapshotRepo,
		reconItemRepo:           reconItemRepo,
		fileStorageService:      fileStorageService,
		baseRepo:                baseRepo,
		db:                      db,
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

// InitiateReconcile starts a reconciliation (epic #38, Part 2). In one tx it
// creates a placeholder pending reconcile submission and captures one snapshot
// per active inventory item (prev_quantity = live quantity at initiate). The
// snapshot is the sole baseline for later synthesize/review/apply, so it is
// captured atomically with the parent; a failed capture rolls the placeholder back.
func (s *inventoryService) InitiateReconcile(ctx context.Context, req dto.InitiateReconcileRequest) (*models.InventorySubmission, error) {
	// Defense-in-depth: mirror ProcessSubmission's in-service permission check in
	// addition to the RBAC route gate.
	if !pkg.HasPermission(ctx, pkg.RBACResourceInventorySubmissions, pkg.RBACActionInitiateReconciliation) {
		return nil, pkg.NewAppError(pkg.ErrorCodeForbidden, "user does not have permission to initiate reconciliation", nil)
	}

	if req.InventoryID == 0 {
		return nil, pkg.ErrInvalidRequestBody(fmt.Errorf("inventory_id is required"))
	}

	// Verify the inventory exists before the tx so a missing id is a 404, not a
	// 500 from the FK on insert. Lightweight existence check (no preloads).
	exists, err := s.inventoryRepo.ExistsByID(ctx, req.InventoryID)
	if err != nil {
		return nil, fmt.Errorf("failed to check inventory existence: %w", err)
	}
	if !exists {
		return nil, pkg.NewAppError(pkg.ErrorCodeNotFound,
			fmt.Sprintf("inventory %d not found", req.InventoryID), nil)
	}

	submission := &models.InventorySubmission{
		InventoryID:      req.InventoryID,
		SubmissionType:   models.InventorySubmissionTypeReconcile,
		ProcessingStatus: models.InventorySubmissionStatusPending,
		ApprovalStatus:   models.InventorySubmissionApprovalStatusPending,
		// The reconcile starts `open`: staff may freely file/edit/delete count rows
		// until an admin/accountant closes it (epic #38, Part 6).
		ReconcileStatus: models.ReconcileLifecycleStatusOpen,
	}

	// One tx threaded via txCtx so the placeholder submission and its baseline
	// snapshots commit or roll back atomically.
	err = s.baseRepo.WithinTx(ctx, func(txCtx context.Context) error {
		// One-active-pending guard (S5): in-tx pre-check; the partial unique index
		// is the race-safe backstop and the repo Create translates its violation
		// into the same domain conflict.
		if err := s.guardNoActivePending(txCtx, req.InventoryID); err != nil {
			return err
		}

		// Create the parent placeholder submission first so snapshot rows can FK to it.
		if err := s.inventorySubmissionRepo.Create(txCtx, submission); err != nil {
			if pkg.IsErrorCode(err, pkg.ErrorCodeActivePendingReconcileConflict) {
				return err
			}
			return fmt.Errorf("failed to create reconcile submission: %w", err)
		}

		// Serialize the snapshot capture with consuming applies (epic #38, Part 6 —
		// closes the stale-baseline race the review raised). The snapshot below is a
		// plain MVCC read of inventory_items row versions; it takes NO row locks, so
		// without this a consuming dispose/transfer/reconcile apply could stamp its
		// processed_at just before this parent's created_at yet COMMIT just after the
		// snapshot reads the pre-apply versions — its stock effect would be neither in
		// our baseline nor caught by StartProcessing's processed_at >= created_at drift
		// re-check, leaving us to apply a stale baseline. By taking
		// pg_advisory_xact_lock(inventory_id) here — the SAME lock every consuming
		// apply (ProcessSubmission approve) and StartProcessing already takes BEFORE it
		// reads/writes stock — snapshot capture and consuming applies fully serialize:
		// a consuming apply either commits entirely before this snapshot (its effect is
		// in the baseline) or is blocked until this initiate tx commits and then
		// stamps a processed_at inside StartProcessing's drift window (caught by the
		// re-check). No interleaving yields a stale baseline.
		//
		// Deadlock-free: initiate takes ONLY this advisory lock plus its own
		// snapshot/submission inserts; it never grabs an inventory_items row lock
		// while holding the advisory lock, and every other holder also acquires the
		// advisory lock FIRST, so there is no lock-ordering cycle.
		if err := s.inventorySubmissionRepo.AcquireInventoryAdvisoryLock(txCtx, req.InventoryID); err != nil {
			return fmt.Errorf("failed to acquire inventory advisory lock: %w", err)
		}

		// Capture the baseline via one set-based INSERT ... SELECT: one snapshot per
		// active item, prev_quantity = its live quantity. count = active-item count.
		count, err := s.snapshotRepo.BuildReconciliationSnapshots(txCtx, submission.ID, req.InventoryID)
		if err != nil {
			return fmt.Errorf("failed to capture reconciliation snapshots: %w", err)
		}
		if count == 0 {
			// Roll back the placeholder: an inventory with no active items has no
			// baseline to reconcile against.
			return pkg.NewAppError(pkg.ErrorCodeNotFound,
				fmt.Sprintf("no active inventory items found for inventory %d", req.InventoryID), nil)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return submission, nil
}

// guardNotInitiatedReconcile blocks the legacy update/approve paths from touching
// a reconcile started via initiate (epic #38, Part 2). Presence of snapshot rows
// marks the new-model flow; letting the legacy path act on it would bypass the
// snapshot baseline or approve an empty payload. Reconcile-only.
func (s *inventoryService) guardNotInitiatedReconcile(ctx context.Context, submission *models.InventorySubmission) error {
	if submission.SubmissionType != models.InventorySubmissionTypeReconcile {
		return nil
	}
	hasSnapshots, err := s.snapshotRepo.ExistsForSubmission(ctx, submission.ID)
	if err != nil {
		return fmt.Errorf("failed to check reconciliation snapshots: %w", err)
	}
	if hasSnapshots {
		return pkg.NewAppError(pkg.ErrorCodeConflict,
			fmt.Sprintf("submission %d was started via reconcile-initiate and uses the snapshot-based flow; it cannot be updated or approved through the legacy path", submission.ID),
			nil)
	}
	return nil
}

// guardNoActivePending is the service pre-check for the one-active-pending-
// reconcile-per-inventory rule (#38 P3, reconcile-only): at most one reconcile
// may be in flight (processing_status='pending') per inventory. Dispose/transfer
// are not guarded. It returns the same ErrActivePendingReconcileConflict the repo
// raises on a race-loser index violation, so common case and race agree. The
// partial unique index is the race-safe backstop; this is tx-aware via DB(ctx).
func (s *inventoryService) guardNoActivePending(ctx context.Context, inventoryID uint) error {
	exists, err := s.inventorySubmissionRepo.ExistsActivePending(ctx, inventoryID)
	if err != nil {
		return fmt.Errorf("failed to check for an existing active reconcile submission: %w", err)
	}
	if exists {
		return pkg.ErrActivePendingReconcileConflict(inventoryID, nil)
	}
	return nil
}

// CreateReconcileSubmission creates a submission for reconciling inventory
func (s *inventoryService) CreateReconcileSubmission(ctx context.Context, req dto.ReconcileInventoryRequest) (*models.InventorySubmission, error) {
	if err := s.guardNoActivePending(ctx, req.InventoryID); err != nil {
		return nil, err
	}

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
		// The repo translates a race-loser unique-violation into the same domain
		// conflict the pre-check returns; propagate it, wrap anything else.
		if pkg.IsErrorCode(err, pkg.ErrorCodeActivePendingReconcileConflict) {
			return nil, err
		}
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

// CreateDisposeSubmission creates a submission for disposing inventory.
//
// Dispose is intentionally NOT subject to the one-active-pending guard (epic #38,
// Part 3 is reconcile-only per the human's decision): a pending reconcile does not
// block a dispose, and multiple pending disposes are allowed, exactly as on main.
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
			if !item.PrevQuantity.Equal(inventoryItem.Quantity) {
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

	// Block legacy *approval* of reconciliations started via the initiate endpoint
	// (snapshot-based flow); approve would unmarshal the empty placeholder payload
	// and bypass the snapshot baseline, so it must go through the later
	// synthesize/approve path. Reject is safe — it only marks the submission
	// rejected/canceled and never reads the payload — so an initiated reconcile can
	// still be rejected through this endpoint.
	if approvalStatus == models.InventorySubmissionApprovalStatusApproved {
		if err := s.guardNotInitiatedReconcile(ctx, submission); err != nil {
			return nil, err
		}
	}

	// Step 2: Handle submission based on approval status. Note: the approval-status
	// write is performed INSIDE each branch (under the branch's transaction/lock),
	// not before the switch, so the reject path can re-check the reconcile lifecycle
	// under the parent lock BEFORE committing to any write (TOCTOU fix below).
	switch approvalStatus {
	case models.InventorySubmissionApprovalStatusApproved:
		// Take the per-inventory advisory lock (epic #38, Part 6) around the
		// consuming apply so it serializes with a concurrent reconcile Start-
		// Processing on the same inventory: pg_advisory_xact_lock(inventory_id) is
		// transaction-scoped, so the apply (which now enlists in this tx via the
		// tx-aware repos) and the processed_at stamp commit together and the
		// reconcile's drift re-check either blocks on this lock or sees this row's
		// processed_at in its window. This is the one sanctioned touch of the
		// consuming ProcessSubmission path. On apply failure the whole tx now rolls
		// back (no partial stock commit) and the failed-status audit is recorded
		// out-of-tx afterwards (epic #38, Part 6 — see processSubmission/recordFailure).
		var failedPS *processingState
		txErr := s.baseRepo.WithinTx(ctx, func(txCtx context.Context) error {
			if err := s.inventorySubmissionRepo.UpdateApprovalStatus(txCtx, submission.ID, approvalStatus, req.Reason); err != nil {
				return fmt.Errorf("failed to update approval status: %w", err)
			}
			if err := s.inventorySubmissionRepo.AcquireInventoryAdvisoryLock(txCtx, submission.InventoryID); err != nil {
				return fmt.Errorf("failed to acquire inventory advisory lock: %w", err)
			}
			applied, ps, applyErr := s.processSubmission(txCtx, submission, true /* atomic */)
			if applyErr != nil {
				// ATOMIC apply path (epic #38, Part 6): the apply enlists in THIS tx
				// via the tx-aware repos, so a failed apply may already have flushed a
				// partial stock mutation into the open tx. We MUST roll the whole tx
				// back — returning the error aborts WithinTx so neither the partial
				// inventory_items/transactions writes NOR the approval-status flip
				// commit. (When the repo owned its own tx, a failed save rolled back on
				// its own; now that it enlists, the rollback responsibility is ours.)
				// The failed-status audit write is deferred to AFTER the tx unwinds
				// (failedPS.recordFailure below) so it neither deadlocks against this
				// tx's row lock nor gets erased by the rollback.
				failedPS = ps
				return applyErr
			}
			if applied {
				// Stamp processed_at inside the same tx as the stock change so a
				// concurrent reconcile drift re-check sees them atomically. The repo
				// stamps it with the DB clock (clock_timestamp()) so it shares a clock
				// with the snapshot created_at it is compared against (no host skew).
				if err := s.inventorySubmissionRepo.SetProcessedAt(txCtx, submission.ID); err != nil {
					return fmt.Errorf("failed to set processed_at: %w", err)
				}
			}
			return nil
		})
		if txErr != nil {
			// Persist the failure-audit trail now that the tx has rolled back and the
			// submission row lock is released (no partial stock committed).
			if failedPS != nil {
				failedPS.recordFailure(ctx)
			}
			return nil, txErr
		}
		// Refresh from the COMMITTED row before returning (epic #38, Part 6 — Codex
		// P2). `submission` was read by GetByID BEFORE the tx, so on a RETRY after an
		// earlier failed apply it still carries processing_status=failed + the old
		// error JSON. The committed tx cleared the error, set processing_status=
		// completed, and DB-stamped processed_at (SetProcessedAt/clock_timestamp()).
		// Patching only approval/reason on the stale struct would make the success
		// response echo the prior failure; reload so the response mirrors the
		// persisted state (status, cleared error, DB-stamped processed_at).
		refreshed, err := s.inventorySubmissionRepo.GetByID(ctx, submission.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to reload submission after approve: %w", err)
		}
		submission = refreshed
	case models.InventorySubmissionApprovalStatusRejected:
		// Reject under the parent FOR UPDATE lock with an AFTER-lock state re-check
		// (epic #38, Part 6 — closes the reject-vs-StartProcessing TOCTOU). The
		// submission was read above WITHOUT a lock; a concurrent StartProcessing on an
		// initiated reconcile can take the parent FOR UPDATE lock, consume stock, and
		// mark the row processed/approved/completed while this request is mid-flight.
		// Acting on the stale pre-lock read would flip an already-applied reconcile to
		// rejected/canceled, orphaning the consuming inventory transactions. So we
		// re-load the row under the lock and re-evaluate on the FRESH status: once the
		// initiated reconcile has left the staff-editable `open` state
		// (closed/processing/processed), the legacy reject is blocked — the admin must
		// reopen first, and a processed reconcile is terminal. Both this path and
		// StartProcessing serialize on the same parent row lock, so a reject can never
		// interleave between StartProcessing's lifecycle check and its apply.
		txErr := s.baseRepo.WithinTx(ctx, func(txCtx context.Context) error {
			locked, err := s.inventorySubmissionRepo.GetByIDForUpdate(txCtx, submission.ID)
			if err != nil {
				return fmt.Errorf("failed to re-load submission for reject: %w", err)
			}
			// Re-assert approval is still pending on the freshly-read, locked row: a
			// concurrent StartProcessing (approved+completed) or another reject may
			// have already terminalized it.
			if locked.ApprovalStatus != models.InventorySubmissionApprovalStatusPending {
				return pkg.NewAppError(pkg.ErrorCodeValidation,
					fmt.Sprintf("submission approval is not pending, current approval status: %s", locked.ApprovalStatus), nil)
			}
			// For an INITIATED reconcile (has snapshot rows), reject is only valid
			// while `open`. closed/processing/processed mean the admin took it out of
			// the staff-editable window or StartProcessing has applied it; rejecting
			// then would corrupt applied state. Legacy reconciles / dispose / transfer
			// carry no reconcile_status and are unaffected.
			if locked.SubmissionType == models.InventorySubmissionTypeReconcile &&
				locked.ReconcileStatus != "" &&
				locked.ReconcileStatus != models.ReconcileLifecycleStatusOpen {
				return pkg.ErrReconCannotRejectInLifecycle(txCtx, locked.ID, string(locked.ReconcileStatus))
			}

			if err := s.inventorySubmissionRepo.UpdateApprovalStatus(txCtx, submission.ID, approvalStatus, req.Reason); err != nil {
				return fmt.Errorf("failed to update approval status: %w", err)
			}
			// Set processing status to canceled when rejected.
			if err := s.inventorySubmissionRepo.UpdateProcessingStatus(txCtx, submission.ID, models.InventorySubmissionStatusCanceled); err != nil {
				return fmt.Errorf("failed to update processing status to canceled: %w", err)
			}
			return nil
		})
		if txErr != nil {
			return nil, txErr
		}
		// Reload the committed row so the response reflects the persisted state
		// (approval_status=rejected, processing_status=canceled) rather than a struct
		// read before the tx — mirrors the approve path above (epic #38, Part 6).
		refreshed, err := s.inventorySubmissionRepo.GetByID(ctx, submission.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to reload submission after reject: %w", err)
		}
		submission = refreshed
	case models.InventorySubmissionApprovalStatusPending:
		// No action needed for pending status
	}

	return submission, nil
}

// processSubmission executes the actual inventory operation based on submission
// type. It returns (applied, ps, err):
//   - applied is true iff the apply completed without errors (so the caller can
//     stamp processed_at);
//   - ps is the processing state; on the atomic path the caller calls
//     ps.recordFailure AFTER its tx unwinds to persist the failure-audit trail;
//   - err is the underlying apply error (the first accumulated error) when the
//     apply failed, and nil otherwise.
//
// When atomic is true (the approve tx, which enlists the apply via the tx-aware
// repos), returning the error lets the caller roll the whole transaction back on
// failure instead of committing a partial stock mutation alongside a failed
// status — the data-correctness fix for epic #38, Part 6. On that path ps.end
// does NOT write the failed status (it would deadlock against the open tx's row
// lock and be rolled back regardless); the caller records it post-tx via
// ps.recordFailure. When atomic is false (the legacy non-atomic path, repo owns
// its own tx), ps.end records completed/failed inline exactly as before.
func (s *inventoryService) processSubmission(ctx context.Context, submission *models.InventorySubmission, atomic bool) (bool, *processingState, error) {
	ps := newProcessingState(s, submission)
	ps.deferFailureToCaller = atomic
	defer ps.end(ctx)

	switch submission.SubmissionType {
	case models.InventorySubmissionTypeReconcile:
		var req dto.ReconcileInventoryRequest
		if err := json.Unmarshal(submission.Payload, &req); err != nil {
			return false, ps, ps.addError(fmt.Errorf("failed to unmarshal reconcile payload: %w", err))
		}
		_ = s.reconcileInventory(ctx, ps, req)
	case models.InventorySubmissionTypeDispose:
		var req dto.DisposeInventoryRequest
		if err := json.Unmarshal(submission.Payload, &req); err != nil {
			return false, ps, ps.addError(fmt.Errorf("failed to unmarshal dispose payload: %w", err))
		}
		_ = s.disposeInventory(ctx, ps, req)
	case models.InventorySubmissionTypeTransfer:
		var req dto.TransferInventoryRequest
		if err := json.Unmarshal(submission.Payload, &req); err != nil {
			return false, ps, ps.addError(fmt.Errorf("failed to unmarshal transfer payload: %w", err))
		}
		_ = s.transferInventory(ctx, ps, req)
	default:
		return false, ps, ps.addError(pkg.NewAppError(pkg.ErrorCodeValidation,
			fmt.Sprintf("unknown submission type: %s", submission.SubmissionType), nil))
	}
	if ps.hasAnyErrors() {
		return false, ps, ps.firstError()
	}
	return true, ps, nil
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

	// Transfer is intentionally NOT subject to the one-active-pending guard (epic
	// #38, Part 3 is reconcile-only per the human's decision): a pending reconcile
	// on the source inventory does not block a transfer, exactly as on main.
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
	transferTransactionCreator := func(consumeItem *models.InventoryItem, consumeTxn *models.InventoryTransaction, quantity decimal.Decimal) []*models.InventoryTransaction {
		txns := []*models.InventoryTransaction{
			{
				InventoryItemID:      consumeItem.ID,
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
		change, ok := destIvtrItemChanges[consumeItem.ProductID]
		if ok {
			change.Quantity = change.Quantity.Add(quantity)
			s.linkTxnWithInventoryItem(txns[1], change.InventoryItem)
			return txns
		}

		// check existing destination item so that we can create new
		// or udpate existing inventory item
		destItem, ok := destItemMap[consumeItem.ProductID]
		if !ok {
			destItem = &models.InventoryItem{
				InventoryID: req.DestinationInventoryID,
				ProductID:   consumeItem.ProductID,
				Status:      models.InventoryItemStatusActive,
				UnitID:      consumeItem.UnitID, // use same unit as source item
			}
		}

		// create new destination change
		change = &models.InventoryItemChange{
			InventoryItem:    destItem,
			OriginalQuantity: destItem.Quantity,
		}
		destIvtrItemChanges[consumeItem.ProductID] = change

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
	// deferFailureToCaller is set on the ATOMIC apply path (the approve tx, which
	// enlists the apply via the tx-aware repos). On that path ps.end must NOT write
	// the failed processing status itself: that write would run on ps.end's OWN base
	// connection while the still-open apply tx holds a row lock on this same
	// submission (taken by UpdateApprovalStatus), self-deadlocking; and even if it
	// did not, the apply tx is about to roll back, so an in-tx failed write would be
	// erased. Instead the caller records the failure AFTER the tx unwinds, on a
	// clean connection (epic #38, Part 6 — data-correctness fix). The legacy
	// non-atomic path leaves this false and keeps recording failures inline.
	deferFailureToCaller bool
}

func (s *processingState) hasAnyErrors() bool {
	return len(s.errors) > 0
}

// firstError returns the earliest accumulated apply error (the root cause), or
// nil if none. The atomic approve path returns this from its tx closure to force
// a full rollback while surfacing a meaningful error to the caller.
func (s *processingState) firstError() error {
	if len(s.errors) == 0 {
		return nil
	}
	return s.errors[0]
}

func (s *processingState) addError(err error) error {
	s.errors = append(s.errors, err)
	return err
}

func (s *processingState) end(ctx context.Context) {
	if s.hasAnyErrors() {
		// On the atomic apply path the caller records the failure after the tx
		// unwinds (see deferFailureToCaller); writing it here would deadlock against
		// the open tx's row lock and be rolled back anyway.
		if s.deferFailureToCaller {
			return
		}
		if err := s.svc.inventorySubmissionRepo.FailSubmissionProcessingWithErrors(ctx, s.submission.ID, s.errors); err != nil {
			log.Error("failed to fail submission processing with errors: %w", err)
		}
		return
	}

	if err := s.svc.inventorySubmissionRepo.UpdateProcessingStatus(ctx, s.submission.ID, models.InventorySubmissionStatusCompleted); err != nil {
		log.Error("failed to update submission status: %w", err)
	}
}

// recordFailure persists processing_status=failed + the accumulated errors on the
// repo's own connection. The atomic approve path calls this AFTER its tx has
// rolled back so the failure-audit trail survives the rollback without
// contending on the (now-released) row lock.
func (s *processingState) recordFailure(ctx context.Context) {
	if !s.hasAnyErrors() {
		return
	}
	if err := s.svc.inventorySubmissionRepo.FailSubmissionProcessingWithErrors(ctx, s.submission.ID, s.errors); err != nil {
		log.Error("failed to fail submission processing with errors: %w", err)
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
	// synthesized holds, for each ACTIVE reconcile (initiated via the new flow,
	// payload still empty), the read-only synthesis over its staff child rows
	// (epic #38, Part 5 / S4). Such a submission's parent Payload is empty until
	// apply, so its items/label/warnings are derived from the child rows instead.
	synthesized := make(map[uint]*dto.SynthesizedReconcile)
	for _, submission := range submissions {
		if isActiveReconcile(submission) {
			syn, err := s.SynthesizeSubmissionPayload(ctx, submission.ID)
			if err != nil {
				return nil, 0, fmt.Errorf("failed to synthesize reconcile submission %d: %w", submission.ID, err)
			}
			synthesized[submission.ID] = syn
			submissionItemMap[submission.ID] = syn.Request.Items
			itemIDs = append(itemIDs, models.GetIDs(syn.Request.Items)...)
			continue
		}

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

		// Warnings reuse the shared formatWarnings reconcile branch, which compares
		// PrevQuantity (the snapshot baseline, set by synthesis) vs the live item
		// quantity — exactly the B2 snapshot-vs-live drift view.
		warnings := formatWarnings(submission, items, inventoryItemMap)

		var label dto.ReconcileReviewLabel
		var countBreakdown []dto.ReconcileItemBreakdown
		if syn, ok := synthesized[submission.ID]; ok {
			label = syn.Label
			// Surface synthesis anomalies (e.g. a stored aggregate exceeding its
			// snapshot baseline) alongside the drift warnings so the admin sees the
			// data oddity rather than it being silently corrected.
			warnings = append(warnings, syn.Anomalies...)
			// Per-(item, label) breakdown behind each summed line (issue #73), so the
			// review screen can show each labeled count, not just the total. Resolve
			// product_name the same way formatSubmissionItems does (inventory_item ->
			// product), gracefully leaving it empty when the product can't be resolved
			// (FE #42).
			countBreakdown = make([]dto.ReconcileItemBreakdown, len(syn.Breakdown))
			for j, b := range syn.Breakdown {
				if inventoryItem, exists := inventoryItemMap[b.InventoryItemID]; exists && inventoryItem.Product != nil {
					b.ProductName = inventoryItem.Product.Name
				}
				countBreakdown[j] = b
			}
		}

		responses[i] = dto.SubmissionResponse{
			ID:              submission.ID,
			InventoryID:     submission.InventoryID,
			Inventory:       submission.Inventory,
			SubmissionType:  submission.SubmissionType,
			Status:          submission.ProcessingStatus,
			ApprovalStatus:  submission.ApprovalStatus,
			Errors:          s.formatProcessingErrors(submission.Error),
			Warnings:        warnings,
			Items:           items,
			ReviewLabel:     label,
			CountBreakdown:  countBreakdown,
			ReconcileStatus: submission.ReconcileStatus,
			Reason:          submission.Reason,
			CreatedBy:       submission.CreatedBy,
			CreatedAt:       submission.CreatedAt.Format(pkg.DateTimeFormat),
			UpdatedBy:       submission.UpdatedBy,
			UpdatedAt:       submission.UpdatedAt.Format(pkg.DateTimeFormat),
		}
	}

	return responses, total, nil
}

// ListReconciliationsAwaitingProcessing returns the cross-inventory awaiting-
// processing reconcile queue (issue #88). Auth is enforced here in the service
// (recon_item_view => 403 otherwise), NOT via route middleware. recon_item_view is
// the shared read action held by staff AND admin/accountant (recon_manage is
// admin/accountant-only); the count Page is staff-facing, so staff must see the
// same active-reconcile queue to reach their open reconciliations. The list is the
// same for every caller — no per-staff scoping — so a single shared read gate is
// all that's needed (unlike ListReconciliationItems, which scopes staff to their
// own child rows). Every row this returns is, by predicate, an active reconcile
// (pending + open/closed), so the per-inventory ListSubmissions path would
// synthesize each row (3 queries/row) — a per-page N+1 on this cross-inventory
// surface. This queue does not need the synthesized review breakdown (that is the
// per-inventory detail/review screen's concern, served by the existing
// GET /{id}/submissions), so it maps directly from the loaded row + the preloaded
// Inventory with ZERO per-row DB work: the page is a bounded, constant query count
// regardless of row count.
func (s *inventoryService) ListReconciliationsAwaitingProcessing(ctx context.Context, params models.ListParams, reconcileStatuses []string) ([]dto.SubmissionResponse, int64, error) {
	if !pkg.HasPermission(ctx, pkg.RBACResourceInventorySubmissions, pkg.RBACActionReconItemView) {
		return nil, 0, pkg.ErrForbidden("user does not have permission to view reconciliations", nil)
	}

	submissions, total, err := s.inventorySubmissionRepo.ListReconciliationsAwaitingProcessing(ctx, params, reconcileStatuses)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list awaiting-processing reconciliations: %w", err)
	}

	responses := make([]dto.SubmissionResponse, len(submissions))
	for i, submission := range submissions {
		responses[i] = mapReconcileQueueRow(submission)
	}

	return responses, total, nil
}

// mapReconcileQueueRow is the LIGHTWEIGHT queue mapper for the awaiting-processing
// reconcile queue (issue #88). Unlike the ListSubmissions mapper it does NOT call
// SynthesizeSubmissionPayload (which is a 3-query-per-row N+1 on this cross-
// inventory surface): it maps directly from the loaded row + preloaded Inventory,
// doing zero per-row DB work. The synthesized review fields (review_label,
// count_breakdown) and the synthesized items/warnings that ride the same call are
// the per-inventory detail/review screen's concern (served by GET /{id}/submissions
// on drill-in) and are intentionally dropped here — left as their zero values, so
// review_label/count_breakdown/warnings (all omitempty) simply do not appear in the
// JSON. Items is NOT omitempty, so it is initialized to an empty slice to serialize
// as `"items":[]` rather than null.
func mapReconcileQueueRow(submission models.InventorySubmission) dto.SubmissionResponse {
	return dto.SubmissionResponse{
		ID:              submission.ID,
		InventoryID:     submission.InventoryID,
		Inventory:       submission.Inventory,
		SubmissionType:  submission.SubmissionType,
		Status:          submission.ProcessingStatus,
		ApprovalStatus:  submission.ApprovalStatus,
		Items:           []dto.QuantityItem{},
		ReconcileStatus: submission.ReconcileStatus,
		Reason:          submission.Reason,
		CreatedBy:       submission.CreatedBy,
		CreatedAt:       submission.CreatedAt.Format(pkg.DateTimeFormat),
		UpdatedBy:       submission.UpdatedBy,
		UpdatedAt:       submission.UpdatedAt.Format(pkg.DateTimeFormat),
	}
}

// isActiveReconcile reports whether a submission is an in-flight reconciliation
// initiated via the new flow whose parent payload is still empty (epic #38, Part
// 5 / S4). Such submissions must render their items/label/warnings by synthesizing
// over the staff child rows rather than reading the empty Payload. The gate is
// intentionally cheap (no per-row query): a legacy single-payload reconcile carries
// a non-empty Payload and so is excluded and keeps its existing behavior; an
// applied/approved or rejected reconcile is likewise excluded (payload populated at
// apply, or never reaching apply). Synthesis itself then reads the child rows and
// snapshot; a reconcile with neither simply yields an empty item set.
func isActiveReconcile(submission models.InventorySubmission) bool {
	return submission.SubmissionType == models.InventorySubmissionTypeReconcile &&
		submission.ApprovalStatus == models.InventorySubmissionApprovalStatusPending &&
		len(payloadItemsRaw(submission.Payload)) == 0
}

// payloadItemsRaw returns the raw bytes of the payload only when it is non-empty
// and parses to at least one item; an empty/`null`/`{}`/zero-item payload yields
// nil so isActiveReconcile treats it as "no persisted payload yet".
func payloadItemsRaw(payload json.RawMessage) json.RawMessage {
	if len(payload) == 0 {
		return nil
	}
	var parsed struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(payload, &parsed); err != nil {
		// Unparseable payload: treat as present (non-active) so we don't override a
		// legacy/foreign payload via synthesis.
		return payload
	}
	if len(parsed.Items) == 0 {
		return nil
	}
	return payload
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

	// Block legacy overwrite of reconciliations started via the initiate endpoint
	// (snapshot-based flow); a legacy payload would bypass the snapshot baseline.
	if err := s.guardNotInitiatedReconcile(ctx, submission); err != nil {
		return nil, err
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
		ID:              submission.ID,
		InventoryID:     submission.InventoryID,
		Inventory:       submission.Inventory,
		SubmissionType:  submission.SubmissionType,
		Status:          submission.ProcessingStatus,
		ApprovalStatus:  submission.ApprovalStatus,
		Items:           items,
		Warnings:        formatWarnings(*submission, req.Items, inventoryItemMap),
		ReconcileStatus: submission.ReconcileStatus,
		Reason:          submission.Reason,
		CreatedBy:       submission.CreatedBy,
		CreatedAt:       submission.CreatedAt.Format(pkg.DateTimeFormat),
		UpdatedBy:       submission.UpdatedBy,
		UpdatedAt:       submission.UpdatedAt.Format(pkg.DateTimeFormat),
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

func (s *inventoryService) GetMonthlyTransactionReport(ctx context.Context, inventoryID uint, month, year int) (*models.TxnReportInventory, error) {
	inventory, err := s.inventoryRepo.GetByID(ctx, inventoryID)
	if err != nil {
		return nil, fmt.Errorf("failed to get inventory: %w", err)
	}

	// Use provided month/year or default to current month
	var from, to time.Time
	if month == 0 || year == 0 {
		// Default to current month
		from = pkg.GetMonthStart(0)
		to = pkg.GetMonthStart(1)
	} else {
		// Use provided month and year
		from = time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
		to = from.AddDate(0, 1, 0) // First day of next month
	}

	// 1. Get ALL transactions in period
	txns, err := s.inventoryRepo.GetTransactionsByInventoryIDs(ctx, inventoryID, &from, &to)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction report data: %w", err)
	}

	// Check if there are any transactions
	if len(txns) == 0 {
		return nil, pkg.ErrNoTransactionsInReportPeriod(ctx)
	}

	// 2. Get consume transactions in period
	consumeTxns := make([]*models.InventoryTransaction, 0)
	periodSourceTxns := make([]*models.InventoryTransaction, 0)
	for _, txn := range txns {
		switch txn.TransactionType {
		case models.InventoryTransactionTypeSell,
			models.InventoryTransactionTypeDisposal,
			models.InventoryTransactionTypeTransferOut:
			consumeTxns = append(consumeTxns, txn)
		case models.InventoryTransactionTypePurchase,
			models.InventoryTransactionTypeTransferIn:
			periodSourceTxns = append(periodSourceTxns, txn)
		}
	}

	// 3. Extract counter_transaction_ids from consume transactions
	sourceIDsMap := make(map[uint]bool)
	for _, consume := range consumeTxns {
		if consume.CounterTransactionID != nil {
			sourceIDsMap[*consume.CounterTransactionID] = true
		}
	}
	sourceIDs := make([]uint, 0, len(sourceIDsMap))
	for id := range sourceIDsMap {
		sourceIDs = append(sourceIDs, id)
	}

	// 4. Get historical source transactions referenced by consumes
	historicalSourceTxns, err := s.inventoryRepo.GetTransactionsByIDs(ctx, sourceIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get historical source transactions: %w", err)
	}

	// 5. Merge and deduplicate source transactions
	allSourceTxns := s.mergeAndDeduplicateSourceTxns(historicalSourceTxns, periodSourceTxns)

	// build inventory item lookup from all transactions
	iiLookup, err := s.getInventoryItemLookup(ctx, txns)
	if err != nil {
		return nil, fmt.Errorf("failed to build inventory item lookup: %w", err)
	}

	// build purchase order item lookup from all transactions
	poItemLookup, err := s.getPurchaseOrderItemsLookup(ctx, txns)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch purchase order items: %w", err)
	}

	// get historical transactions to calculate starting quantities
	historicalTxns, err := s.inventoryRepo.GetTransactionsByInventoryIDs(ctx, inventoryID, nil, &from)
	if err != nil {
		return nil, fmt.Errorf("failed to get historical transactions: %w", err)
	}

	rb, err := models.NewReportBuilder(inventory, from, to).
		Txns(txns).
		ConsumeTxns(consumeTxns).
		SourceTxns(allSourceTxns).
		HistoricalTxns(historicalTxns).
		InventoryItemLookup(iiLookup).
		PurchaseOrderItemLookup(poItemLookup).
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build report: %w", err)
	}

	r, err := rb.GetOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get report output: %w", err)
	}

	// format XLSX content
	formatter := excel.NewTxnReportFormatter()
	content, err := formatter.FormatToXLSX(r)
	if err != nil {
		return nil, fmt.Errorf("failed to format report as XLSX: %w", err)
	}
	r.ExportFile.Content = content
	// @todo: debug save excelize file here

	if err := s.fileStorageService.PopulateExportURL(ctx, r.ExportFile); err != nil {
		return nil, fmt.Errorf("failed to populate export url")
	}

	return r, nil
}

// mergeAndDeduplicateSourceTxns merges historical and period source transactions and removes duplicates
func (s *inventoryService) mergeAndDeduplicateSourceTxns(
	historicalSources []*models.InventoryTransaction,
	periodSources []*models.InventoryTransaction,
) []*models.InventoryTransaction {
	txnMap := make(map[uint]*models.InventoryTransaction)

	// Add historical sources first
	for _, txn := range historicalSources {
		txnMap[txn.ID] = txn
	}

	// Add period sources (will overwrite if duplicate, but that's fine)
	for _, txn := range periodSources {
		txnMap[txn.ID] = txn
	}

	// Convert map to slice
	result := make([]*models.InventoryTransaction, 0, len(txnMap))
	for _, txn := range txnMap {
		result = append(result, txn)
	}

	return result
}

// getInventoryItemLookup builds a lookup map of inventory items by their IDs from the given transactions.
func (s *inventoryService) getInventoryItemLookup(
	ctx context.Context,
	txns []*models.InventoryTransaction,
) (map[uint]*models.InventoryItem, error) {
	itemIDs := make([]uint, 0)
	for _, txn := range txns {
		if txn.InventoryItemID != 0 {
			itemIDs = append(itemIDs, txn.InventoryItemID)
		}
	}
	inventoryItems, err := s.inventoryItemRepo.GetByIDs(ctx, itemIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get inventory items: %w", err)
	}
	return models.BuildIDMap(inventoryItems), nil
}

// fetchPurchaseOrderItemsLookup batch fetches purchase order items and builds a lookup map.
func (s *inventoryService) getPurchaseOrderItemsLookup(
	ctx context.Context,
	txns []*models.InventoryTransaction,
) (map[uint]*models.PurchaseOrderItem, error) {
	if len(txns) == 0 {
		return make(map[uint]*models.PurchaseOrderItem), nil
	}

	poItemIDs := []uint{}
	poItemIDSet := make(map[uint]bool)
	for _, txn := range txns {
		if txn.PurchaseOrderItemID != nil && !poItemIDSet[*txn.PurchaseOrderItemID] {
			poItemIDs = append(poItemIDs, *txn.PurchaseOrderItemID)
			poItemIDSet[*txn.PurchaseOrderItemID] = true
		}
	}

	var poItems []*models.PurchaseOrderItem
	err := s.db.WithContext(ctx).
		Preload("PurchaseOrder").
		Where("id IN ?", poItemIDs).
		Find(&poItems).Error
	if err != nil {
		return nil, fmt.Errorf("failed to fetch purchase order items: %w", err)
	}

	// Build lookup map
	lookup := make(map[uint]*models.PurchaseOrderItem)
	for _, poItem := range poItems {
		lookup[poItem.ID] = poItem
	}

	return lookup, nil
}
