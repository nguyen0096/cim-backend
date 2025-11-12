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
	err := r.db.Preload("Items").Preload("Items.Product").Preload("Items.Supplier").Preload("Items.Unit").Preload("Inventory").First(&purchaseOrder, "id = ?", id).Error
	if err != nil {
		return nil, err
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
	if params.Search != "" {
		baseQuery = baseQuery.Where("order_number ILIKE ? OR notes ILIKE ?", "%"+params.Search+"%", "%"+params.Search+"%")
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

	statuses := models.AllPurchaseOrderStatuses

	if params.Status != "" {
		statuses = []models.PurchaseOrderStatus{models.PurchaseOrderStatus(params.Status)}
	}

	if !pkg.HasPermission(ctx, "purchase-orders", "view_status_completed") {
		statuses = slices.DeleteFunc(statuses, func(status models.PurchaseOrderStatus) bool {
			return status == models.PurchaseOrderStatusCompleted
		})
	}

	// Apply status filter
	if len(statuses) > 0 {
		baseQuery = baseQuery.Where("status IN ?", statuses)
	}

	// Apply same search filter for data
	if params.Search != "" {
		baseQuery = baseQuery.Where("order_number ILIKE ? OR notes ILIKE ?", "%"+params.Search+"%", "%"+params.Search+"%")
	}

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
			u.conversion_factor as unit_conversion_factor
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
		var updateInvetoryDeltas = make(map[uint]int)

		for _, dtoItem := range req.Items {
			// Find corresponding purchase order item data
			poItem, exists := poItemMap[dtoItem.ID]
			if !exists {
				return pkg.NewAppError(pkg.ErrorCodeNotFound, fmt.Sprintf("purchase order item with ID %d not found", dtoItem.ID), nil)
			}

			// Validate received quantity doesn't exceed remaining quantity
			remainingQuantity := poItem.Quantity - poItem.ReceivedQuantity
			if dtoItem.ReceivedQuantity > remainingQuantity {
				return pkg.NewAppError(pkg.ErrorCodeValidation,
					fmt.Sprintf("received quantity %d exceeds remaining quantity %d for item ID %d",
						dtoItem.ReceivedQuantity, remainingQuantity, dtoItem.ID), nil)
			}

			poItem.ReceivedQuantity += dtoItem.ReceivedQuantity
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
				updateInvetoryDeltas[*poItem.InventoryItemID] += dtoItem.ReceivedQuantity
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
		err = po.UpdateStatus()
		if err != nil {
			return fmt.Errorf("failed to calculate purchase order status: %w", err)
		}

		if err := tx.Save(po).Error; err != nil {
			return fmt.Errorf("failed to update purchase order status: %w", err)
		}

		return nil
	})
}

func (r *purchaseOrderRepository) increaseQuantityInventoryItems(db *gorm.DB, deltaMap map[uint]int) error {
	values := make([]string, 0, len(deltaMap))
	for k, v := range deltaMap {
		values = append(values, fmt.Sprintf("(%d, %d)", k, v))
	}
	valuesStr := strings.Join(values, ",")

	return db.Exec(fmt.Sprintf(`
		WITH payload (id, delta) AS ( VALUES %s )
		UPDATE inventory_items ii
			SET quantity = ii.quantity + payload.delta
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
		var itemsToDeleteIDs []uint
		for key, existingItem := range existingItemsMap {
			if _, found := newItemsKeyMap[key]; !found && existingItem != nil {
				itemsToDeleteIDs = append(itemsToDeleteIDs, existingItem.ID)
			}
		}

		// Find existing items and new items to upsert
		upsertItems := make([]*models.PurchaseOrderItem, 0, len(req.Items))
		for _, itemReq := range req.Items {
			key := fmt.Sprintf("%d-%d", *itemReq.SupplierID, *itemReq.ProductID)
			if existingItem, found := existingItemsMap[key]; found {
				if existingItem.ReceivedQuantity > itemReq.Quantity {
					return pkg.NewAppError(
						pkg.ErrorCodeValidation,
						fmt.Sprintf("received quantity (%d) for product %d from supplier %d is greater than updated quantity (%d)", existingItem.ReceivedQuantity, *itemReq.ProductID, *itemReq.SupplierID, itemReq.Quantity),
						nil,
					)
				}

				if existingItem.ReceivedQuantity > 0 && existingItem.ReceivedQuantity < itemReq.Quantity {
					po.Status = models.PurchaseOrderStatusPartiallyDelivered
				}

				// Only update item if new quantity, unit price, or unit ID are different
				unitIDMatch := (existingItem.UnitID == nil && itemReq.UnitID == nil) ||
					(existingItem.UnitID != nil && itemReq.UnitID != nil && *existingItem.UnitID == *itemReq.UnitID)
				if existingItem.Quantity == itemReq.Quantity && existingItem.UnitPrice == itemReq.UnitPrice && unitIDMatch {
					continue
				}

				existingItem.Quantity = itemReq.Quantity
				existingItem.UnitPrice = itemReq.UnitPrice
				if itemReq.UnitID != nil {
					existingItem.UnitID = itemReq.UnitID
				}
				existingItem.UpdateStatus()
				upsertItems = append(upsertItems, existingItem)
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
					ReceivedQuantity: 0, // Default to 0
					Status:           models.PurchaseOrderItemStatusAwaitingDelivery,
				}
				upsertItems = append(upsertItems, newItem)
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

		// Set items to purchase order
		po.Items = upsertItems
		po.Notes = req.Notes

		// Update quantity and prices of purchase order items
		for _, item := range upsertItems {
			if err := tx.Save(item).Error; err != nil {
				return fmt.Errorf("failed to save purchase order item: %w", err)
			}
		}

		if err := tx.Save(po).Error; err != nil {
			return fmt.Errorf("failed to save purchase order: %w", err)
		}

		return nil
	})
}
