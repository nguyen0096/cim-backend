package services

import (
	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/internal/services/dto"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

const (
	// defaultTimelinePageLimit is used when the request omits/zeroes limit.
	defaultTimelinePageLimit = 50
	// maxTimelinePageLimit caps the page size to bound response size.
	maxTimelinePageLimit = 200
)

type InventoryTimelineService interface {
	GetInventoryTimeline(ctx context.Context, req dto.InventoryTimelineRequest) (*dto.InventoryTimelineResponse, error)
}

type inventoryTimelineService struct {
	inventoryRepo     repository.InventoryRepository
	inventoryItemRepo repository.InventoryItemRepository
	sellingPriceRepo  repository.SellingPriceRepository
}

func NewInventoryTimelineService(
	inventoryRepo repository.InventoryRepository,
	inventoryItemRepo repository.InventoryItemRepository,
	sellingPriceRepo repository.SellingPriceRepository,
) InventoryTimelineService {
	return &inventoryTimelineService{
		inventoryRepo:     inventoryRepo,
		inventoryItemRepo: inventoryItemRepo,
		sellingPriceRepo:  sellingPriceRepo,
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

	page, limit := normalizePageLimit(req.Page, req.Limit)

	// Filters are applied once, in the repository, for both the count and the
	// page query (shared builder) — search by name tokens, sort by product name.
	filters := repository.InventoryItemFilters{
		SearchTokens: searchTokens(req.Search),
		ProductIDs:   req.ProductIDs,
		Sort:         string(repository.InventoryItemSortFieldProductName),
		Order:        "ASC",
	}

	// 1. Total matching products → drives pagination (and lets us short-circuit
	// out-of-range pages without a data query, which also dodges offset overflow).
	total, err := s.inventoryItemRepo.CountByInventoryIDWithFilters(ctx, req.InventoryID, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to count inventory items: %w", err)
	}

	totalPages := 0
	if limit > 0 {
		totalPages = int((total + int64(limit) - 1) / int64(limit))
	}

	pagination := dto.TimelinePagination{Page: page, Limit: limit, Total: total, TotalPages: totalPages}

	// No rows, or a page past the end → empty page. `int64(page) > int64(totalPages)`
	// is a plain comparison (no multiply), so a huge page can't overflow here.
	if total == 0 || int64(page) > int64(totalPages) {
		return &dto.InventoryTimelineResponse{Products: []dto.ProductTimeline{}, Pagination: pagination}, nil
	}

	// 2. Fetch just this page of items (sorted by name in the DB).
	offset := (page - 1) * limit // safe: page <= totalPages ⇒ offset < total
	items, err := s.inventoryItemRepo.GetByInventoryIDWithFilters(ctx, req.InventoryID, filters, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get inventory items: %w", err)
	}

	type productInfo struct {
		ID   uint
		Name string
		Unit string
	}
	productMap := make(map[uint]productInfo)
	itemToProduct := make(map[uint]uint)
	var productIDs []uint // preserves the DB sort order (product name asc)
	productIDSeen := make(map[uint]bool)
	var itemIDs []uint

	for _, item := range items {
		if item.Product == nil {
			continue
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
		itemIDs = append(itemIDs, item.ID)
		if !productIDSeen[item.Product.ID] {
			productIDs = append(productIDs, item.Product.ID)
			productIDSeen[item.Product.ID] = true
		}
	}

	if len(productIDs) == 0 {
		return &dto.InventoryTimelineResponse{Products: []dto.ProductTimeline{}, Pagination: pagination}, nil
	}

	// 3. Historical txns (scoped to this page's items) → beginning stock.
	historicalTxns, err := s.inventoryRepo.GetTransactionsByInventoryIDs(ctx, req.InventoryID, nil, &startDate, itemIDs...)
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

	// 4. Period txns + counter POI (scoped to this page's items).
	periodTxns, err := s.inventoryRepo.GetTransactionsByInventoryIDsWithCounter(ctx, req.InventoryID, &startDate, &endDateExclusive, itemIDs...)
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

	// 5. PO/POI metadata + effective price for all relevant POIs (one query, scoped to inventory).
	poInfoByItemID, err := s.sellingPriceRepo.GetPOItemsWithPriceByIDs(ctx, poItemIDs, req.InventoryID)
	if err != nil {
		return nil, fmt.Errorf("failed to get PO item info: %w", err)
	}

	// 6. Walk: assemble transactions[] and purchase_orders[] per product.
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

		// reconcile_stock_up surfaces as its own zero-cost "adjustment" row (no PO)
		// and metric, and contributes to the stock aggregate so ending stock foots.
		if txn.TransactionType == models.InventoryTransactionTypeReconcileStockUp {
			zero := 0.0
			pd.transactions = append(pd.transactions, dto.TimelineTransaction{
				TransactionID:   txn.ID,
				TransactionType: "adjustment",
				Date:            txn.CreatedAt.Format("2006-01-02"),
				Quantity:        qty,
				CostPrice:       &zero,
			})
			d, _ := txn.TransactionType.StockDelta(txn.Quantity).Float64()
			pd.periodStockDelta += d
			pd.metrics.TotalAdjustment += qty
			continue
		}

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
						POItemID:         info.POItemID,
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

	// 7. Assemble response in DB sort order (product name asc).
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

	return &dto.InventoryTimelineResponse{Products: products, Pagination: pagination}, nil
}

// searchTokens lowercases and splits a search string into non-empty tokens.
func searchTokens(search string) []string {
	return strings.Fields(strings.ToLower(search))
}

// normalizePageLimit applies defaults and caps to the requested page/limit.
func normalizePageLimit(page, limit int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = defaultTimelinePageLimit
	}
	if limit > maxTimelinePageLimit {
		limit = maxTimelinePageLimit
	}
	return page, limit
}
