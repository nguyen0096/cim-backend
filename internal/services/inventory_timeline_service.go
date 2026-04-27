package services

import (
	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/internal/services/dto"
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

type InventoryTimelineService interface {
	GetInventoryTimeline(ctx context.Context, req dto.InventoryTimelineRequest) (*dto.InventoryTimelineResponse, error)
}

type inventoryTimelineService struct {
	inventoryRepo    repository.InventoryRepository
	sellingPriceRepo repository.SellingPriceRepository
}

func NewInventoryTimelineService(
	inventoryRepo repository.InventoryRepository,
	sellingPriceRepo repository.SellingPriceRepository,
) InventoryTimelineService {
	return &inventoryTimelineService{
		inventoryRepo:    inventoryRepo,
		sellingPriceRepo: sellingPriceRepo,
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
	endDateExclusive := endDate.AddDate(0, 0, 1)

	// 1. Load inventory with items and build product map.
	inventory, err := s.inventoryRepo.GetByID(ctx, req.InventoryID)
	if err != nil {
		return nil, fmt.Errorf("failed to get inventory: %w", err)
	}

	type productInfo struct {
		ID   uint
		Name string
		Unit string
	}
	productMap := make(map[uint]productInfo)
	itemToProduct := make(map[uint]uint)
	var productIDs []uint
	productIDSeen := make(map[uint]bool)

	for _, item := range inventory.Items {
		if item.Product == nil {
			continue
		}
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
		if !productIDSeen[item.Product.ID] {
			productIDs = append(productIDs, item.Product.ID)
			productIDSeen[item.Product.ID] = true
		}
	}

	if len(productIDs) == 0 {
		return &dto.InventoryTimelineResponse{Products: []dto.ProductTimeline{}}, nil
	}

	// 2. Historical txns → beginning stock.
	historicalTxns, err := s.inventoryRepo.GetTransactionsByInventoryIDs(ctx, req.InventoryID, nil, &startDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get historical transactions: %w", err)
	}
	beginningStock := make(map[uint]decimal.Decimal)
	for _, txn := range historicalTxns {
		pid, ok := itemToProduct[txn.InventoryItemID]
		if !ok {
			continue
		}
		beginningStock[pid] = beginningStock[pid].Add(txn.TransactionType.StockDelta(txn.Quantity))
	}

	// 3. Period txns + counter POI for sells/disposals/transfers (single query, self-join).
	periodTxns, err := s.inventoryRepo.GetTransactionsByInventoryIDsWithCounter(ctx, req.InventoryID, &startDate, &endDateExclusive)
	if err != nil {
		return nil, fmt.Errorf("failed to get period transactions: %w", err)
	}

	// Collect POI IDs referenced by any period txn — purchases via own PurchaseOrderItemID,
	// sells/disposals/transfers via the counter purchase txn's POI.
	txnToPOItemID := make(map[uint]uint)
	poItemIDSet := make(map[uint]bool)
	var poItemIDs []uint
	for _, txn := range periodTxns {
		var poiID uint
		switch {
		case txn.PurchaseOrderItemID != nil && *txn.PurchaseOrderItemID > 0:
			poiID = *txn.PurchaseOrderItemID
		case txn.CounterPOIID != nil && *txn.CounterPOIID > 0:
			poiID = *txn.CounterPOIID
		default:
			continue
		}
		txnToPOItemID[txn.ID] = poiID
		if !poItemIDSet[poiID] {
			poItemIDs = append(poItemIDs, poiID)
			poItemIDSet[poiID] = true
		}
	}

	// 4. PO/POI metadata + effective price for all relevant POIs (one query, scoped to inventory).
	poInfoByItemID, err := s.sellingPriceRepo.GetPOItemsWithPriceByIDs(ctx, poItemIDs, req.InventoryID)
	if err != nil {
		return nil, fmt.Errorf("failed to get PO item info: %w", err)
	}

	// 5. Walk: assemble transactions[] and purchase_orders[] per product.
	type productData struct {
		transactions     []dto.TimelineTransaction
		metrics          dto.TimelineProductMetrics
		periodStockDelta float64
		poItemSeen       map[uint]bool // dedupe POIs within a product's purchase_orders[]
		purchaseOrders   []dto.TimelinePurchaseOrder
	}
	dataByProduct := make(map[uint]*productData, len(productIDs))
	for _, pid := range productIDs {
		dataByProduct[pid] = &productData{poItemSeen: make(map[uint]bool)}
	}

	for _, txn := range periodTxns {
		pid, ok := itemToProduct[txn.InventoryItemID]
		if !ok {
			continue
		}
		qty, _ := txn.Quantity.Float64()
		if qty == 0 {
			continue
		}

		pd := dataByProduct[pid]
		costPrice := txn.Price

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

		// Resolve POI → PO id and selling price from the consolidated map.
		if poiID, ok := txnToPOItemID[txn.ID]; ok {
			if info, exists := poInfoByItemID[poiID]; exists {
				poID := info.POID
				tlTxn.POID = &poID

				// Sell txns get the effective selling price for revenue calculation.
				if txn.TransactionType == models.InventoryTransactionTypeSell {
					if info.EffectivePrice != nil {
						sp, _ := info.EffectivePrice.Float64()
						tlTxn.SellingPrice = &sp
						pd.metrics.TotalRevenue += qty * sp
					}
				}

				// First time we see this POI for this product — add it to purchase_orders[].
				if !pd.poItemSeen[poiID] {
					pd.poItemSeen[poiID] = true
					qtyOrdered, _ := info.QuantityOrdered.Float64()
					qtyReceived, _ := info.QuantityReceived.Float64()
					tlPO := dto.TimelinePurchaseOrder{
						POID:             info.POID,
						PONumber:         info.PONumber,
						DeliveryStatus:   info.POItemStatus,
						PaymentStatus:    info.POStatus,
						QuantityOrdered:  qtyOrdered,
						QuantityReceived: qtyReceived,
					}
					if info.EffectivePrice != nil {
						sp, _ := info.EffectivePrice.Float64()
						tlPO.SellingPrice = &sp
					}
					pd.purchaseOrders = append(pd.purchaseOrders, tlPO)
				}
			}
		}

		pd.transactions = append(pd.transactions, tlTxn)

		delta, _ := txn.TransactionType.StockDelta(txn.Quantity).Float64()
		pd.periodStockDelta += delta

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

	// 6. Assemble response.
	products := make([]dto.ProductTimeline, 0, len(productIDs))
	for _, pid := range productIDs {
		info := productMap[pid]
		pd := dataByProduct[pid]

		bs, _ := beginningStock[pid].Float64()
		es := bs + pd.periodStockDelta

		txns := pd.transactions
		if txns == nil {
			txns = []dto.TimelineTransaction{}
		}
		pos := pd.purchaseOrders
		if pos == nil {
			pos = []dto.TimelinePurchaseOrder{}
		}

		products = append(products, dto.ProductTimeline{
			ProductID:      info.ID,
			ProductName:    info.Name,
			ProductUnit:    info.Unit,
			BeginningStock: bs,
			EndingStock:    es,
			PurchaseOrders: pos,
			Transactions:   txns,
			Metrics:        pd.metrics,
		})
	}

	return &dto.InventoryTimelineResponse{Products: products}, nil
}
