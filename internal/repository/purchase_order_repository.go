package repository

import (
	"context"
	"fmt"
	"import-export-backend/internal/models"
	"import-export-backend/internal/services/dto"
	"import-export-backend/pkg"
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

	// v1
	GetByID(id uint) (*models.PurchaseOrder, error)
	ReceiveInventory(ctx context.Context, req dto.UpdatePurchaseOrderDeliveryStatusRequest) error
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
	err := r.db.Preload("Items").Preload("Items.Product").First(&purchaseOrder, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &purchaseOrder, nil
}

func (r *purchaseOrderRepository) Update(ctx context.Context, purchaseOrder *models.PurchaseOrder) error {
	return r.db.WithContext(ctx).Save(purchaseOrder).Error
}

func (r *purchaseOrderRepository) Delete(id uint) error {
	return r.db.Delete(&models.PurchaseOrder{}, "id = ?", id).Error
}

func (r *purchaseOrderRepository) List(ctx context.Context, params models.ListParams) ([]models.PurchaseOrder, int64, error) {
	var purchaseOrders []models.PurchaseOrder
	var total int64

	// Build base query for both count and data
	baseQuery := r.db.WithContext(ctx).Model(&models.PurchaseOrder{})

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

	// Apply status filter
	if params.Status != "" {
		baseQuery = baseQuery.Where("status = ?", params.Status)
	}

	// Apply same search filter for data
	if params.Search != "" {
		baseQuery = baseQuery.Where("order_number ILIKE ? OR notes ILIKE ?", "%"+params.Search+"%", "%"+params.Search+"%")
	}

	// Check if user has permission to view prices
	if pkg.HasPermission(ctx, "prices", "view") {
		// Include all fields with preloads
		baseQuery = baseQuery.Preload("Items").Preload("Items.Product")
	} else {
		// Omit price fields when user doesn't have price view permission
		baseQuery = baseQuery.Preload("Items", func(db *gorm.DB) *gorm.DB {
			return db.Omit("unit_price")
		}).Preload("Items.Product")
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

func (r *purchaseOrderRepository) UpdateStatus(ctx context.Context, purchaseOrderID uint, status models.PurchaseOrderStatus) error {
	return r.db.WithContext(ctx).Model(&models.PurchaseOrder{}).Where("id = ?", purchaseOrderID).Update("status", status).Error
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
func (r *purchaseOrderRepository) ReceiveInventory(ctx context.Context, req dto.UpdatePurchaseOrderDeliveryStatusRequest) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var po *models.PurchaseOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", req.PurchaseOrderID).
			Where("status NOT IN ?", []any{models.PurchaseOrderStatusCancelled}).
			Find(&po).Error; err != nil {
			return pkg.NewAppError(pkg.ErrorCodeNotFound,
				fmt.Sprintf("purchase order ID %d not found", req.PurchaseOrderID), nil)
		}

		type POIData struct {
			*models.PurchaseOrderItem
			InventoryItemID *uint
			ProductUnit     string
		}

		var poiData []POIData
		err := tx.Table("purchase_order_items poi").
			Select(`
				poi.*,
				ii.id as inventory_item_id,
				p.unit as product_unit
			`).
			Joins(`LEFT JOIN inventory_items ii ON poi.product_id = ii.product_id
					AND poi.supplier_id = ii.supplier_id
					AND ii.status = ?
			`, models.InventoryStatusActive).
			Joins(`JOIN products p ON poi.product_id = p.id`).
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
					SupplierID:  *poItem.SupplierID,
					Unit:        poItem.ProductUnit,
					Quantity:    dtoItem.ReceivedQuantity,
					Status:      models.InventoryItemStatusActive,
				}
				newInventoryItems = append(newInventoryItems, transaction.InventoryItem)
			}

			transactions = append(transactions, transaction)
		}

		// convert poiData to slice of PurchaseOrderItem model and set to
		// PurchaseOrder field for updating status.
		poItems := make([]*models.PurchaseOrderItem, 0, len(poiData))
		for _, data := range poiData {
			poItems = append(poItems, data.PurchaseOrderItem)
		}
		po.Items = poItems
		err = po.UpdateStatus()
		if err != nil {
			return fmt.Errorf("failed to calculate purchase order status: %w", err)
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

		err = tx.Model(&models.PurchaseOrder{}).
			Where("id = ?", po.ID).Update("status", po.Status).Error
		if err != nil {
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
