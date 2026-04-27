package services

import (
	"context"
	"encoding/json"
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

func tlInventory() *models.Inventory {
	return &models.Inventory{
		Base: models.Base{ID: tlInventoryID},
		Items: []*models.InventoryItem{
			{
				Base:        models.Base{ID: tlItemID},
				InventoryID: tlInventoryID,
				ProductID:   tlProductID,
				Product:     &models.Product{Base: models.Base{ID: tlProductID}, Name: "Widget"},
				UnitID:      tlUnitID,
				Unit:        &models.Unit{Base: models.Base{ID: tlUnitID}, Name: "kg"},
			},
		},
	}
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

// ---------- selling-price resolution tests ----------

func Test_GetInventoryTimeline_SellSellingPrice_Override(t *testing.T) {
	ctx := context.Background()
	invRepo := repositorymocks.NewInventoryRepository(t)
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	svc := NewInventoryTimelineService(invRepo, spRepo)

	counterPOI := uint(tlPOItemID)
	override := decimal.NewFromInt(99)

	invRepo.On("GetByID", ctx, tlInventoryID).Return(tlInventory(), nil)
	invRepo.On("GetTransactionsByInventoryIDs", ctx, tlInventoryID, (*time.Time)(nil), mock.Anything).Return([]*models.InventoryTransaction{}, nil)
	invRepo.On("GetTransactionsByInventoryIDsWithCounter", ctx, tlInventoryID, mock.Anything, mock.Anything).Return([]*repository.InventoryTransactionWithCounter{
		tlConsumeTxn(2, 3, models.InventoryTransactionTypeSell, &counterPOI),
	}, nil)
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
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	svc := NewInventoryTimelineService(invRepo, spRepo)

	counterPOI := uint(tlPOItemID)
	def := decimal.NewFromInt(50)

	invRepo.On("GetByID", ctx, tlInventoryID).Return(tlInventory(), nil)
	invRepo.On("GetTransactionsByInventoryIDs", ctx, tlInventoryID, (*time.Time)(nil), mock.Anything).Return([]*models.InventoryTransaction{}, nil)
	invRepo.On("GetTransactionsByInventoryIDsWithCounter", ctx, tlInventoryID, mock.Anything, mock.Anything).Return([]*repository.InventoryTransactionWithCounter{
		tlConsumeTxn(2, 2, models.InventoryTransactionTypeSell, &counterPOI),
	}, nil)
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
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	svc := NewInventoryTimelineService(invRepo, spRepo)

	counterPOI := uint(tlPOItemID)

	invRepo.On("GetByID", ctx, tlInventoryID).Return(tlInventory(), nil)
	invRepo.On("GetTransactionsByInventoryIDs", ctx, tlInventoryID, (*time.Time)(nil), mock.Anything).Return([]*models.InventoryTransaction{}, nil)
	invRepo.On("GetTransactionsByInventoryIDsWithCounter", ctx, tlInventoryID, mock.Anything, mock.Anything).Return([]*repository.InventoryTransactionWithCounter{
		tlConsumeTxn(2, 1, models.InventoryTransactionTypeSell, &counterPOI),
	}, nil)
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
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	svc := NewInventoryTimelineService(invRepo, spRepo)

	counterPOI := uint(tlPOItemID)

	invRepo.On("GetByID", ctx, tlInventoryID).Return(tlInventory(), nil)
	invRepo.On("GetTransactionsByInventoryIDs", ctx, tlInventoryID, (*time.Time)(nil), mock.Anything).Return([]*models.InventoryTransaction{}, nil)
	invRepo.On("GetTransactionsByInventoryIDsWithCounter", ctx, tlInventoryID, mock.Anything, mock.Anything).Return([]*repository.InventoryTransactionWithCounter{
		tlConsumeTxn(2, 1, models.InventoryTransactionTypeSell, &counterPOI),
	}, nil)
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
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	svc := NewInventoryTimelineService(invRepo, spRepo)

	invRepo.On("GetByID", ctx, tlInventoryID).Return(tlInventory(), nil)
	invRepo.On("GetTransactionsByInventoryIDs", ctx, tlInventoryID, (*time.Time)(nil), mock.Anything).Return([]*models.InventoryTransaction{}, nil)
	invRepo.On("GetTransactionsByInventoryIDsWithCounter", ctx, tlInventoryID, mock.Anything, mock.Anything).Return([]*repository.InventoryTransactionWithCounter{
		// No CounterPOIID set — counter purchase has no PurchaseOrderItemID.
		tlConsumeTxn(2, 1, models.InventoryTransactionTypeSell, nil),
	}, nil)
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
	// Bug 1A: today's resolution loop only handles sell — disposal/transfer never get po_id.
	// After refactor: every txn with a counter walks the same path.
	ctx := context.Background()
	invRepo := repositorymocks.NewInventoryRepository(t)
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	svc := NewInventoryTimelineService(invRepo, spRepo)

	counterPOI := uint(tlPOItemID)
	price := decimal.NewFromInt(20)

	invRepo.On("GetByID", ctx, tlInventoryID).Return(tlInventory(), nil)
	invRepo.On("GetTransactionsByInventoryIDs", ctx, tlInventoryID, (*time.Time)(nil), mock.Anything).Return([]*models.InventoryTransaction{}, nil)
	invRepo.On("GetTransactionsByInventoryIDsWithCounter", ctx, tlInventoryID, mock.Anything, mock.Anything).Return([]*repository.InventoryTransactionWithCounter{
		tlConsumeTxn(3, 1, models.InventoryTransactionTypeDisposal, &counterPOI),
		tlConsumeTxn(4, 1, models.InventoryTransactionTypeTransferOut, &counterPOI),
	}, nil)
	spRepo.On("GetPOItemsWithPriceByIDs", ctx, []uint{tlPOItemID}, tlInventoryID).Return(map[uint]*repository.POItemSellingPriceInfo{
		tlPOItemID: tlPOInfo(&price),
	}, nil)

	resp, err := svc.GetInventoryTimeline(ctx, tlReq())
	require.NoError(t, err)
	require.Len(t, resp.Products[0].Transactions, 2)
	for _, txn := range resp.Products[0].Transactions {
		require.NotNil(t, txn.POID, "disposal/transfer should have po_id resolved (Bug 1A fix), txn type %s", txn.TransactionType)
		assert.Equal(t, tlPOID, *txn.POID)
		// Disposal/transfer don't carry selling_price — only sells do.
		assert.Nil(t, txn.SellingPrice)
	}
	// Disposal/transfer don't contribute to revenue.
	assert.Equal(t, 0.0, resp.Products[0].Metrics.TotalRevenue)
}

func Test_GetInventoryTimeline_PONotInPeriod_Bug2A(t *testing.T) {
	// Bug 2A: PO list query had no date filter. After: PO list is derived from period txns,
	// so a PO with no period activity must not appear.
	ctx := context.Background()
	invRepo := repositorymocks.NewInventoryRepository(t)
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	svc := NewInventoryTimelineService(invRepo, spRepo)

	invRepo.On("GetByID", ctx, tlInventoryID).Return(tlInventory(), nil)
	invRepo.On("GetTransactionsByInventoryIDs", ctx, tlInventoryID, (*time.Time)(nil), mock.Anything).Return([]*models.InventoryTransaction{}, nil)
	// No period txns at all.
	invRepo.On("GetTransactionsByInventoryIDsWithCounter", ctx, tlInventoryID, mock.Anything, mock.Anything).Return([]*repository.InventoryTransactionWithCounter{}, nil)
	spRepo.On("GetPOItemsWithPriceByIDs", ctx, []uint(nil), tlInventoryID).Return(map[uint]*repository.POItemSellingPriceInfo{}, nil)

	resp, err := svc.GetInventoryTimeline(ctx, tlReq())
	require.NoError(t, err)
	require.Len(t, resp.Products, 1)
	assert.Empty(t, resp.Products[0].PurchaseOrders, "PO without period activity must not appear (Bug 2A fix)")
	assert.Empty(t, resp.Products[0].Transactions)
}

func Test_GetInventoryTimeline_TransferInCrossInventory(t *testing.T) {
	// TransferIn's counter purchase is in a different inventory. Its POI lives outside this
	// timeline's inventory. The repo's inventory_id filter ensures the source PO is not in
	// the destination's purchase_orders[].
	ctx := context.Background()
	invRepo := repositorymocks.NewInventoryRepository(t)
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	svc := NewInventoryTimelineService(invRepo, spRepo)

	foreignPOI := uint(999) // belongs to another inventory's PO
	invRepo.On("GetByID", ctx, tlInventoryID).Return(tlInventory(), nil)
	invRepo.On("GetTransactionsByInventoryIDs", ctx, tlInventoryID, (*time.Time)(nil), mock.Anything).Return([]*models.InventoryTransaction{}, nil)
	invRepo.On("GetTransactionsByInventoryIDsWithCounter", ctx, tlInventoryID, mock.Anything, mock.Anything).Return([]*repository.InventoryTransactionWithCounter{
		tlConsumeTxn(5, 1, models.InventoryTransactionTypeTransferIn, &foreignPOI),
	}, nil)
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
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	svc := NewInventoryTimelineService(invRepo, spRepo)

	price := decimal.NewFromInt(15)

	invRepo.On("GetByID", ctx, tlInventoryID).Return(tlInventory(), nil)
	invRepo.On("GetTransactionsByInventoryIDs", ctx, tlInventoryID, (*time.Time)(nil), mock.Anything).Return([]*models.InventoryTransaction{}, nil)
	invRepo.On("GetTransactionsByInventoryIDsWithCounter", ctx, tlInventoryID, mock.Anything, mock.Anything).Return([]*repository.InventoryTransactionWithCounter{
		tlPurchaseTxn(1, 5, tlPOItemID),
	}, nil)
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
	// Sell whose counter purchase happened before the period. The self-join in the repo
	// query exposes counter.purchase_order_item_id without an extra round-trip, so the
	// timeline still resolves po_id and selling_price.
	ctx := context.Background()
	invRepo := repositorymocks.NewInventoryRepository(t)
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	svc := NewInventoryTimelineService(invRepo, spRepo)

	counterPOI := uint(tlPOItemID)
	price := decimal.NewFromInt(40)

	invRepo.On("GetByID", ctx, tlInventoryID).Return(tlInventory(), nil)
	invRepo.On("GetTransactionsByInventoryIDs", ctx, tlInventoryID, (*time.Time)(nil), mock.Anything).Return([]*models.InventoryTransaction{}, nil)
	// Only the sell is in period; the counter purchase is not in the returned slice.
	// The repo populated CounterPOIID via the self-join.
	invRepo.On("GetTransactionsByInventoryIDsWithCounter", ctx, tlInventoryID, mock.Anything, mock.Anything).Return([]*repository.InventoryTransactionWithCounter{
		tlConsumeTxn(2, 1, models.InventoryTransactionTypeSell, &counterPOI),
	}, nil)
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
	// Verify the JSON contract: selling_price_changes and has_selling_price are gone.
	ctx := context.Background()
	invRepo := repositorymocks.NewInventoryRepository(t)
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	svc := NewInventoryTimelineService(invRepo, spRepo)

	counterPOI := uint(tlPOItemID)
	price := decimal.NewFromInt(10)

	invRepo.On("GetByID", ctx, tlInventoryID).Return(tlInventory(), nil)
	invRepo.On("GetTransactionsByInventoryIDs", ctx, tlInventoryID, (*time.Time)(nil), mock.Anything).Return([]*models.InventoryTransaction{}, nil)
	invRepo.On("GetTransactionsByInventoryIDsWithCounter", ctx, tlInventoryID, mock.Anything, mock.Anything).Return([]*repository.InventoryTransactionWithCounter{
		tlConsumeTxn(2, 1, models.InventoryTransactionTypeSell, &counterPOI),
	}, nil)
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
	// Two sells from two POIs: one priced, one with no resolvable price. Revenue counts only the priced one.
	ctx := context.Background()
	invRepo := repositorymocks.NewInventoryRepository(t)
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	svc := NewInventoryTimelineService(invRepo, spRepo)

	pricedPOI := uint(tlPOItemID)
	nullPOI := uint(tlPOItemID + 1)
	price := decimal.NewFromInt(25)

	invRepo.On("GetByID", ctx, tlInventoryID).Return(tlInventory(), nil)
	invRepo.On("GetTransactionsByInventoryIDs", ctx, tlInventoryID, (*time.Time)(nil), mock.Anything).Return([]*models.InventoryTransaction{}, nil)
	invRepo.On("GetTransactionsByInventoryIDsWithCounter", ctx, tlInventoryID, mock.Anything, mock.Anything).Return([]*repository.InventoryTransactionWithCounter{
		tlConsumeTxn(2, 4, models.InventoryTransactionTypeSell, &pricedPOI),
		tlConsumeTxn(3, 7, models.InventoryTransactionTypeSell, &nullPOI),
	}, nil)
	spRepo.On("GetPOItemsWithPriceByIDs", ctx, mock.MatchedBy(func(ids []uint) bool {
		// Order isn't guaranteed across iterations; just check the set.
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

// ---------- empty / edge cases ----------

func Test_GetInventoryTimeline_NoMatchingProducts_ReturnsEmpty(t *testing.T) {
	ctx := context.Background()
	invRepo := repositorymocks.NewInventoryRepository(t)
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	svc := NewInventoryTimelineService(invRepo, spRepo)

	invRepo.On("GetByID", ctx, tlInventoryID).Return(&models.Inventory{Base: models.Base{ID: tlInventoryID}}, nil)

	req := tlReq()
	req.ProductIDs = []uint{tlProductID + 99} // no inventory item matches
	resp, err := svc.GetInventoryTimeline(ctx, req)
	require.NoError(t, err)
	assert.Empty(t, resp.Products)
}

func Test_GetInventoryTimeline_HistoricalBeginningStock(t *testing.T) {
	ctx := context.Background()
	invRepo := repositorymocks.NewInventoryRepository(t)
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	svc := NewInventoryTimelineService(invRepo, spRepo)

	invRepo.On("GetByID", ctx, tlInventoryID).Return(tlInventory(), nil)
	invRepo.On("GetTransactionsByInventoryIDs", ctx, tlInventoryID, (*time.Time)(nil), mock.Anything).Return([]*models.InventoryTransaction{
		{
			Base:            models.Base{ID: 1},
			InventoryItemID: tlItemID,
			TransactionType: models.InventoryTransactionTypePurchase,
			Quantity:        decimal.NewFromInt(10),
			Price:           5.0,
			PurchaseOrderItemID: pkg.Ptr(uint(tlPOItemID)),
		},
		{
			Base:            models.Base{ID: 2},
			InventoryItemID: tlItemID,
			TransactionType: models.InventoryTransactionTypeSell,
			Quantity:        decimal.NewFromInt(3),
			Price:           5.0,
		},
	}, nil)
	invRepo.On("GetTransactionsByInventoryIDsWithCounter", ctx, tlInventoryID, mock.Anything, mock.Anything).Return([]*repository.InventoryTransactionWithCounter{}, nil)
	spRepo.On("GetPOItemsWithPriceByIDs", ctx, []uint(nil), tlInventoryID).Return(map[uint]*repository.POItemSellingPriceInfo{}, nil)

	resp, err := svc.GetInventoryTimeline(ctx, tlReq())
	require.NoError(t, err)
	require.Len(t, resp.Products, 1)
	assert.Equal(t, 7.0, resp.Products[0].BeginningStock) // 10 purchased - 3 sold
	assert.Equal(t, 7.0, resp.Products[0].EndingStock)    // no period activity
}
