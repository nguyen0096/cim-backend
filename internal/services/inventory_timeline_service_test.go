package services

import (
	"context"
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"cim-backend/internal/mocks/repositorymocks"
	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
)

// ---------- helpers ----------

const (
	tlInventoryID = uint(10)
	tlProductID   = uint(1)
	tlItemID      = uint(100)
	tlUnitID      = uint(5)
	tlPOItemID    = uint(200)
	tlPOID        = uint(50)
)

// tlWidgetItems is the single-product page used by the assembly tests:
// item 100 → product 1 (Widget), unit kg.
func tlWidgetItems() []models.InventoryItem {
	return []models.InventoryItem{
		{
			Base:        models.Base{ID: tlItemID},
			InventoryID: tlInventoryID,
			ProductID:   tlProductID,
			Product:     &models.Product{Base: models.Base{ID: tlProductID}, Name: "Widget"},
			UnitID:      tlUnitID,
			Unit:        &models.Unit{Base: models.Base{ID: tlUnitID}, Name: "kg"},
		},
	}
}

// tlMultiItems builds a page with one item per product name, each with a
// distinct product/item id (no transactions).
func tlMultiItems(names ...string) []models.InventoryItem {
	items := make([]models.InventoryItem, 0, len(names))
	for i, name := range names {
		items = append(items, models.InventoryItem{
			Base:        models.Base{ID: uint(2000 + i)},
			InventoryID: tlInventoryID,
			ProductID:   uint(1000 + i),
			Product:     &models.Product{Base: models.Base{ID: uint(1000 + i)}, Name: name},
			UnitID:      tlUnitID,
			Unit:        &models.Unit{Base: models.Base{ID: tlUnitID}, Name: "kg"},
		})
	}
	return items
}

// tlSetupItems mocks the item-page repo: total = len(items), page returns items.
func tlSetupItems(ctx context.Context, itemRepo *repositorymocks.InventoryItemRepository, items []models.InventoryItem) {
	itemRepo.On("CountByInventoryIDWithFilters", ctx, tlInventoryID, mock.Anything).Return(int64(len(items)), nil)
	itemRepo.On("GetByInventoryIDWithFilters", ctx, tlInventoryID, mock.Anything, mock.Anything, mock.Anything).Return(items, nil)
}

// tlOnTxns mocks the two scoped transaction queries. The variadic itemIDs count
// equals the number of page items, so we append one matcher per item.
func tlOnTxns(ctx context.Context, invRepo *repositorymocks.InventoryRepository, nItems int, historical []*models.InventoryTransaction, period []*repository.InventoryTransactionWithCounter) {
	hist := []interface{}{ctx, tlInventoryID, (*time.Time)(nil), mock.Anything}
	per := []interface{}{ctx, tlInventoryID, mock.Anything, mock.Anything}
	for i := 0; i < nItems; i++ {
		hist = append(hist, mock.Anything)
		per = append(per, mock.Anything)
	}
	invRepo.On("GetTransactionsByInventoryIDs", hist...).Return(historical, nil)
	invRepo.On("GetTransactionsByInventoryIDsWithCounter", per...).Return(period, nil)
}

func tlProductNames(resp *dto.InventoryTimelineResponse) []string {
	names := make([]string, len(resp.Products))
	for i, p := range resp.Products {
		names[i] = p.ProductName
	}
	return names
}

func tlPurchaseTxn(id uint, qty float64, poItemID uint) *repository.InventoryTransactionWithCounter {
	poiID := poItemID
	return &repository.InventoryTransactionWithCounter{
		InventoryTransaction: &models.InventoryTransaction{
			Base:                models.Base{ID: id, CreatedAt: time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)},
			InventoryItemID:     tlItemID,
			TransactionType:     models.InventoryTransactionTypePurchase,
			Quantity:            decimal.NewFromFloat(qty),
			Price:               5.0,
			PurchaseOrderItemID: &poiID,
		},
	}
}

func tlConsumeTxn(id uint, qty float64, txType models.InventoryTransactionType, counterPOI *uint) *repository.InventoryTransactionWithCounter {
	return &repository.InventoryTransactionWithCounter{
		InventoryTransaction: &models.InventoryTransaction{
			Base:            models.Base{ID: id, CreatedAt: time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)},
			InventoryItemID: tlItemID,
			TransactionType: txType,
			Quantity:        decimal.NewFromFloat(qty),
			Price:           5.0,
		},
		CounterPOIID: counterPOI,
	}
}

func tlPOInfo(price *decimal.Decimal) *repository.POItemSellingPriceInfo {
	return &repository.POItemSellingPriceInfo{
		POItemID:         tlPOItemID,
		POID:             tlPOID,
		PONumber:         "PO-2026-001",
		POStatus:         string(models.PurchaseOrderStatusFullyDelivered),
		POItemStatus:     string(models.PurchaseOrderItemStatusDelivered),
		ProductID:        tlProductID,
		QuantityOrdered:  decimal.NewFromInt(10),
		QuantityReceived: decimal.NewFromInt(10),
		EffectivePrice:   price,
	}
}

func tlReq() dto.InventoryTimelineRequest {
	return dto.InventoryTimelineRequest{
		InventoryID: tlInventoryID,
		StartDate:   "2026-04-01",
		EndDate:     "2026-04-30",
	}
}

// ---------- selling-price resolution / assembly tests ----------

func Test_GetInventoryTimeline_SellSellingPrice_Override(t *testing.T) {
	ctx := context.Background()
	invRepo := repositorymocks.NewInventoryRepository(t)
	itemRepo := repositorymocks.NewInventoryItemRepository(t)
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	svc := NewInventoryTimelineService(invRepo, itemRepo, spRepo)

	counterPOI := uint(tlPOItemID)
	override := decimal.NewFromInt(99)

	tlSetupItems(ctx, itemRepo, tlWidgetItems())
	tlOnTxns(ctx, invRepo, 1, []*models.InventoryTransaction{}, []*repository.InventoryTransactionWithCounter{
		tlConsumeTxn(2, 3, models.InventoryTransactionTypeSell, &counterPOI),
	})
	spRepo.On("GetPOItemsWithPriceByIDs", ctx, []uint{tlPOItemID}, tlInventoryID).Return(map[uint]*repository.POItemSellingPriceInfo{
		tlPOItemID: tlPOInfo(&override),
	}, nil)

	resp, err := svc.GetInventoryTimeline(ctx, tlReq())
	require.NoError(t, err)
	require.Len(t, resp.Products, 1)

	txns := resp.Products[0].Transactions
	require.Len(t, txns, 1)
	require.NotNil(t, txns[0].SellingPrice)
	assert.Equal(t, 99.0, *txns[0].SellingPrice)
	assert.Equal(t, 297.0, resp.Products[0].Metrics.TotalRevenue) // 3 * 99
}

func Test_GetInventoryTimeline_SellSellingPrice_Default(t *testing.T) {
	ctx := context.Background()
	invRepo := repositorymocks.NewInventoryRepository(t)
	itemRepo := repositorymocks.NewInventoryItemRepository(t)
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	svc := NewInventoryTimelineService(invRepo, itemRepo, spRepo)

	counterPOI := uint(tlPOItemID)
	def := decimal.NewFromInt(50)

	tlSetupItems(ctx, itemRepo, tlWidgetItems())
	tlOnTxns(ctx, invRepo, 1, []*models.InventoryTransaction{}, []*repository.InventoryTransactionWithCounter{
		tlConsumeTxn(2, 2, models.InventoryTransactionTypeSell, &counterPOI),
	})
	spRepo.On("GetPOItemsWithPriceByIDs", ctx, []uint{tlPOItemID}, tlInventoryID).Return(map[uint]*repository.POItemSellingPriceInfo{
		tlPOItemID: tlPOInfo(&def),
	}, nil)

	resp, err := svc.GetInventoryTimeline(ctx, tlReq())
	require.NoError(t, err)
	require.Len(t, resp.Products[0].Transactions, 1)
	require.NotNil(t, resp.Products[0].Transactions[0].SellingPrice)
	assert.Equal(t, 50.0, *resp.Products[0].Transactions[0].SellingPrice)
}

func Test_GetInventoryTimeline_SellSellingPrice_BothNull(t *testing.T) {
	ctx := context.Background()
	invRepo := repositorymocks.NewInventoryRepository(t)
	itemRepo := repositorymocks.NewInventoryItemRepository(t)
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	svc := NewInventoryTimelineService(invRepo, itemRepo, spRepo)

	counterPOI := uint(tlPOItemID)

	tlSetupItems(ctx, itemRepo, tlWidgetItems())
	tlOnTxns(ctx, invRepo, 1, []*models.InventoryTransaction{}, []*repository.InventoryTransactionWithCounter{
		tlConsumeTxn(2, 1, models.InventoryTransactionTypeSell, &counterPOI),
	})
	// pisp row exists but resolves to nil (both selling_price and sp.price are NULL)
	spRepo.On("GetPOItemsWithPriceByIDs", ctx, []uint{tlPOItemID}, tlInventoryID).Return(map[uint]*repository.POItemSellingPriceInfo{
		tlPOItemID: tlPOInfo(nil),
	}, nil)

	resp, err := svc.GetInventoryTimeline(ctx, tlReq())
	require.NoError(t, err)
	require.Len(t, resp.Products[0].Transactions, 1)
	assert.Nil(t, resp.Products[0].Transactions[0].SellingPrice)
	assert.Equal(t, 0.0, resp.Products[0].Metrics.TotalRevenue)
	// PO is still listed (txn exists) but its selling_price is nil.
	require.Len(t, resp.Products[0].PurchaseOrders, 1)
	assert.Nil(t, resp.Products[0].PurchaseOrders[0].SellingPrice)
}

func Test_GetInventoryTimeline_SellSellingPrice_NoPISP(t *testing.T) {
	ctx := context.Background()
	invRepo := repositorymocks.NewInventoryRepository(t)
	itemRepo := repositorymocks.NewInventoryItemRepository(t)
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	svc := NewInventoryTimelineService(invRepo, itemRepo, spRepo)

	counterPOI := uint(tlPOItemID)

	tlSetupItems(ctx, itemRepo, tlWidgetItems())
	tlOnTxns(ctx, invRepo, 1, []*models.InventoryTransaction{}, []*repository.InventoryTransactionWithCounter{
		tlConsumeTxn(2, 1, models.InventoryTransactionTypeSell, &counterPOI),
	})
	// POI was not in the map at all (e.g. no pisp row was ever created for it).
	spRepo.On("GetPOItemsWithPriceByIDs", ctx, []uint{tlPOItemID}, tlInventoryID).Return(map[uint]*repository.POItemSellingPriceInfo{}, nil)

	resp, err := svc.GetInventoryTimeline(ctx, tlReq())
	require.NoError(t, err)
	require.Len(t, resp.Products[0].Transactions, 1)
	assert.Nil(t, resp.Products[0].Transactions[0].SellingPrice)
	assert.Nil(t, resp.Products[0].Transactions[0].POID)
	// PO not in the list either — POI metadata wasn't returned by the repo.
	assert.Empty(t, resp.Products[0].PurchaseOrders)
}

func Test_GetInventoryTimeline_SellWithoutCounterPOI_HasNoPrice(t *testing.T) {
	// Sell whose counter purchase txn has purchase_order_item_id = NULL (manual stock-in).
	ctx := context.Background()
	invRepo := repositorymocks.NewInventoryRepository(t)
	itemRepo := repositorymocks.NewInventoryItemRepository(t)
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	svc := NewInventoryTimelineService(invRepo, itemRepo, spRepo)

	tlSetupItems(ctx, itemRepo, tlWidgetItems())
	tlOnTxns(ctx, invRepo, 1, []*models.InventoryTransaction{}, []*repository.InventoryTransactionWithCounter{
		// No CounterPOIID set — counter purchase has no PurchaseOrderItemID.
		tlConsumeTxn(2, 1, models.InventoryTransactionTypeSell, nil),
	})
	// No POI lookup happens because no POI IDs are collected.
	spRepo.On("GetPOItemsWithPriceByIDs", ctx, []uint(nil), tlInventoryID).Return(map[uint]*repository.POItemSellingPriceInfo{}, nil)

	resp, err := svc.GetInventoryTimeline(ctx, tlReq())
	require.NoError(t, err)
	require.Len(t, resp.Products[0].Transactions, 1)
	assert.Nil(t, resp.Products[0].Transactions[0].POID)
	assert.Nil(t, resp.Products[0].Transactions[0].SellingPrice)
}

// ---------- bug-fix tests ----------

func Test_GetInventoryTimeline_DisposalAndTransferGetPOID_Bug1A(t *testing.T) {
	ctx := context.Background()
	invRepo := repositorymocks.NewInventoryRepository(t)
	itemRepo := repositorymocks.NewInventoryItemRepository(t)
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	svc := NewInventoryTimelineService(invRepo, itemRepo, spRepo)

	counterPOI := uint(tlPOItemID)
	price := decimal.NewFromInt(20)

	tlSetupItems(ctx, itemRepo, tlWidgetItems())
	tlOnTxns(ctx, invRepo, 1, []*models.InventoryTransaction{}, []*repository.InventoryTransactionWithCounter{
		tlConsumeTxn(3, 1, models.InventoryTransactionTypeDisposal, &counterPOI),
		tlConsumeTxn(4, 1, models.InventoryTransactionTypeTransferOut, &counterPOI),
	})
	spRepo.On("GetPOItemsWithPriceByIDs", ctx, []uint{tlPOItemID}, tlInventoryID).Return(map[uint]*repository.POItemSellingPriceInfo{
		tlPOItemID: tlPOInfo(&price),
	}, nil)

	resp, err := svc.GetInventoryTimeline(ctx, tlReq())
	require.NoError(t, err)
	require.Len(t, resp.Products[0].Transactions, 2)
	for _, txn := range resp.Products[0].Transactions {
		require.NotNil(t, txn.POID, "disposal/transfer should have po_id resolved (Bug 1A fix), txn type %s", txn.TransactionType)
		assert.Equal(t, tlPOID, *txn.POID)
		assert.Nil(t, txn.SellingPrice)
	}
	assert.Equal(t, 0.0, resp.Products[0].Metrics.TotalRevenue)
}

func Test_GetInventoryTimeline_PONotInPeriod_Bug2A(t *testing.T) {
	ctx := context.Background()
	invRepo := repositorymocks.NewInventoryRepository(t)
	itemRepo := repositorymocks.NewInventoryItemRepository(t)
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	svc := NewInventoryTimelineService(invRepo, itemRepo, spRepo)

	tlSetupItems(ctx, itemRepo, tlWidgetItems())
	// No period txns at all.
	tlOnTxns(ctx, invRepo, 1, []*models.InventoryTransaction{}, []*repository.InventoryTransactionWithCounter{})
	spRepo.On("GetPOItemsWithPriceByIDs", ctx, []uint(nil), tlInventoryID).Return(map[uint]*repository.POItemSellingPriceInfo{}, nil)

	resp, err := svc.GetInventoryTimeline(ctx, tlReq())
	require.NoError(t, err)
	require.Len(t, resp.Products, 1)
	assert.Empty(t, resp.Products[0].PurchaseOrders, "PO without period activity must not appear (Bug 2A fix)")
	assert.Empty(t, resp.Products[0].Transactions)
}

func Test_GetInventoryTimeline_TransferInCrossInventory(t *testing.T) {
	ctx := context.Background()
	invRepo := repositorymocks.NewInventoryRepository(t)
	itemRepo := repositorymocks.NewInventoryItemRepository(t)
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	svc := NewInventoryTimelineService(invRepo, itemRepo, spRepo)

	foreignPOI := uint(999) // belongs to another inventory's PO
	tlSetupItems(ctx, itemRepo, tlWidgetItems())
	tlOnTxns(ctx, invRepo, 1, []*models.InventoryTransaction{}, []*repository.InventoryTransactionWithCounter{
		tlConsumeTxn(5, 1, models.InventoryTransactionTypeTransferIn, &foreignPOI),
	})
	// Repo returns empty because foreignPOI's PO is for a different inventory.
	spRepo.On("GetPOItemsWithPriceByIDs", ctx, []uint{foreignPOI}, tlInventoryID).Return(map[uint]*repository.POItemSellingPriceInfo{}, nil)

	resp, err := svc.GetInventoryTimeline(ctx, tlReq())
	require.NoError(t, err)
	require.Len(t, resp.Products[0].Transactions, 1)
	assert.Nil(t, resp.Products[0].Transactions[0].POID, "transfer-in's source POI is filtered out")
	assert.Empty(t, resp.Products[0].PurchaseOrders, "source-inventory PO must not appear in destination's PO list")
}

// ---------- transactions[].po_id resolution tests ----------

func Test_GetInventoryTimeline_PurchaseTxnPOID(t *testing.T) {
	ctx := context.Background()
	invRepo := repositorymocks.NewInventoryRepository(t)
	itemRepo := repositorymocks.NewInventoryItemRepository(t)
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	svc := NewInventoryTimelineService(invRepo, itemRepo, spRepo)

	price := decimal.NewFromInt(15)

	tlSetupItems(ctx, itemRepo, tlWidgetItems())
	tlOnTxns(ctx, invRepo, 1, []*models.InventoryTransaction{}, []*repository.InventoryTransactionWithCounter{
		tlPurchaseTxn(1, 5, tlPOItemID),
	})
	spRepo.On("GetPOItemsWithPriceByIDs", ctx, []uint{tlPOItemID}, tlInventoryID).Return(map[uint]*repository.POItemSellingPriceInfo{
		tlPOItemID: tlPOInfo(&price),
	}, nil)

	resp, err := svc.GetInventoryTimeline(ctx, tlReq())
	require.NoError(t, err)
	require.Len(t, resp.Products[0].Transactions, 1)
	require.NotNil(t, resp.Products[0].Transactions[0].POID)
	assert.Equal(t, tlPOID, *resp.Products[0].Transactions[0].POID)
}

func Test_GetInventoryTimeline_OutOfPeriodCounterStillResolves(t *testing.T) {
	ctx := context.Background()
	invRepo := repositorymocks.NewInventoryRepository(t)
	itemRepo := repositorymocks.NewInventoryItemRepository(t)
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	svc := NewInventoryTimelineService(invRepo, itemRepo, spRepo)

	counterPOI := uint(tlPOItemID)
	price := decimal.NewFromInt(40)

	tlSetupItems(ctx, itemRepo, tlWidgetItems())
	tlOnTxns(ctx, invRepo, 1, []*models.InventoryTransaction{}, []*repository.InventoryTransactionWithCounter{
		tlConsumeTxn(2, 1, models.InventoryTransactionTypeSell, &counterPOI),
	})
	spRepo.On("GetPOItemsWithPriceByIDs", ctx, []uint{tlPOItemID}, tlInventoryID).Return(map[uint]*repository.POItemSellingPriceInfo{
		tlPOItemID: tlPOInfo(&price),
	}, nil)

	resp, err := svc.GetInventoryTimeline(ctx, tlReq())
	require.NoError(t, err)
	require.Len(t, resp.Products[0].Transactions, 1)
	require.NotNil(t, resp.Products[0].Transactions[0].POID)
	assert.Equal(t, tlPOID, *resp.Products[0].Transactions[0].POID)
	require.NotNil(t, resp.Products[0].Transactions[0].SellingPrice)
	assert.Equal(t, 40.0, *resp.Products[0].Transactions[0].SellingPrice)
}

// ---------- DTO contract tests ----------

func Test_GetInventoryTimeline_DTO_DroppedFieldsAbsent(t *testing.T) {
	ctx := context.Background()
	invRepo := repositorymocks.NewInventoryRepository(t)
	itemRepo := repositorymocks.NewInventoryItemRepository(t)
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	svc := NewInventoryTimelineService(invRepo, itemRepo, spRepo)

	counterPOI := uint(tlPOItemID)
	price := decimal.NewFromInt(10)

	tlSetupItems(ctx, itemRepo, tlWidgetItems())
	tlOnTxns(ctx, invRepo, 1, []*models.InventoryTransaction{}, []*repository.InventoryTransactionWithCounter{
		tlConsumeTxn(2, 1, models.InventoryTransactionTypeSell, &counterPOI),
	})
	spRepo.On("GetPOItemsWithPriceByIDs", ctx, []uint{tlPOItemID}, tlInventoryID).Return(map[uint]*repository.POItemSellingPriceInfo{
		tlPOItemID: tlPOInfo(&price),
	}, nil)

	resp, err := svc.GetInventoryTimeline(ctx, tlReq())
	require.NoError(t, err)

	raw, err := json.Marshal(resp)
	require.NoError(t, err)
	jsonStr := string(raw)
	assert.NotContains(t, jsonStr, "selling_price_changes", "selling_price_changes must be removed from the response")
	assert.NotContains(t, jsonStr, "has_selling_price", "has_selling_price must be removed from the response")
}

func Test_GetInventoryTimeline_TotalRevenue_MixedNullAndNonNull(t *testing.T) {
	ctx := context.Background()
	invRepo := repositorymocks.NewInventoryRepository(t)
	itemRepo := repositorymocks.NewInventoryItemRepository(t)
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	svc := NewInventoryTimelineService(invRepo, itemRepo, spRepo)

	pricedPOI := uint(tlPOItemID)
	nullPOI := uint(tlPOItemID + 1)
	price := decimal.NewFromInt(25)

	tlSetupItems(ctx, itemRepo, tlWidgetItems())
	tlOnTxns(ctx, invRepo, 1, []*models.InventoryTransaction{}, []*repository.InventoryTransactionWithCounter{
		tlConsumeTxn(2, 4, models.InventoryTransactionTypeSell, &pricedPOI),
		tlConsumeTxn(3, 7, models.InventoryTransactionTypeSell, &nullPOI),
	})
	spRepo.On("GetPOItemsWithPriceByIDs", ctx, mock.MatchedBy(func(ids []uint) bool {
		return len(ids) == 2
	}), tlInventoryID).Return(map[uint]*repository.POItemSellingPriceInfo{
		pricedPOI: {POItemID: pricedPOI, POID: tlPOID, PONumber: "PO-1", ProductID: tlProductID, EffectivePrice: &price},
		nullPOI:   {POItemID: nullPOI, POID: tlPOID + 1, PONumber: "PO-2", ProductID: tlProductID, EffectivePrice: nil},
	}, nil)

	resp, err := svc.GetInventoryTimeline(ctx, tlReq())
	require.NoError(t, err)
	require.Len(t, resp.Products[0].Transactions, 2)
	assert.Equal(t, 100.0, resp.Products[0].Metrics.TotalRevenue) // 4*25; second sell has no price
}

func Test_GetInventoryTimeline_HistoricalBeginningStock(t *testing.T) {
	ctx := context.Background()
	invRepo := repositorymocks.NewInventoryRepository(t)
	itemRepo := repositorymocks.NewInventoryItemRepository(t)
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	svc := NewInventoryTimelineService(invRepo, itemRepo, spRepo)

	tlSetupItems(ctx, itemRepo, tlWidgetItems())
	tlOnTxns(ctx, invRepo, 1, []*models.InventoryTransaction{
		{
			Base:                models.Base{ID: 1},
			InventoryItemID:     tlItemID,
			TransactionType:     models.InventoryTransactionTypePurchase,
			Quantity:            decimal.NewFromInt(10),
			Price:               5.0,
			PurchaseOrderItemID: pkg.Ptr(uint(tlPOItemID)),
		},
		{
			Base:            models.Base{ID: 2},
			InventoryItemID: tlItemID,
			TransactionType: models.InventoryTransactionTypeSell,
			Quantity:        decimal.NewFromInt(3),
			Price:           5.0,
		},
	}, []*repository.InventoryTransactionWithCounter{})
	spRepo.On("GetPOItemsWithPriceByIDs", ctx, []uint(nil), tlInventoryID).Return(map[uint]*repository.POItemSellingPriceInfo{}, nil)

	resp, err := svc.GetInventoryTimeline(ctx, tlReq())
	require.NoError(t, err)
	require.Len(t, resp.Products, 1)
	assert.Equal(t, 7.0, resp.Products[0].BeginningStock) // 10 purchased - 3 sold
	assert.Equal(t, 7.0, resp.Products[0].EndingStock)    // no period activity
}

// ---------- pagination / search delegation tests ----------
//
// Search/sort/slice now happen in SQL (GetByInventoryIDWithFilters +
// CountByInventoryIDWithFilters). These tests cover the service's contribution:
// pagination-metadata math, out-of-range/overflow handling, and that the
// tokenized search + name sort are passed down to the repository.

func Test_GetInventoryTimeline_ReturnsRepoPageInOrder(t *testing.T) {
	ctx := context.Background()
	invRepo := repositorymocks.NewInventoryRepository(t)
	itemRepo := repositorymocks.NewInventoryItemRepository(t)
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	svc := NewInventoryTimelineService(invRepo, itemRepo, spRepo)

	// Repo reports 5 matches total but returns this page of 2 (already sorted).
	itemRepo.On("CountByInventoryIDWithFilters", ctx, tlInventoryID, mock.Anything).Return(int64(5), nil)
	itemRepo.On("GetByInventoryIDWithFilters", ctx, tlInventoryID, mock.Anything, mock.Anything, mock.Anything).
		Return(tlMultiItems("c", "d"), nil)
	tlOnTxns(ctx, invRepo, 2, []*models.InventoryTransaction{}, []*repository.InventoryTransactionWithCounter{})
	spRepo.On("GetPOItemsWithPriceByIDs", ctx, []uint(nil), tlInventoryID).Return(map[uint]*repository.POItemSellingPriceInfo{}, nil)

	req := tlReq()
	req.Page = 2
	req.Limit = 2
	resp, err := svc.GetInventoryTimeline(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, []string{"c", "d"}, tlProductNames(resp)) // returned in repo (DB) order
	assert.Equal(t, dto.TimelinePagination{Page: 2, Limit: 2, Total: 5, TotalPages: 3}, resp.Pagination)
}

func Test_GetInventoryTimeline_DefaultPageAndLimit(t *testing.T) {
	ctx := context.Background()
	invRepo := repositorymocks.NewInventoryRepository(t)
	itemRepo := repositorymocks.NewInventoryItemRepository(t)
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	svc := NewInventoryTimelineService(invRepo, itemRepo, spRepo)

	tlSetupItems(ctx, itemRepo, tlMultiItems("a", "b"))
	tlOnTxns(ctx, invRepo, 2, []*models.InventoryTransaction{}, []*repository.InventoryTransactionWithCounter{})
	spRepo.On("GetPOItemsWithPriceByIDs", ctx, []uint(nil), tlInventoryID).Return(map[uint]*repository.POItemSellingPriceInfo{}, nil)

	resp, err := svc.GetInventoryTimeline(ctx, tlReq()) // no page/limit set
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Pagination.Page)
	assert.Equal(t, defaultTimelinePageLimit, resp.Pagination.Limit)
	assert.Equal(t, int64(2), resp.Pagination.Total)
	assert.Equal(t, 1, resp.Pagination.TotalPages)
	assert.Len(t, resp.Products, 2)
}

func Test_GetInventoryTimeline_OutOfRangePage_NoDataFetch(t *testing.T) {
	ctx := context.Background()
	invRepo := repositorymocks.NewInventoryRepository(t)
	itemRepo := repositorymocks.NewInventoryItemRepository(t)
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	svc := NewInventoryTimelineService(invRepo, itemRepo, spRepo)

	// Only Count is mocked: a page past the end must NOT fetch items/txns.
	itemRepo.On("CountByInventoryIDWithFilters", ctx, tlInventoryID, mock.Anything).Return(int64(3), nil)

	req := tlReq()
	req.Page = 10
	req.Limit = 2
	resp, err := svc.GetInventoryTimeline(ctx, req)
	require.NoError(t, err)
	assert.Empty(t, resp.Products)
	assert.Equal(t, dto.TimelinePagination{Page: 10, Limit: 2, Total: 3, TotalPages: 2}, resp.Pagination)
}

func Test_GetInventoryTimeline_HugePageDoesNotOverflow(t *testing.T) {
	ctx := context.Background()
	invRepo := repositorymocks.NewInventoryRepository(t)
	itemRepo := repositorymocks.NewInventoryItemRepository(t)
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	svc := NewInventoryTimelineService(invRepo, itemRepo, spRepo)

	itemRepo.On("CountByInventoryIDWithFilters", ctx, tlInventoryID, mock.Anything).Return(int64(3), nil)

	req := tlReq()
	req.Page = math.MaxInt64 // (page-1)*limit would overflow if it were evaluated
	req.Limit = 200
	resp, err := svc.GetInventoryTimeline(ctx, req)
	require.NoError(t, err) // must not panic
	assert.Empty(t, resp.Products)
	assert.Equal(t, int64(3), resp.Pagination.Total)
}

func Test_GetInventoryTimeline_EmptyInventory(t *testing.T) {
	ctx := context.Background()
	invRepo := repositorymocks.NewInventoryRepository(t)
	itemRepo := repositorymocks.NewInventoryItemRepository(t)
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	svc := NewInventoryTimelineService(invRepo, itemRepo, spRepo)

	itemRepo.On("CountByInventoryIDWithFilters", ctx, tlInventoryID, mock.Anything).Return(int64(0), nil)

	resp, err := svc.GetInventoryTimeline(ctx, tlReq())
	require.NoError(t, err)
	assert.Empty(t, resp.Products)
	assert.Equal(t, dto.TimelinePagination{Page: 1, Limit: defaultTimelinePageLimit, Total: 0, TotalPages: 0}, resp.Pagination)
}

func Test_GetInventoryTimeline_PassesTokenizedSearchAndNameSort(t *testing.T) {
	ctx := context.Background()
	invRepo := repositorymocks.NewInventoryRepository(t)
	itemRepo := repositorymocks.NewInventoryItemRepository(t)
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	svc := NewInventoryTimelineService(invRepo, itemRepo, spRepo)

	// Assert the service tokenizes the search (lowercased, any-word) and asks
	// the repo to sort by product name ascending.
	filterCheck := mock.MatchedBy(func(f repository.InventoryItemFilters) bool {
		return len(f.SearchTokens) == 2 &&
			f.SearchTokens[0] == "green" && f.SearchTokens[1] == "apple" &&
			f.Sort == string(repository.InventoryItemSortFieldProductName) &&
			f.Order == "ASC"
	})
	itemRepo.On("CountByInventoryIDWithFilters", ctx, tlInventoryID, filterCheck).Return(int64(0), nil)

	req := tlReq()
	req.Search = "Green APPLE"
	resp, err := svc.GetInventoryTimeline(ctx, req)
	require.NoError(t, err)
	assert.Empty(t, resp.Products)
}
