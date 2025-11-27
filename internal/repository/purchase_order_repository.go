package repository

import (
	"cim-backend/internal/models"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

//go:generate mockery --name=PurchaseOrderRepository --structname=PurchaseOrderRepository --output=../mocks/repositorymocks --outpkg=repositorymocks
type PurchaseOrderRepository interface {
	Create(ctx context.Context, purchaseOrder *models.PurchaseOrder) error
	Update(ctx context.Context, purchaseOrder *models.PurchaseOrder) error
	Delete(id uint) error
	List(ctx context.Context, params models.ListParams) ([]models.PurchaseOrder, int64, error)
	GetByStatus(status string) ([]models.PurchaseOrder, error)
	UpdateStatus(ctx context.Context, purchaseOrderID uint, status models.PurchaseOrderStatus) error

	// v1
	GetByID(id uint) (*models.PurchaseOrder, error)
	UpdatePurchaseOrder(ctx context.Context, id uint, req dto.UpdatePurchaseOrderRequest) (*models.PurchaseOrder, error)
	ReceiveInventory(ctx context.Context, req dto.UpdatePurchaseOrderDeliveryStatusRequest) (*models.PurchaseOrder, error)
	GetPurchaseOrderItemByID(ctx context.Context, purchaseOrderID, itemID uint) (*models.PurchaseOrderItem, error)
}

type purchaseOrderRepository struct {
	db *gorm.DB
}

func NewPurchaseOrderRepository(db *gorm.DB) PurchaseOrderRepository {
	return &purchaseOrderRepository{db: db}
}

func (r *purchaseOrderRepository) Create(ctx context.Context, purchaseOrder *models.PurchaseOrder) error {
	return r.db.WithContext(ctx).Create(purchaseOrder).Error
}

func (r *purchaseOrderRepository) GetByID(id uint) (*models.PurchaseOrder, error) {
	var purchaseOrder models.PurchaseOrder
	err := r.db.Preload("Items").
		Preload("Items.Product").
		Preload("Items.Supplier").
		Preload("Items.Unit").
		Preload("Items.Unit.BaseUnit").
		Preload("Items.Unit.DerivedUnits").
		Preload("Inventory").
		First(&purchaseOrder, "id = ?", id).Error
	if err != nil {
		return nil, err
	}

	// Load all derived units in the hierarchy for each item's unit
	ctx := context.Background()
	for _, item := range purchaseOrder.Items {
		if item.Unit != nil {
			// Load the full base unit chain
			visited := make(map[uint]bool)
			r.loadBaseUnitChain(ctx, item.Unit, visited)

			// Find the root base unit ID for this unit
			rootBaseUnitID := item.Unit.GetRootBaseUnitID()

			// Populate DerivedUnits with all units in the hierarchy from the root base unit
			allDerivedUnits, err := r.findAllUnitsWithRootBaseUnit(ctx, rootBaseUnitID, item.Unit.UnitType)
			if err == nil {
				item.Unit.DerivedUnits = allDerivedUnits
			}
		}
	}

	return &purchaseOrder, nil
}

func (r *purchaseOrderRepository) Update(ctx context.Context, purchaseOrder *models.PurchaseOrder) error {
	return r.db.WithContext(ctx).Save(purchaseOrder).Error
}

func (r *purchaseOrderRepository) UpdateStatus(ctx context.Context, purchaseOrderID uint, status models.PurchaseOrderStatus) error {
	return r.db.WithContext(ctx).Model(&models.PurchaseOrder{}).Where("id = ?", purchaseOrderID).Where("status != ?", models.PurchaseOrderStatusCompleted).Update("status", status).Error
}

func (r *purchaseOrderRepository) Delete(id uint) error {
	return r.db.Delete(&models.PurchaseOrder{}, "id = ?", id).Error
}

func (r *purchaseOrderRepository) List(ctx context.Context, params models.ListParams) ([]models.PurchaseOrder, int64, error) {
	var purchaseOrders []models.PurchaseOrder
	var total int64

	// Build base query for both count and data
	baseQuery := r.db.WithContext(ctx).Model(&models.PurchaseOrder{}).Preload("Inventory")

	// Apply search filter
	// Uses indexes:
	// - idx_purchase_order_items_purchase_order_id for EXISTS subquery lookups
	// - idx_suppliers_name_trgm (GIN trigram) for ILIKE pattern matching on supplier names
	// - order_number has unique constraint (automatic index) for order_number searches
	if params.Search != "" {
		searchPattern := "%" + params.Search + "%"
		// EXISTS subquery uses index on purchase_order_items.purchase_order_id
		// GIN trigram index on suppliers.name is automatically used for ILIKE queries
		baseQuery = baseQuery.Where(
			"order_number ILIKE ? OR notes ILIKE ? OR EXISTS (SELECT 1 FROM purchase_order_items poi JOIN suppliers s ON poi.supplier_id = s.id WHERE poi.purchase_order_id = purchase_orders.id AND s.name ILIKE ?)",
			searchPattern, searchPattern, searchPattern,
		)
	}

	// Apply date range filter
	if params.StartDate != "" || params.EndDate != "" {
		if params.StartDate != "" {
			startTime, err := time.Parse("2006-01-02", params.StartDate)
			if err != nil {
				return nil, 0, fmt.Errorf("invalid start_date format, expected YYYY-MM-DD: %w", err)
			}
			baseQuery = baseQuery.Where("created_at >= ?", startTime)
		}
		if params.EndDate != "" {
			endTime, err := time.Parse("2006-01-02", params.EndDate)
			if err != nil {
				return nil, 0, fmt.Errorf("invalid end_date format, expected YYYY-MM-DD: %w", err)
			}
			// Add one day to include the entire end date
			endTime = endTime.Add(24 * time.Hour)
			baseQuery = baseQuery.Where("created_at < ?", endTime)
		}
	}

	// Handle status filtering
	var statuses []models.PurchaseOrderStatus
	if params.Status != "" {
		// Parse comma-separated status values
		statusStrings := strings.Split(params.Status, ",")
		statuses = make([]models.PurchaseOrderStatus, 0, len(statusStrings))
		for _, statusStr := range statusStrings {
			trimmedStatus := strings.TrimSpace(statusStr)
			if trimmedStatus != "" {
				statuses = append(statuses, models.PurchaseOrderStatus(trimmedStatus))
			}
		}
	} else {
		// Create a copy of all statuses to avoid modifying the global slice
		statuses = make([]models.PurchaseOrderStatus, len(models.AllPurchaseOrderStatuses))
		copy(statuses, models.AllPurchaseOrderStatuses)
	}

	if !pkg.HasPermission(ctx, "purchase-orders", "view_status_completed") {
		statuses = slices.DeleteFunc(statuses, func(status models.PurchaseOrderStatus) bool {
			return status == models.PurchaseOrderStatusCompleted || status == models.PurchaseOrderStatusCancelled
		})
	}

	baseQuery = baseQuery.Where("status IN ?", statuses)
	baseQuery = baseQuery.Preload("Items").Preload("Items.Product").Preload("Items.Supplier").Preload("Items.Unit")

	// Check if user has permission to view prices
	if !pkg.HasPermission(ctx, "prices", "view") {
		baseQuery.Preload("Items", func(db *gorm.DB) *gorm.DB {
			return db.Omit("unit_price")
		})
	}

	// Get total count first
	err := baseQuery.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// Apply sorting
	if params.Sort != "" {
		orderDirection := "ASC"
		if params.Order == "desc" {
			orderDirection = "DESC"
		}

		switch params.Sort {
		case "order_number", "status", "total_amount", "created_at", "updated_at":
			baseQuery = baseQuery.Order(params.Sort + " " + orderDirection)
		default:
			baseQuery = baseQuery.Order("created_at DESC") // Default sorting
		}
	} else {
		baseQuery = baseQuery.Order("created_at DESC") // Default sorting
	}

	err = baseQuery.Limit(params.Limit).Offset(params.GetOffset()).
		Find(&purchaseOrders).Error
	if err != nil {
		return nil, 0, fmt.Errorf("failed to fetch purchase orders: %w", err)
	}
	return purchaseOrders, total, err
}

func (r *purchaseOrderRepository) GetByStatus(status string) ([]models.PurchaseOrder, error) {
	var purchaseOrders []models.PurchaseOrder
	err := r.db.Preload("Items").Preload("Items.Product").Where("status = ?", status).Find(&purchaseOrders).Error
	return purchaseOrders, err
}

// AnyDeliveringItem checks if all items in a purchase order are delivered
func (r *purchaseOrderRepository) AnyDeliveringItem(ctx context.Context, purchaseOrderID uint) bool {
	// Get count of delivered items
	err := r.db.WithContext(ctx).Model(&models.PurchaseOrderItem{}).
		Where("purchase_order_id = ? AND status != ?", purchaseOrderID, models.PurchaseOrderItemStatusDelivered).
		First(&models.PurchaseOrderItem{}).Error
	return err == nil
}

// ReceiveInventory updates purchase order delivery status, creating inventory items and transactions
func (r *purchaseOrderRepository) ReceiveInventory(ctx context.Context, req dto.UpdatePurchaseOrderDeliveryStatusRequest) (*models.PurchaseOrder, error) {
	var po *models.PurchaseOrder
	return po, r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", req.PurchaseOrderID).
			Where("status NOT IN ?", []any{models.PurchaseOrderStatusCancelled}).
			Find(&po).Error; err != nil {
			return pkg.NewAppError(pkg.ErrorCodeNotFound,
				fmt.Sprintf("purchase order ID %d not found", req.PurchaseOrderID), nil)
		}

		type POIData struct {
			*models.PurchaseOrderItem
			InventoryItemID      *uint
			Product              *models.Product `gorm:"embedded;embeddedPrefix:product_"`
			UnitID               uint
			UnitName             string
			UnitSymbol           string
			UnitType             string
			UnitBaseUnitID       *uint
			UnitConversionFactor float64
			UnitDecimalPlaces    int
		}

		var poiData []POIData
		err := tx.Table("purchase_order_items poi").
			Select(`
			poi.*,
			ii.id as inventory_item_id,
			p.id as product_id,
			p.name as product_name,
			p.description as product_description,
			p.product_type as product_product_type,
			p.status as product_status,
			p.unit_id as product_unit_id,
			u.id as unit_id,
			u.name as unit_name,
			u.symbol as unit_symbol,
			u.unit_type as unit_type,
			u.base_unit_id as unit_base_unit_id,
			u.conversion_factor as unit_conversion_factor,
			u.decimal_places as unit_decimal_places
		`).
			Joins(`LEFT JOIN inventory_items ii ON poi.product_id = ii.product_id
				AND ii.status = ?
		`, models.InventoryItemStatusActive).
			Joins(`JOIN products p ON poi.product_id = p.id`).
			Joins(`JOIN units u ON p.unit_id = u.id`).
			Where("poi.purchase_order_id = ?", req.PurchaseOrderID).
			Scan(&poiData).Error
		if err != nil {
			return fmt.Errorf("failed to query purchase order items with inventory items: %w", err)
		}

		if len(poiData) == 0 {
			return pkg.NewAppError(pkg.ErrorCodeNotFound, "no purchase order items found", nil)
		}

		poItemMap := make(map[uint]*POIData)
		for i := range poiData {
			poItemMap[poiData[i].ID] = &poiData[i]
		}

		// Step 3: Process dto items and build transactions and new inventory items
		var transactions []*models.InventoryTransaction
		var newInventoryItems []*models.InventoryItem
		var updateInvetoryDeltas = make(map[uint]decimal.Decimal)

		for _, dtoItem := range req.Items {
			// Find corresponding purchase order item data
			poItem, exists := poItemMap[dtoItem.ID]
			if !exists {
				return pkg.NewAppError(pkg.ErrorCodeNotFound, fmt.Sprintf("purchase order item with ID %d not found", dtoItem.ID), nil)
			}

			// Validate received quantity doesn't exceed remaining quantity
			remainingQuantity := poItem.Quantity.Sub(poItem.ReceivedQuantity)
			if dtoItem.ReceivedQuantity.GreaterThan(remainingQuantity) {
				return pkg.NewAppError(pkg.ErrorCodeValidation,
					fmt.Sprintf("received quantity %s exceeds remaining quantity %s for item ID %d",
						dtoItem.ReceivedQuantity.String(), remainingQuantity.String(), dtoItem.ID), nil)
			}

			receivedQuantityDecimalPlaces := getDecimalPlaces(dtoItem.ReceivedQuantity)
			if receivedQuantityDecimalPlaces > poItem.UnitDecimalPlaces {
				return pkg.NewAppError(pkg.ErrorCodeValidation,
					fmt.Sprintf("decimal places must be less than or equal to %d", poItem.UnitDecimalPlaces), nil)
			}

			poItem.ReceivedQuantity = poItem.ReceivedQuantity.Add(dtoItem.ReceivedQuantity)
			poItem.PurchaseOrderItem.UpdateStatus()

			transaction := &models.InventoryTransaction{
				TransactionType:     models.InventoryTransactionTypePurchase,
				Price:               poItem.UnitPrice,
				Quantity:            dtoItem.ReceivedQuantity,
				PurchaseOrderItemID: poItem.PurchaseOrderID,
			}

			if poItem.InventoryItemID != nil {
				// Use existing inventory item
				transaction.InventoryItemID = *poItem.InventoryItemID
				if existing, ok := updateInvetoryDeltas[*poItem.InventoryItemID]; ok {
					updateInvetoryDeltas[*poItem.InventoryItemID] = existing.Add(dtoItem.ReceivedQuantity)
				} else {
					updateInvetoryDeltas[*poItem.InventoryItemID] = dtoItem.ReceivedQuantity
				}
			} else {
				transaction.InventoryItem = &models.InventoryItem{
					InventoryID: *po.InventoryID,
					ProductID:   *poItem.ProductID,
					Quantity:    dtoItem.ReceivedQuantity,
					Status:      models.InventoryItemStatusActive,
				}
				newInventoryItems = append(newInventoryItems, transaction.InventoryItem)
			}

			transaction.SupplierID = poItem.SupplierID
			transactions = append(transactions, transaction)
		}

		// convert poiData to slice of PurchaseOrderItem model and set to
		// PurchaseOrder field for updating status.
		poItems := make([]*models.PurchaseOrderItem, 0, len(poiData))
		for _, data := range poiData {
			if data.Product != nil {
				data.Product.UnitID = data.UnitID
				unit := &models.Unit{
					Name:             data.UnitName,
					Symbol:           data.UnitSymbol,
					UnitType:         data.UnitType,
					BaseUnitID:       data.UnitBaseUnitID,
					ConversionFactor: data.UnitConversionFactor,
				}
				unit.ID = data.UnitID
				data.Product.Unit = unit
			}
			// Ensure UnitID is set on purchase order item
			// If it's not set (from old data), use the product's unit_id as fallback
			if data.PurchaseOrderItem.UnitID == nil {
				data.PurchaseOrderItem.UnitID = &data.UnitID
			}
			data.PurchaseOrderItem.Product = data.Product
			poItems = append(poItems, data.PurchaseOrderItem)
		}

		// Persist data
		if len(newInventoryItems) > 0 {
			if err := tx.Create(newInventoryItems).Error; err != nil {
				return fmt.Errorf("failed to create new inventory items: %w", err)
			}
			for _, txn := range transactions {
				if txn.InventoryItem != nil && txn.InventoryItem.ID != 0 {
					txn.InventoryItemID = txn.InventoryItem.ID
				}
			}
		}

		if len(updateInvetoryDeltas) > 0 {
			err = r.increaseQuantityInventoryItems(tx, updateInvetoryDeltas)
			if err != nil {
				return fmt.Errorf("failed to increase quantity of inventory items: %w", err)
			}
		}

		if err := tx.Save(transactions).Error; err != nil {
			return fmt.Errorf("failed to save transaction: %w", err)
		}

		if err := tx.Save(poItems).Error; err != nil {
			return fmt.Errorf("failed to save purchase order items: %w", err)
		}

		po.Items = poItems
		now := time.Now()
		po.ConfirmedAt = &now
		po.ConfirmationNotes = req.ConfirmationNotes
		err = po.UpdateStatus(ctx)
		if err != nil {
			return fmt.Errorf("failed to calculate purchase order status: %w", err)
		}

		if err := tx.Save(po).Error; err != nil {
			return fmt.Errorf("failed to update purchase order status: %w", err)
		}

		return nil
	})
}

func (r *purchaseOrderRepository) increaseQuantityInventoryItems(db *gorm.DB, deltaMap map[uint]decimal.Decimal) error {
	values := make([]string, 0, len(deltaMap))
	for k, v := range deltaMap {
		values = append(values, fmt.Sprintf("(%d, %s)", k, v.String()))
	}
	valuesStr := strings.Join(values, ",")

	return db.Exec(fmt.Sprintf(`
		WITH payload (id, delta) AS ( VALUES %s )
		UPDATE inventory_items ii
			SET quantity = ii.quantity + payload.delta::decimal(10,2)
		FROM payload WHERE ii.id = payload.id;
	`, valuesStr)).Error
}

// UpdatePurchaseOrder updates a purchase order and its items while preserving ReceivedQuantity
func (r *purchaseOrderRepository) UpdatePurchaseOrder(ctx context.Context, id uint, req dto.UpdatePurchaseOrderRequest) (*models.PurchaseOrder, error) {
	var po *models.PurchaseOrder
	return po, r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Fetch existing purchase order with items
		if err := tx.Preload("Items").First(&po, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return pkg.NewAppError(pkg.ErrorCodeNotFound, fmt.Sprintf("purchase order with ID %d not found", id), nil)
			}
			return fmt.Errorf("failed to fetch purchase order: %w", err)
		}

		// Check if purchase order can be edited (not completed or cancelled)
		if po.Status == models.PurchaseOrderStatusCompleted || po.Status == models.PurchaseOrderStatusCancelled {
			return pkg.NewAppError(pkg.ErrorCodeValidation,
				fmt.Sprintf("cannot edit purchase order with status %s", po.Status), nil)
		}
		// Build a map of existing items by their ID to preserve ReceivedQuantity
		existingItemsMap := make(map[string]*models.PurchaseOrderItem)
		for _, item := range po.Items {
			if item != nil {
				// Create a unique key based on supplier_id and product_id
				key := fmt.Sprintf("%d-%d", *item.SupplierID, *item.ProductID)
				existingItemsMap[key] = item
			}
		}

		// Delete purchase order items if not found in the request
		// Build a map to track requested items by their unique key
		newItemsKeyMap := make(map[string]struct{}, len(req.Items))
		for _, itemReq := range req.Items {
			key := fmt.Sprintf("%d-%d", *itemReq.SupplierID, *itemReq.ProductID)
			newItemsKeyMap[key] = struct{}{}
		}

		// Find existing items to delete (i.e., items missing in newItemsKeyMap)
		// Do not delete items with received_quantity > 0
		var itemsToDeleteIDs []uint
		for key, existingItem := range existingItemsMap {
			if _, found := newItemsKeyMap[key]; !found && existingItem != nil {
				// Preserve items that have received quantity > 0
				if existingItem.ReceivedQuantity.GreaterThan(decimal.Zero) {
					return pkg.NewAppError(pkg.ErrorCodeValidation,
						fmt.Sprintf("cannot delete item with received quantity %s", existingItem.ReceivedQuantity.String()), nil)
				}
				itemsToDeleteIDs = append(itemsToDeleteIDs, existingItem.ID)
			}
		}

		// Find existing items and new items to upsert
		// Separate items that need database updates from items that don't need updating
		itemsToUpdate := make([]*models.PurchaseOrderItem, 0, len(req.Items))
		for _, itemReq := range req.Items {
			key := fmt.Sprintf("%d-%d", *itemReq.SupplierID, *itemReq.ProductID)
			if existingItem, found := existingItemsMap[key]; found {
				if existingItem.ReceivedQuantity.GreaterThan(itemReq.Quantity) {
					return pkg.NewAppError(
						pkg.ErrorCodeValidation,
						fmt.Sprintf("received quantity (%s) for product %d from supplier %d is greater than updated quantity (%s)", existingItem.ReceivedQuantity.String(), *itemReq.ProductID, *itemReq.SupplierID, itemReq.Quantity.String()),
						nil,
					)
				}

				if existingItem.ReceivedQuantity.GreaterThan(decimal.Zero) && existingItem.ReceivedQuantity.LessThan(itemReq.Quantity) {
					po.Status = models.PurchaseOrderStatusPartiallyDelivered
				}

				// Only update item if new quantity, unit price, or unit ID are different
				unitIDMatch := (existingItem.UnitID == nil && itemReq.UnitID == nil) ||
					(existingItem.UnitID != nil && itemReq.UnitID != nil && *existingItem.UnitID == *itemReq.UnitID)
				if existingItem.Quantity.Equal(itemReq.Quantity) && existingItem.UnitPrice == itemReq.UnitPrice && unitIDMatch {
					// Item doesn't need updating, skip it (will be reloaded from DB later)
					continue
				}

				existingItem.Quantity = itemReq.Quantity
				existingItem.UnitPrice = itemReq.UnitPrice
				if itemReq.UnitID != nil {
					existingItem.UnitID = itemReq.UnitID
				}
				existingItem.UpdateStatus()
				itemsToUpdate = append(itemsToUpdate, existingItem)
			} else {
				if po.Status == models.PurchaseOrderStatusFullyDelivered {
					po.Status = models.PurchaseOrderStatusPartiallyDelivered
				}
				newItem := &models.PurchaseOrderItem{
					PurchaseOrderID:  &id,
					ProductID:        itemReq.ProductID,
					SupplierID:       itemReq.SupplierID,
					UnitID:           itemReq.UnitID,
					UnitPrice:        itemReq.UnitPrice,
					Quantity:         itemReq.Quantity,
					ReceivedQuantity: decimal.Zero, // Default to 0
					Status:           models.PurchaseOrderItemStatusAwaitingDelivery,
				}
				itemsToUpdate = append(itemsToUpdate, newItem)
			}
		}

		// Update quantity and prices of purchase order items that need updating
		if len(itemsToUpdate) > 0 {
			for _, item := range itemsToUpdate {
				if err := tx.Save(item).Error; err != nil {
					return fmt.Errorf("failed to save purchase order item: %w", err)
				}
			}
		}

		// Delete items in one batch if any
		if len(itemsToDeleteIDs) > 0 {
			if err := tx.
				Where("id IN (?)", itemsToDeleteIDs).
				Delete(&models.PurchaseOrderItem{}).
				Error; err != nil {
				return fmt.Errorf("failed to delete removed purchase order items: %w", err)
			}
		}

		// Reload all remaining items from database to ensure we have complete, up-to-date list
		// This is important after deletion to get all remaining items with their current status
		var remainingItems []*models.PurchaseOrderItem
		if err := tx.Where("purchase_order_id = ?", id).Find(&remainingItems).Error; err != nil {
			return fmt.Errorf("failed to reload purchase order items: %w", err)
		}

		// Set items to purchase order (all remaining items)
		po.Items = remainingItems
		po.Notes = req.Notes

		// Always update status if there are items remaining (after deletion or update)
		if len(remainingItems) > 0 {
			if err := po.UpdateStatus(ctx); err != nil {
				return fmt.Errorf("failed to update purchase order status: %w", err)
			}
		}

		if err := tx.Save(po).Error; err != nil {
			return fmt.Errorf("failed to save purchase order: %w", err)
		}

		return nil
	})
}

// GetPurchaseOrderItemByID retrieves a purchase order item by purchase order ID and item ID
func (r *purchaseOrderRepository) GetPurchaseOrderItemByID(ctx context.Context, purchaseOrderID, itemID uint) (*models.PurchaseOrderItem, error) {
	var item models.PurchaseOrderItem
	err := r.db.WithContext(ctx).
		Preload("Product").
		Preload("Product.Unit").
		Preload("Unit").
		Preload("Unit.BaseUnit").
		Where("purchase_order_id = ? AND id = ?", purchaseOrderID, itemID).
		First(&item).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, pkg.NewAppError(pkg.ErrorCodeNotFound, fmt.Sprintf("purchase order item with ID %d not found in purchase order %d", itemID, purchaseOrderID), nil)
		}
		return nil, fmt.Errorf("failed to get purchase order item: %w", err)
	}
	return &item, nil
}

// loadBaseUnitChain recursively loads the base unit chain for multi-level hierarchies
// visited map prevents infinite loops in case of circular references
func (r *purchaseOrderRepository) loadBaseUnitChain(ctx context.Context, unit *models.Unit, visited map[uint]bool) {
	if unit.BaseUnitID == nil {
		return
	}
	// Safety check: prevent infinite loops
	if visited[unit.ID] {
		return
	}
	visited[unit.ID] = true

	// If base unit is not loaded, load it
	if unit.BaseUnit == nil {
		var baseUnit models.Unit
		if err := r.db.WithContext(ctx).
			Preload("BaseUnit").
			First(&baseUnit, "id = ?", *unit.BaseUnitID).Error; err != nil {
			return
		}
		unit.BaseUnit = &baseUnit
	}
	// Recursively load the base unit's chain
	r.loadBaseUnitChain(ctx, unit.BaseUnit, visited)
}

// findAllUnitsWithRootBaseUnit finds all units that have the same root base unit
// It recursively traverses the hierarchy to find all descendants
func (r *purchaseOrderRepository) findAllUnitsWithRootBaseUnit(ctx context.Context, rootBaseUnitID uint, unitType string) ([]*models.Unit, error) {
	var allUnits []*models.Unit
	visited := make(map[uint]bool)

	// Recursively collect all units in the hierarchy
	var collectUnits func(unitID uint) error
	collectUnits = func(unitID uint) error {
		if visited[unitID] {
			return nil
		}
		visited[unitID] = true

		// Find all units that have this unit as their base unit
		var directChildren []models.Unit
		if err := r.db.WithContext(ctx).
			Preload("BaseUnit").
			Where("base_unit_id = ? AND unit_type = ?", unitID, unitType).
			Find(&directChildren).Error; err != nil {
			return fmt.Errorf("failed to find direct children of unit %d: %w", unitID, err)
		}

		// Add direct children to the result and load their base unit chains
		for i := range directChildren {
			// Load the full base unit chain for each unit
			chainVisited := make(map[uint]bool)
			r.loadBaseUnitChain(ctx, &directChildren[i], chainVisited)
			allUnits = append(allUnits, &directChildren[i])
			// Recursively collect their children
			if err := collectUnits(directChildren[i].ID); err != nil {
				return err
			}
		}

		return nil
	}

	// Start collecting from the root base unit
	if err := collectUnits(rootBaseUnitID); err != nil {
		return nil, err
	}

	return allUnits, nil
}

// getDecimalPlaces returns the number of decimal places in a decimal.Decimal
// This counts the actual significant decimal places (e.g., "100.11" has 2, "100.00" has 0)
func getDecimalPlaces(d decimal.Decimal) int {
	// Convert to string and count decimal places
	// The String() method will show the actual decimal places without trailing zeros
	str := d.String()
	parts := strings.Split(str, ".")
	if len(parts) == 1 {
		return 0
	}
	// Count actual decimal places (trailing zeros are already removed by String())
	return len(parts[1])
}
