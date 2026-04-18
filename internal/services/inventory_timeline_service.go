package services

import (
	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/internal/services/dto"
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type InventoryTimelineService interface {
	GetInventoryTimeline(ctx context.Context, req dto.InventoryTimelineRequest) (*dto.InventoryTimelineResponse, error)
}

type inventoryTimelineService struct {
	inventoryRepo    repository.InventoryRepository
	sellingPriceRepo repository.SellingPriceRepository
	db               *gorm.DB
}

func NewInventoryTimelineService(
	inventoryRepo repository.InventoryRepository,
	sellingPriceRepo repository.SellingPriceRepository,
	db *gorm.DB,
) InventoryTimelineService {
	return &inventoryTimelineService{
		inventoryRepo:    inventoryRepo,
		sellingPriceRepo: sellingPriceRepo,
		db:               db,
	}
}

func (s *inventoryTimelineService) GetInventoryTimeline(ctx context.Context, req dto.InventoryTimelineRequest) (*dto.InventoryTimelineResponse, error) {
	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		return nil, fmt.Errorf("invalid start_date format, expected YYYY-MM-DD: %w", err)
	}
	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		return nil, fmt.Errorf("invalid end_date format, expected YYYY-MM-DD: %w", err)
	}
	// End date is exclusive — add 1 day
	endDateExclusive := endDate.AddDate(0, 0, 1)

	// 1. Load inventory with items
	inventory, err := s.inventoryRepo.GetByID(ctx, req.InventoryID)
	if err != nil {
		return nil, fmt.Errorf("failed to get inventory: %w", err)
	}

	// Build product info maps from inventory items
	type productInfo struct {
		ID   uint
		Name string
		Unit string
	}
	productMap := make(map[uint]productInfo)        // productID -> info
	itemToProduct := make(map[uint]uint)             // inventoryItemID -> productID
	var inventoryItemIDs []uint
	var productIDs []uint

	for _, item := range inventory.Items {
		if item.Product == nil {
			continue
		}
		// Filter by requested product IDs if specified
		if len(req.ProductIDs) > 0 {
			found := false
			for _, pid := range req.ProductIDs {
				if item.Product.ID == pid {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		unitName := ""
		if item.Unit != nil {
			unitName = item.Unit.Name
		}
		productMap[item.Product.ID] = productInfo{
			ID:   item.Product.ID,
			Name: item.Product.Name,
			Unit: unitName,
		}
		itemToProduct[item.ID] = item.Product.ID
		inventoryItemIDs = append(inventoryItemIDs, item.ID)
		productIDs = append(productIDs, item.Product.ID)
	}

	if len(inventoryItemIDs) == 0 {
		return &dto.InventoryTimelineResponse{Products: []dto.ProductTimeline{}}, nil
	}

	// 2. Fetch historical transactions (before start_date) to compute beginning_stock
	historicalTxns, err := s.inventoryRepo.GetTransactionsByInventoryIDs(ctx, req.InventoryID, nil, &startDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get historical transactions: %w", err)
	}

	beginningStock := make(map[uint]decimal.Decimal) // productID -> stock
	for _, txn := range historicalTxns {
		pid, ok := itemToProduct[txn.InventoryItemID]
		if !ok {
			continue
		}
		beginningStock[pid] = beginningStock[pid].Add(txn.TransactionType.StockDelta(txn.Quantity))
	}

	// 3. Query 1: Fetch all transactions within [start_date, end_date)
	periodTxns, err := s.inventoryRepo.GetTransactionsByInventoryIDs(ctx, req.InventoryID, &startDate, &endDateExclusive)
	if err != nil {
		return nil, fmt.Errorf("failed to get period transactions: %w", err)
	}

	// 4. Query 2: Get selling prices for sell transactions
	var sellTxnIDs []uint
	for _, txn := range periodTxns {
		if txn.TransactionType == models.InventoryTransactionTypeSell {
			sellTxnIDs = append(sellTxnIDs, txn.ID)
		}
	}

	sellPriceMap, err := s.sellingPriceRepo.GetSellingPricesForSellTransactions(ctx, sellTxnIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get selling prices for sell transactions: %w", err)
	}

	// 5. Fetch selling price changes in date range
	priceChanges, err := s.sellingPriceRepo.GetByProductIDsInDateRange(ctx, productIDs, startDate, endDateExclusive)
	if err != nil {
		return nil, fmt.Errorf("failed to get selling price changes: %w", err)
	}

	// Group price changes by product and compute previous_price
	priceChangesByProduct := make(map[uint][]dto.TimelineSellingPriceChange)
	// Batch-fetch the last price before the range for each product to compute previous_price for the first change
	var inventoryIDPtr *uint
	if inventory.ID > 0 {
		inventoryIDPtr = &inventory.ID
	}
	lastPriceBeforeRange, _ := s.sellingPriceRepo.GetLatestForProducts(ctx, productIDs, inventoryIDPtr, startDate)

	// Build price change DTOs with previous_price
	for _, sp := range priceChanges {
		var prevPrice *float64

		// Check if there are already changes for this product in the list
		existing := priceChangesByProduct[sp.ProductID]
		if len(existing) > 0 {
			// Previous is the last entry in the list
			prev := existing[len(existing)-1].Price
			prevPrice = &prev
		} else if lastSP, ok := lastPriceBeforeRange[sp.ProductID]; ok {
			// Previous is the last known price before the range
			prev, _ := lastSP.Price.Float64()
			prevPrice = &prev
		}

		price, _ := sp.Price.Float64()
		priceChangesByProduct[sp.ProductID] = append(priceChangesByProduct[sp.ProductID], dto.TimelineSellingPriceChange{
			ID:            sp.ID,
			Price:         price,
			PreviousPrice: prevPrice,
			EffectiveFrom: sp.EffectiveFrom.Format("2006-01-02"),
			Notes:         sp.Notes,
		})
	}

	// 6. Fetch purchase orders for these products in this inventory
	type poRow struct {
		POID             uint             `gorm:"column:po_id"`
		PONumber         string           `gorm:"column:po_number"`
		ProductID        uint             `gorm:"column:product_id"`
		QuantityOrdered  decimal.Decimal  `gorm:"column:quantity_ordered"`
		QuantityReceived decimal.Decimal  `gorm:"column:quantity_received"`
		Status           string           `gorm:"column:status"`
		POStatus         string           `gorm:"column:po_status"`
		SellingPrice     *decimal.Decimal `gorm:"column:selling_price"`
		HasSellingPrice  bool             `gorm:"column:has_selling_price"`
	}
	var poRows []poRow
	if err := s.db.WithContext(ctx).
		Raw(`SELECT po.id as po_id, po.order_number as po_number, poi.product_id,
				poi.quantity as quantity_ordered, poi.received_quantity as quantity_received,
				poi.status as status, po.status as po_status,
				COALESCE(pisp.selling_price, sp.price) as selling_price,
				(pisp.id IS NOT NULL) as has_selling_price
			FROM purchase_order_items poi
			JOIN purchase_orders po ON po.id = poi.purchase_order_id
			LEFT JOIN purchase_order_item_selling_prices pisp ON pisp.purchase_order_item_id = poi.id
			LEFT JOIN selling_prices sp ON sp.id = pisp.selling_price_id
			WHERE po.inventory_id = ?
			AND poi.product_id IN ?
			AND po.deleted_at IS NULL
			AND poi.deleted_at IS NULL
			ORDER BY po.created_at DESC`, req.InventoryID, productIDs).
		Scan(&poRows).Error; err != nil {
		return nil, fmt.Errorf("failed to get purchase orders: %w", err)
	}

	posByProduct := make(map[uint][]dto.TimelinePurchaseOrder)
	for _, row := range poRows {
		qtyOrdered, _ := row.QuantityOrdered.Float64()
		qtyReceived, _ := row.QuantityReceived.Float64()
		tlPO := dto.TimelinePurchaseOrder{
			POID:             row.POID,
			PONumber:         row.PONumber,
			DeliveryStatus:   row.Status,
			PaymentStatus:    row.POStatus,
			QuantityOrdered:  qtyOrdered,
			QuantityReceived: qtyReceived,
			HasSellingPrice:  row.HasSellingPrice,
		}
		if row.SellingPrice != nil {
			sp, _ := row.SellingPrice.Float64()
			tlPO.SellingPrice = &sp
		}
		posByProduct[row.ProductID] = append(posByProduct[row.ProductID], tlPO)
	}

	// 7. Group transactions by product and compute metrics
	type productData struct {
		transactions    []dto.TimelineTransaction
		metrics         dto.TimelineProductMetrics
		periodStockDelta float64 // net stock change during period, computed via StockDelta
	}
	dataByProduct := make(map[uint]*productData)
	for _, pid := range productIDs {
		dataByProduct[pid] = &productData{}
	}

	// 7a. Resolve PO IDs for transactions
	// Purchase txns have PurchaseOrderItemID → look up purchase_order_id
	// Sell txns have CounterTransactionID → purchase txn → same lookup
	var poItemIDs []uint
	poItemIDToTxnIDs := make(map[uint][]uint)        // poi_id → txn IDs that reference it
	counterTxnToSellTxn := make(map[uint][]uint)      // counter_txn_id → sell txn IDs

	for _, txn := range periodTxns {
		if txn.PurchaseOrderItemID != nil && *txn.PurchaseOrderItemID > 0 {
			poItemIDs = append(poItemIDs, *txn.PurchaseOrderItemID)
			poItemIDToTxnIDs[*txn.PurchaseOrderItemID] = append(poItemIDToTxnIDs[*txn.PurchaseOrderItemID], txn.ID)
		}
		if txn.CounterTransactionID != nil && *txn.CounterTransactionID > 0 {
			counterTxnToSellTxn[*txn.CounterTransactionID] = append(counterTxnToSellTxn[*txn.CounterTransactionID], txn.ID)
		}
	}

	// Query PO IDs from purchase_order_items
	txnToPOID := make(map[uint]uint) // txn_id → purchase_order_id
	if len(poItemIDs) > 0 {
		type poiRow struct {
			ID              uint `gorm:"column:id"`
			PurchaseOrderID uint `gorm:"column:purchase_order_id"`
		}
		var poiRows []poiRow
		if err := s.db.WithContext(ctx).
			Raw(`SELECT id, purchase_order_id FROM purchase_order_items WHERE id IN ?`, poItemIDs).
			Scan(&poiRows).Error; err == nil {
			for _, row := range poiRows {
				// Map purchase txns that reference this POI → PO ID
				for _, txnID := range poItemIDToTxnIDs[row.ID] {
					txnToPOID[txnID] = row.PurchaseOrderID
				}
			}
		}
	}

	// Resolve sell txns via their counter (purchase) txn
	// The counter txn may be outside the period, so query the DB directly
	var counterTxnIDs []uint
	sellTxnByCounterID := make(map[uint][]uint) // counter_txn_id → sell txn IDs
	for _, txn := range periodTxns {
		if txn.TransactionType == models.InventoryTransactionTypeSell && txn.CounterTransactionID != nil {
			// Check if already resolved from in-memory period txns
			if _, ok := txnToPOID[*txn.CounterTransactionID]; ok {
				txnToPOID[txn.ID] = txnToPOID[*txn.CounterTransactionID]
			} else {
				counterTxnIDs = append(counterTxnIDs, *txn.CounterTransactionID)
				sellTxnByCounterID[*txn.CounterTransactionID] = append(sellTxnByCounterID[*txn.CounterTransactionID], txn.ID)
			}
		}
	}

	// Look up PO IDs for counter txns that were outside the period
	if len(counterTxnIDs) > 0 {
		type counterRow struct {
			ID              uint  `gorm:"column:id"`
			PurchaseOrderID *uint `gorm:"column:purchase_order_id"`
		}
		var counterRows []counterRow
		if err := s.db.WithContext(ctx).
			Raw(`SELECT it.id, poi.purchase_order_id
				FROM inventory_transactions it
				JOIN purchase_order_items poi ON poi.id = it.purchase_order_item_id
				WHERE it.id IN ?`, counterTxnIDs).
			Scan(&counterRows).Error; err == nil {
			for _, row := range counterRows {
				if row.PurchaseOrderID != nil {
					for _, sellTxnID := range sellTxnByCounterID[row.ID] {
						txnToPOID[sellTxnID] = *row.PurchaseOrderID
					}
				}
			}
		}
	}

	for _, txn := range periodTxns {
		pid, ok := itemToProduct[txn.InventoryItemID]
		if !ok {
			continue
		}

		qty, _ := txn.Quantity.Float64()

		// Skip 0-quantity transactions
		if qty == 0 {
			continue
		}

		pd := dataByProduct[pid]
		costPrice := txn.Price

		// Map backend transaction types to frontend-friendly types
		txnType := string(txn.TransactionType)
		switch txn.TransactionType {
		case models.InventoryTransactionTypeDisposal:
			txnType = "dispose"
		case models.InventoryTransactionTypeTransferOut, models.InventoryTransactionTypeTransferIn:
			txnType = "transfer"
		}

		tlTxn := dto.TimelineTransaction{
			TransactionID:   txn.ID,
			TransactionType: txnType,
			Date:            txn.CreatedAt.Format("2006-01-02"),
			Quantity:        qty,
			CostPrice:       &costPrice,
		}

		// Set PO ID if resolved
		if poID, ok := txnToPOID[txn.ID]; ok {
			tlTxn.POID = &poID
		}

		// Add selling price for sell transactions
		if txn.TransactionType == models.InventoryTransactionTypeSell {
			if sp, ok := sellPriceMap[txn.ID]; ok {
				spFloat, _ := sp.Float64()
				tlTxn.SellingPrice = &spFloat
				pd.metrics.TotalRevenue += qty * spFloat
			}
		}

		pd.transactions = append(pd.transactions, tlTxn)

		// Accumulate stock delta from canonical StockDelta method
		delta, _ := txn.TransactionType.StockDelta(txn.Quantity).Float64()
		pd.periodStockDelta += delta

		// Per-type metrics for display
		switch txn.TransactionType {
		case models.InventoryTransactionTypePurchase:
			pd.metrics.TotalPurchased += qty
		case models.InventoryTransactionTypeSell:
			pd.metrics.TotalSold += qty
		case models.InventoryTransactionTypeDisposal:
			pd.metrics.TotalDisposed += qty
		case models.InventoryTransactionTypeTransferIn:
			pd.metrics.TotalTransferIn += qty
		case models.InventoryTransactionTypeTransferOut:
			pd.metrics.TotalTransferred += qty
		}
	}

	// 7. Assemble response
	var products []dto.ProductTimeline
	for _, pid := range productIDs {
		info := productMap[pid]
		pd := dataByProduct[pid]

		bs, _ := beginningStock[pid].Float64()
		es := bs + pd.periodStockDelta

		txns := pd.transactions
		if txns == nil {
			txns = []dto.TimelineTransaction{}
		}

		changes := priceChangesByProduct[pid]
		if changes == nil {
			changes = []dto.TimelineSellingPriceChange{}
		}

		products = append(products, dto.ProductTimeline{
			ProductID:           info.ID,
			ProductName:         info.Name,
			ProductUnit:         info.Unit,
			BeginningStock:      bs,
			EndingStock:         es,
			PurchaseOrders:      func() []dto.TimelinePurchaseOrder {
				if pos, ok := posByProduct[pid]; ok {
					return pos
				}
				return []dto.TimelinePurchaseOrder{}
			}(),
			Transactions:        txns,
			SellingPriceChanges: changes,
			Metrics:             pd.metrics,
		})
	}

	return &dto.InventoryTimelineResponse{Products: products}, nil
}
