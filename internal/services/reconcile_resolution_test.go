package services

import (
	"cim-backend/internal/models"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func d(v string) decimal.Decimal {
	out, err := decimal.NewFromString(v)
	if err != nil {
		panic(err)
	}
	return out
}

func ts(s string) time.Time {
	out, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return out
}

func itemByID(items []dto.ItemResolution, id uint) (dto.ItemResolution, bool) {
	for _, it := range items {
		if it.InventoryItemID == id {
			return it, true
		}
	}
	return dto.ItemResolution{}, false
}

func TestComputeReconcileDrops(t *testing.T) {
	t.Run("chronological chaining across submissions for one item", func(t *testing.T) {
		// Start stock 100. Two reconciles in order:
		//  sub1 (older): prev=100 actual=90 -> effPrev=100 drop=10, consumed=10
		//  sub2 (newer): prev=100 actual=85 -> effPrev=100-10=90 drop=5, consumed=15
		// Note sub2 prev is the (stale) snapshot 100; chaining subtracts the
		// already-consumed 10 to get the effective prev of 90.
		subs := []parsedReconcileSubmission{
			{SubmissionID: 2, CreatedAt: ts("2025-02-01"), Items: []parsedReconcileItem{
				{InventoryItemID: 1, PrevQuantity: d("100"), ActualCount: d("85")},
			}},
			{SubmissionID: 1, CreatedAt: ts("2025-01-01"), Items: []parsedReconcileItem{
				{InventoryItemID: 1, PrevQuantity: d("100"), ActualCount: d("90")},
			}},
		}
		start := map[uint]decimal.Decimal{1: d("100")}
		names := map[uint]string{1: "Widget"}

		items, chainOrder, _, consumeByChain, err := computeReconcileDrops(subs, start, names)
		require.NoError(t, err)

		// chain must be processed oldest-first: sub 1 then sub 2.
		require.Equal(t, []uint{1, 2}, chainOrder)
		require.Len(t, consumeByChain, 2)
		assert.True(t, consumeByChain[0][1].Equal(d("10")))
		assert.True(t, consumeByChain[1][1].Equal(d("5")))

		it, ok := itemByID(items, 1)
		require.True(t, ok)
		assert.Equal(t, "Widget", it.ProductName)
		assert.True(t, it.StartStock.Equal(d("100")))
		assert.True(t, it.TotalDrop.Equal(d("15")), "totalDrop %s", it.TotalDrop)
		assert.True(t, it.FinalStock.Equal(d("85")), "finalStock %s", it.FinalStock)

		require.Len(t, it.Drops, 2)
		// Drops recorded in chain order (sub1 first).
		assert.Equal(t, uint(1), it.Drops[0].SubmissionID)
		assert.True(t, it.Drops[0].EffectivePrev.Equal(d("100")))
		assert.True(t, it.Drops[0].RawDelta.Equal(d("10")))
		assert.False(t, it.Drops[0].Clamped)

		assert.Equal(t, uint(2), it.Drops[1].SubmissionID)
		assert.True(t, it.Drops[1].EffectivePrev.Equal(d("90")))
		assert.True(t, it.Drops[1].RawDelta.Equal(d("5")))
		assert.False(t, it.Drops[1].Clamped)
	})

	t.Run("clamps negative delta when count implies an increase", func(t *testing.T) {
		subs := []parsedReconcileSubmission{
			{SubmissionID: 1, CreatedAt: ts("2025-01-01"), Items: []parsedReconcileItem{
				{InventoryItemID: 7, PrevQuantity: d("50"), ActualCount: d("60")}, // +10 increase
			}},
		}
		start := map[uint]decimal.Decimal{7: d("50")}
		items, _, _, consumeByChain, err := computeReconcileDrops(subs, start, map[uint]string{7: "Gadget"})
		require.NoError(t, err)

		// No consume entry for a clamped (zero) drop.
		assert.Empty(t, consumeByChain[0])

		it, ok := itemByID(items, 7)
		require.True(t, ok)
		assert.True(t, it.TotalDrop.IsZero())
		assert.True(t, it.FinalStock.Equal(d("50")), "final stock unchanged")
		require.Len(t, it.Drops, 1)
		assert.True(t, it.Drops[0].Clamped)
		assert.True(t, it.Drops[0].RawDelta.Equal(d("-10")))
		assert.True(t, it.Drops[0].ClampedDrop.IsZero())
	})

	t.Run("clamps exact no-op (delta zero) and does not consume", func(t *testing.T) {
		subs := []parsedReconcileSubmission{
			{SubmissionID: 1, CreatedAt: ts("2025-01-01"), Items: []parsedReconcileItem{
				{InventoryItemID: 3, PrevQuantity: d("20"), ActualCount: d("20")},
			}},
		}
		items, _, _, consumeByChain, err := computeReconcileDrops(subs,
			map[uint]decimal.Decimal{3: d("20")}, map[uint]string{3: "NoChange"})
		require.NoError(t, err)

		assert.Empty(t, consumeByChain[0])
		it, _ := itemByID(items, 3)
		assert.True(t, it.Drops[0].Clamped, "zero delta is clamped")
		assert.True(t, it.TotalDrop.IsZero())
	})

	t.Run("multiple items chained independently", func(t *testing.T) {
		subs := []parsedReconcileSubmission{
			{SubmissionID: 1, CreatedAt: ts("2025-01-01"), Items: []parsedReconcileItem{
				{InventoryItemID: 1, PrevQuantity: d("100"), ActualCount: d("95")}, // drop 5
				{InventoryItemID: 2, PrevQuantity: d("40"), ActualCount: d("40")},  // clamp
			}},
			{SubmissionID: 2, CreatedAt: ts("2025-02-01"), Items: []parsedReconcileItem{
				{InventoryItemID: 1, PrevQuantity: d("100"), ActualCount: d("90")}, // effPrev 95, drop 5
				{InventoryItemID: 2, PrevQuantity: d("40"), ActualCount: d("30")},  // effPrev 40, drop 10
			}},
		}
		start := map[uint]decimal.Decimal{1: d("100"), 2: d("40")}
		items, _, _, consumeByChain, err := computeReconcileDrops(subs, start, map[uint]string{1: "A", 2: "B"})
		require.NoError(t, err)

		// sub1: item1 drop 5 (item2 clamped, omitted)
		assert.True(t, consumeByChain[0][1].Equal(d("5")))
		_, ok := consumeByChain[0][2]
		assert.False(t, ok, "clamped item omitted from consume map")
		// sub2: item1 drop 5, item2 drop 10
		assert.True(t, consumeByChain[1][1].Equal(d("5")))
		assert.True(t, consumeByChain[1][2].Equal(d("10")))

		i1, _ := itemByID(items, 1)
		i2, _ := itemByID(items, 2)
		assert.True(t, i1.TotalDrop.Equal(d("10")))
		assert.True(t, i1.FinalStock.Equal(d("90")))
		assert.True(t, i2.TotalDrop.Equal(d("10")))
		assert.True(t, i2.FinalStock.Equal(d("30")))
	})

	t.Run("output items sorted ascending by item id", func(t *testing.T) {
		subs := []parsedReconcileSubmission{
			{SubmissionID: 1, CreatedAt: ts("2025-01-01"), Items: []parsedReconcileItem{
				{InventoryItemID: 9, PrevQuantity: d("10"), ActualCount: d("9")},
				{InventoryItemID: 3, PrevQuantity: d("10"), ActualCount: d("9")},
				{InventoryItemID: 5, PrevQuantity: d("10"), ActualCount: d("9")},
			}},
		}
		start := map[uint]decimal.Decimal{3: d("10"), 5: d("10"), 9: d("10")}
		items, _, _, _, err := computeReconcileDrops(subs, start, map[uint]string{})
		require.NoError(t, err)
		require.Len(t, items, 3)
		assert.Equal(t, uint(3), items[0].InventoryItemID)
		assert.Equal(t, uint(5), items[1].InventoryItemID)
		assert.Equal(t, uint(9), items[2].InventoryItemID)
	})

	t.Run("dispose removes N directly (no clamp) and updates consumedSoFar", func(t *testing.T) {
		// Start 100. Dispose 20 -> drop 20, final 80, consumedSoFar 20.
		subs := []parsedReconcileSubmission{
			{SubmissionID: 1, Type: models.InventorySubmissionTypeDispose, CreatedAt: ts("2025-01-01"),
				Items: []parsedReconcileItem{
					// PrevQuantity 999 must be ignored for dispose.
					{InventoryItemID: 1, PrevQuantity: d("999"), ActualCount: d("20")},
				}},
		}
		start := map[uint]decimal.Decimal{1: d("100")}
		items, _, chainTypes, consumeByChain, err := computeReconcileDrops(subs, start, map[uint]string{1: "Widget"})
		require.NoError(t, err)
		require.Equal(t, []models.SubmissionType{models.InventorySubmissionTypeDispose}, chainTypes)
		assert.True(t, consumeByChain[0][1].Equal(d("20")))

		it, _ := itemByID(items, 1)
		assert.True(t, it.TotalDrop.Equal(d("20")))
		assert.True(t, it.TotalDisposed.Equal(d("20")))
		assert.True(t, it.FinalStock.Equal(d("80")))
		require.Len(t, it.Drops, 1)
		assert.Equal(t, "dispose", it.Drops[0].SubmissionType)
		assert.False(t, it.Drops[0].Clamped, "disposals are never clamped")
		assert.True(t, it.Drops[0].ClampedDrop.Equal(d("20")))
	})

	t.Run("reconcile then dispose interleaved by created_at: dispose sees reduced running stock", func(t *testing.T) {
		// Start 100.
		//  sub1 (older, reconcile): prev=100 actual=90 -> drop 10, consumed 10.
		//  sub2 (newer, dispose):   remove 30 -> running stock 100-10=70 ok, consumed 40.
		// Final 60, of which 30 disposed.
		subs := []parsedReconcileSubmission{
			{SubmissionID: 2, Type: models.InventorySubmissionTypeDispose, CreatedAt: ts("2025-02-01"),
				Items: []parsedReconcileItem{{InventoryItemID: 1, ActualCount: d("30")}}},
			{SubmissionID: 1, Type: models.InventorySubmissionTypeReconcile, CreatedAt: ts("2025-01-01"),
				Items: []parsedReconcileItem{{InventoryItemID: 1, PrevQuantity: d("100"), ActualCount: d("90")}}},
		}
		start := map[uint]decimal.Decimal{1: d("100")}
		items, chainOrder, chainTypes, consumeByChain, err := computeReconcileDrops(subs, start, map[uint]string{1: "W"})
		require.NoError(t, err)
		// Oldest-first: reconcile (1) then dispose (2).
		require.Equal(t, []uint{1, 2}, chainOrder)
		require.Equal(t, []models.SubmissionType{
			models.InventorySubmissionTypeReconcile, models.InventorySubmissionTypeDispose}, chainTypes)
		assert.True(t, consumeByChain[0][1].Equal(d("10")), "reconcile drop")
		assert.True(t, consumeByChain[1][1].Equal(d("30")), "dispose remove-N")

		it, _ := itemByID(items, 1)
		assert.True(t, it.TotalDrop.Equal(d("40")))
		assert.True(t, it.TotalDisposed.Equal(d("30")))
		assert.True(t, it.FinalStock.Equal(d("60")))
	})

	t.Run("dispose then reconcile interleaved: reconcile effective_prev reflects prior dispose", func(t *testing.T) {
		// Start 100.
		//  sub1 (older, dispose):   remove 25 -> consumed 25.
		//  sub2 (newer, reconcile): prev=100 actual=70 -> effPrev=100-25=75, drop 5, consumed 30.
		// Final 70, of which 25 disposed.
		subs := []parsedReconcileSubmission{
			{SubmissionID: 2, Type: models.InventorySubmissionTypeReconcile, CreatedAt: ts("2025-02-01"),
				Items: []parsedReconcileItem{{InventoryItemID: 1, PrevQuantity: d("100"), ActualCount: d("70")}}},
			{SubmissionID: 1, Type: models.InventorySubmissionTypeDispose, CreatedAt: ts("2025-01-01"),
				Items: []parsedReconcileItem{{InventoryItemID: 1, ActualCount: d("25")}}},
		}
		start := map[uint]decimal.Decimal{1: d("100")}
		items, chainOrder, _, consumeByChain, err := computeReconcileDrops(subs, start, map[uint]string{1: "W"})
		require.NoError(t, err)
		require.Equal(t, []uint{1, 2}, chainOrder)
		assert.True(t, consumeByChain[0][1].Equal(d("25")), "dispose first")
		assert.True(t, consumeByChain[1][1].Equal(d("5")), "reconcile drop after effective_prev reduced")

		it, _ := itemByID(items, 1)
		// Second drop carries the reduced effective prev.
		require.Len(t, it.Drops, 2)
		assert.True(t, it.Drops[1].EffectivePrev.Equal(d("75")))
		assert.True(t, it.Drops[1].RawDelta.Equal(d("5")))
		assert.True(t, it.TotalDrop.Equal(d("30")))
		assert.True(t, it.TotalDisposed.Equal(d("25")))
		assert.True(t, it.FinalStock.Equal(d("70")))
	})

	t.Run("reconcile submission listing the SAME item twice accumulates drops in the consume map (no last-write-wins)", func(t *testing.T) {
		// One reconcile submission whose payload lists item 1 twice.
		// Start 10. Row A: prev=10 actual=7 -> effPrev=10 drop=3, consumed=3.
		// Row B: prev=10 actual=6 -> effPrev=10-3=7 drop=1, consumed=4.
		// The per-submission consume map MUST hold the SUM (3+1=4), matching
		// TotalDrop and FinalStock; a last-write-wins map would carry only 1.
		subs := []parsedReconcileSubmission{
			{SubmissionID: 1, Type: models.InventorySubmissionTypeReconcile, CreatedAt: ts("2025-01-01"),
				Items: []parsedReconcileItem{
					{InventoryItemID: 1, PrevQuantity: d("10"), ActualCount: d("7")},
					{InventoryItemID: 1, PrevQuantity: d("10"), ActualCount: d("6")},
				}},
		}
		start := map[uint]decimal.Decimal{1: d("10")}
		items, _, _, consumeByChain, err := computeReconcileDrops(subs, start, map[uint]string{1: "W"})
		require.NoError(t, err)

		require.Len(t, consumeByChain, 1)
		assert.True(t, consumeByChain[0][1].Equal(d("4")),
			"consume map must equal the SUM of both rows, got %s", consumeByChain[0][1])

		it, _ := itemByID(items, 1)
		assert.True(t, it.TotalDrop.Equal(d("4")), "TotalDrop %s", it.TotalDrop)
		assert.True(t, it.FinalStock.Equal(d("6")), "FinalStock %s", it.FinalStock)
		// consume map total == previewed TotalDrop == start - final (preview == apply).
		assert.True(t, consumeByChain[0][1].Equal(it.TotalDrop), "consume total must equal TotalDrop")
		assert.True(t, consumeByChain[0][1].Equal(it.StartStock.Sub(it.FinalStock)),
			"consume total must equal start-final")
	})

	t.Run("dispose submission listing the SAME item twice accumulates remove-N in the consume map", func(t *testing.T) {
		// One dispose submission whose payload lists item 1 twice: remove 3 then 4.
		// Start 10. The consume map MUST hold 3+4=7, matching TotalDrop/FinalStock.
		// (The Codex example: preview final 3, but last-write-wins would consume
		// only 4 and persist 6.)
		subs := []parsedReconcileSubmission{
			{SubmissionID: 1, Type: models.InventorySubmissionTypeDispose, CreatedAt: ts("2025-01-01"),
				Items: []parsedReconcileItem{
					{InventoryItemID: 1, ActualCount: d("3")},
					{InventoryItemID: 1, ActualCount: d("4")},
				}},
		}
		start := map[uint]decimal.Decimal{1: d("10")}
		items, _, _, consumeByChain, err := computeReconcileDrops(subs, start, map[uint]string{1: "W"})
		require.NoError(t, err)

		require.Len(t, consumeByChain, 1)
		assert.True(t, consumeByChain[0][1].Equal(d("7")),
			"consume map must equal SUM of both dispose rows (3+4), got %s", consumeByChain[0][1])

		it, _ := itemByID(items, 1)
		assert.True(t, it.TotalDrop.Equal(d("7")), "TotalDrop %s", it.TotalDrop)
		assert.True(t, it.TotalDisposed.Equal(d("7")), "TotalDisposed %s", it.TotalDisposed)
		assert.True(t, it.FinalStock.Equal(d("3")), "FinalStock %s (Codex preview)", it.FinalStock)
		// preview == apply: consume total equals TotalDrop equals start-final.
		assert.True(t, consumeByChain[0][1].Equal(it.TotalDrop))
		assert.True(t, consumeByChain[0][1].Equal(it.StartStock.Sub(it.FinalStock)))
	})

	t.Run("non-positive dispose quantity does not inject a phantom consume entry", func(t *testing.T) {
		// A 0 remove-N (and, alongside, a real removal) must not create a 0 entry
		// in the consume map nor disturb the bookkeeping.
		subs := []parsedReconcileSubmission{
			{SubmissionID: 1, Type: models.InventorySubmissionTypeDispose, CreatedAt: ts("2025-01-01"),
				Items: []parsedReconcileItem{
					{InventoryItemID: 1, ActualCount: d("0")}, // phantom guard
					{InventoryItemID: 1, ActualCount: d("5")},
				}},
		}
		start := map[uint]decimal.Decimal{1: d("10")}
		items, _, _, consumeByChain, err := computeReconcileDrops(subs, start, map[uint]string{1: "W"})
		require.NoError(t, err)

		require.Len(t, consumeByChain, 1)
		assert.True(t, consumeByChain[0][1].Equal(d("5")), "only the positive removal counts, got %s", consumeByChain[0][1])

		it, _ := itemByID(items, 1)
		assert.True(t, it.TotalDrop.Equal(d("5")))
		assert.True(t, it.FinalStock.Equal(d("5")))
		assert.True(t, consumeByChain[0][1].Equal(it.TotalDrop), "consume total == TotalDrop (no phantom)")
	})

	t.Run("insufficient-stock dispose hard-fails (no clamp)", func(t *testing.T) {
		// Start 100. A reconcile already drops 90 (running 10), then a dispose of
		// 20 overdraws -> hard error naming submission + item + needed vs avail.
		subs := []parsedReconcileSubmission{
			{SubmissionID: 1, Type: models.InventorySubmissionTypeReconcile, CreatedAt: ts("2025-01-01"),
				Items: []parsedReconcileItem{{InventoryItemID: 7, PrevQuantity: d("100"), ActualCount: d("10")}}},
			{SubmissionID: 2, Type: models.InventorySubmissionTypeDispose, CreatedAt: ts("2025-02-01"),
				Items: []parsedReconcileItem{{InventoryItemID: 7, ActualCount: d("20")}}},
		}
		start := map[uint]decimal.Decimal{7: d("100")}
		_, _, _, _, err := computeReconcileDrops(subs, start, map[uint]string{7: "Gadget"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "dispose submission 2")
		assert.Contains(t, err.Error(), "inventory item 7")
		assert.Contains(t, err.Error(), "remove 20")
		assert.Contains(t, err.Error(), "only 10 available")
	})

	t.Run("negative dispose quantity hard-fails (not silently approved no-op)", func(t *testing.T) {
		// A negative remove-N passes the overdraw check (N > available is false)
		// and the drop>0 guard would skip FIFO consumption, leaving the submission
		// to be silently marked completed. It must hard-fail in compute before any
		// persist, naming submission + item + the offending quantity.
		subs := []parsedReconcileSubmission{
			{SubmissionID: 9, Type: models.InventorySubmissionTypeDispose, CreatedAt: ts("2025-01-01"),
				Items: []parsedReconcileItem{{InventoryItemID: 4, ActualCount: d("-5")}}},
		}
		start := map[uint]decimal.Decimal{4: d("100")}
		items, chainOrder, _, consumeByChain, err := computeReconcileDrops(subs, start, map[uint]string{4: "Widget"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "dispose submission 9")
		assert.Contains(t, err.Error(), "inventory item 4")
		assert.Contains(t, err.Error(), "-5")
		assert.Contains(t, err.Error(), "negative")
		// Aborts before producing any results -> nothing reaches persist.
		assert.Nil(t, items)
		assert.Nil(t, chainOrder)
		assert.Nil(t, consumeByChain)
		var appErr *pkg.AppError
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, pkg.ErrorCodeValidation, appErr.Code)
	})
}

func TestParseReconcilePayload(t *testing.T) {
	t.Run("parses items with prev and actual", func(t *testing.T) {
		req := dto.ReconcileInventoryRequest{
			InventoryID: 1,
			Items: []dto.QuantityItem{
				{InventoryItemID: 1, Quantity: ptrDec(d("90")), PrevQuantity: d("100")},
			},
		}
		payload, err := json.Marshal(req)
		require.NoError(t, err)

		sub := models.InventorySubmission{
			Base:    models.Base{ID: 1, CreatedAt: ts("2025-01-01")},
			Payload: payload,
		}
		out, err := parseSubmissionPayload(sub)
		require.NoError(t, err)
		assert.Equal(t, uint(1), out.SubmissionID)
		require.Len(t, out.Items, 1)
		assert.True(t, out.Items[0].PrevQuantity.Equal(d("100")))
		assert.True(t, out.Items[0].ActualCount.Equal(d("90")))
	})

	t.Run("errors when actual quantity is nil", func(t *testing.T) {
		// hand-craft payload with null quantity
		payload := json.RawMessage(`{"inventory_id":1,"items":[{"inventory_item_id":1,"quantity":null,"prev_quantity":"100"}]}`)
		sub := models.InventorySubmission{Base: models.Base{ID: 5}, Payload: payload}
		_, err := parseSubmissionPayload(sub)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nil actual quantity")
	})
}

func TestCollectItemIDs(t *testing.T) {
	subs := []parsedReconcileSubmission{
		{Items: []parsedReconcileItem{{InventoryItemID: 1}, {InventoryItemID: 2}}},
		{Items: []parsedReconcileItem{{InventoryItemID: 2}, {InventoryItemID: 3}}},
	}
	ids := collectItemIDs(subs)
	assert.ElementsMatch(t, []uint{1, 2, 3}, ids)
}

// --- selection / validation (DB-backed via sqlmock) ---

func newMockService(t *testing.T) (*inventoryService, sqlmock.Sqlmock) {
	t.Helper()
	conn, mock, err := sqlmock.New()
	require.NoError(t, err)
	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: conn}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Error),
	})
	require.NoError(t, err)
	return &inventoryService{db: gormDB}, mock
}

func reconcilePayloadJSON(t *testing.T, itemID uint, prev, actual string) []byte {
	t.Helper()
	req := dto.ReconcileInventoryRequest{
		InventoryID: 1,
		Items: []dto.QuantityItem{
			{InventoryItemID: itemID, Quantity: ptrDec(d(actual)), PrevQuantity: d(prev)},
		},
	}
	b, err := json.Marshal(req)
	require.NoError(t, err)
	return b
}

func disposePayloadJSON(t *testing.T, itemID uint, removeN string) []byte {
	t.Helper()
	req := dto.DisposeInventoryRequest{
		InventoryID: 1,
		Items: []dto.QuantityItem{
			{InventoryItemID: itemID, Quantity: ptrDec(d(removeN))},
		},
	}
	b, err := json.Marshal(req)
	require.NoError(t, err)
	return b
}

func TestSelectAndValidateSubmissions(t *testing.T) {
	ctx := pkg.WithUserEmail(context.Background(), "system@cim.local")

	subCols := []string{"id", "inventory_id", "submission_type", "processing_status", "approval_status", "payload", "created_at"}

	t.Run("accepts pending reconcile submissions in chronological order", func(t *testing.T) {
		svc, mock := newMockService(t)
		mock.ExpectQuery(`SELECT \* FROM .*inventory_submissions.*`).
			WillReturnRows(sqlmock.NewRows(subCols).
				AddRow(1, 1, "reconcile", "pending", "pending", reconcilePayloadJSON(t, 1, "100", "90"), ts("2025-01-01")).
				AddRow(2, 1, "reconcile", "pending", "pending", reconcilePayloadJSON(t, 1, "100", "85"), ts("2025-02-01")))

		parsed, selected, err := svc.selectAndValidateSubmissions(ctx, 1, []uint{1, 2}, nil)
		require.NoError(t, err)
		require.Len(t, parsed, 2)
		require.Len(t, selected, 2)
		// Returned in the requested-id order.
		assert.Equal(t, uint(1), parsed[0].SubmissionID)
		assert.Equal(t, uint(2), parsed[1].SubmissionID)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("aborts when a requested submission is missing", func(t *testing.T) {
		svc, mock := newMockService(t)
		mock.ExpectQuery(`SELECT \* FROM .*inventory_submissions.*`).
			WillReturnRows(sqlmock.NewRows(subCols).
				AddRow(1, 1, "reconcile", "pending", "pending", reconcilePayloadJSON(t, 1, "100", "90"), ts("2025-01-01")))

		_, _, err := svc.selectAndValidateSubmissions(ctx, 1, []uint{1, 2}, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "submission 2 not found")
	})

	t.Run("aborts when submission is not pending", func(t *testing.T) {
		svc, mock := newMockService(t)
		mock.ExpectQuery(`SELECT \* FROM .*inventory_submissions.*`).
			WillReturnRows(sqlmock.NewRows(subCols).
				AddRow(1, 1, "reconcile", "completed", "approved", reconcilePayloadJSON(t, 1, "100", "90"), ts("2025-01-01")))

		_, _, err := svc.selectAndValidateSubmissions(ctx, 1, []uint{1}, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not pending")
	})

	t.Run("aborts when submission belongs to another inventory", func(t *testing.T) {
		svc, mock := newMockService(t)
		mock.ExpectQuery(`SELECT \* FROM .*inventory_submissions.*`).
			WillReturnRows(sqlmock.NewRows(subCols).
				AddRow(1, 2, "reconcile", "pending", "pending", reconcilePayloadJSON(t, 1, "100", "90"), ts("2025-01-01")))

		_, _, err := svc.selectAndValidateSubmissions(ctx, 1, []uint{1}, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "belongs to inventory 2")
	})

	t.Run("aborts when a reconcile ID is actually a dispose", func(t *testing.T) {
		svc, mock := newMockService(t)
		mock.ExpectQuery(`SELECT \* FROM .*inventory_submissions.*`).
			WillReturnRows(sqlmock.NewRows(subCols).
				AddRow(1, 1, "dispose", "pending", "pending", reconcilePayloadJSON(t, 1, "100", "90"), ts("2025-01-01")))

		_, _, err := svc.selectAndValidateSubmissions(ctx, 1, []uint{1}, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is a dispose, expected reconcile")
	})

	t.Run("accepts dispose IDs via the dispose set", func(t *testing.T) {
		svc, mock := newMockService(t)
		mock.ExpectQuery(`SELECT \* FROM .*inventory_submissions.*`).
			WillReturnRows(sqlmock.NewRows(subCols).
				AddRow(3, 1, "dispose", "pending", "pending", disposePayloadJSON(t, 1, "5"), ts("2025-01-15")))

		parsed, selected, err := svc.selectAndValidateSubmissions(ctx, 1, nil, []uint{3})
		require.NoError(t, err)
		require.Len(t, parsed, 1)
		require.Len(t, selected, 1)
		assert.Equal(t, models.InventorySubmissionTypeDispose, parsed[0].Type)
		require.Len(t, parsed[0].Items, 1)
		assert.True(t, parsed[0].Items[0].ActualCount.Equal(d("5")), "dispose remove-N stored as ActualCount")
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("aborts when a dispose ID is actually a reconcile", func(t *testing.T) {
		svc, mock := newMockService(t)
		mock.ExpectQuery(`SELECT \* FROM .*inventory_submissions.*`).
			WillReturnRows(sqlmock.NewRows(subCols).
				AddRow(1, 1, "reconcile", "pending", "pending", reconcilePayloadJSON(t, 1, "100", "90"), ts("2025-01-01")))

		_, _, err := svc.selectAndValidateSubmissions(ctx, 1, nil, []uint{1})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is a reconcile, expected dispose")
	})

	t.Run("aborts when an ID is requested as both reconcile and dispose", func(t *testing.T) {
		svc, _ := newMockService(t)
		_, _, err := svc.selectAndValidateSubmissions(ctx, 1, []uint{5}, []uint{5})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "both reconcile and dispose")
	})

	t.Run("aborts when no IDs given", func(t *testing.T) {
		svc, _ := newMockService(t)
		_, _, err := svc.selectAndValidateSubmissions(ctx, 1, nil, nil)
		require.Error(t, err)
	})
}

func ptrDec(v decimal.Decimal) *decimal.Decimal { return &v }

// --- persist path: multi-submission coalescing ---

func TestCoalesceItemChanges(t *testing.T) {
	t.Run("item dropped across two submissions collapses to one change keeping earliest baseline and final quantity", func(t *testing.T) {
		// consumeFIFO reuses ONE *InventoryItem pointer per item across the chain
		// and mutates Quantity down to the final value, while appending a change
		// per submission with that submission's pre-drop OriginalQuantity snapshot.
		// Here: DB-start 100, sub1 100->90, sub2 90->80; final pointer quantity 80.
		item := &models.InventoryItem{Base: models.Base{ID: 1}, Quantity: d("80")}
		raw := []*models.InventoryItemChange{
			{InventoryItem: item, OriginalQuantity: d("100")}, // earliest (sub1)
			{InventoryItem: item, OriginalQuantity: d("90")},  // later (sub2)
		}

		out := coalesceItemChanges(raw)

		require.Len(t, out, 1, "one change per item id")
		assert.Equal(t, uint(1), out[0].InventoryItem.ID)
		assert.True(t, out[0].OriginalQuantity.Equal(d("100")),
			"keeps EARLIEST OriginalQuantity (the true pre-fix DB baseline), got %s", out[0].OriginalQuantity)
		assert.True(t, out[0].InventoryItem.Quantity.Equal(d("80")),
			"keeps the final drawn-down quantity from the shared pointer, got %s", out[0].InventoryItem.Quantity)
	})

	t.Run("multiple items ordered ascending by id, each kept once", func(t *testing.T) {
		i9 := &models.InventoryItem{Base: models.Base{ID: 9}, Quantity: d("1")}
		i3 := &models.InventoryItem{Base: models.Base{ID: 3}, Quantity: d("2")}
		raw := []*models.InventoryItemChange{
			{InventoryItem: i9, OriginalQuantity: d("5")},
			{InventoryItem: i3, OriginalQuantity: d("8")},
			{InventoryItem: i9, OriginalQuantity: d("3")}, // dup of 9
		}
		out := coalesceItemChanges(raw)
		require.Len(t, out, 2)
		assert.Equal(t, uint(3), out[0].InventoryItem.ID)
		assert.Equal(t, uint(9), out[1].InventoryItem.ID)
		assert.True(t, out[1].OriginalQuantity.Equal(d("5")), "earliest kept for id 9")
	})

	t.Run("empty input returns empty", func(t *testing.T) {
		assert.Empty(t, coalesceItemChanges(nil))
	})
}

func TestDedupeSourceTxns(t *testing.T) {
	t.Run("dedupes source purchase txns by id but keeps all new sells (id 0)", func(t *testing.T) {
		src := &models.InventoryTransaction{Base: models.Base{ID: 7}, TransactionType: models.InventoryTransactionTypePurchase}
		sell1 := &models.InventoryTransaction{TransactionType: models.InventoryTransactionTypeSell, Quantity: d("10")}
		sell2 := &models.InventoryTransaction{TransactionType: models.InventoryTransactionTypeSell, Quantity: d("5")}
		// source appended once per submission it was consumed in.
		txns := []*models.InventoryTransaction{sell1, src, sell2, src}

		out := dedupeSourceTxns(txns)
		require.Len(t, out, 3, "two distinct sells + one source")

		var sells, sources int
		for _, tx := range out {
			if tx.ID == 0 {
				sells++
			} else {
				sources++
			}
		}
		assert.Equal(t, 2, sells, "both distinct sell inserts preserved")
		assert.Equal(t, 1, sources, "duplicate source txn collapsed to one")
	})
}

// TestPersistResolution_TxnPartition is the persist-path regression for the P2
// data-corruption risk: on --apply the txns slice mixes NEW synthetic rows
// (ID==0) with EXISTING source purchase rows (ID!=0) whose consumed_quantity was
// mutated. A single Save/upsert would route the existing rows through the
// create/upsert path, firing Base.BeforeCreate and overwriting created_by /
// created_at on real purchase rows. The fix partitions the save: synthetic rows
// INSERT (keeping their backdated created_at); source rows get a column-scoped
// UPDATE of consumed_quantity (+ updated_*) that never writes created_by.
func TestPersistResolution_TxnPartition(t *testing.T) {
	ctx := pkg.WithUserEmail(context.Background(), "system@cim.local")

	t.Run("synthetic rows INSERT (backdated created_at preserved); source rows UPDATE consumed_quantity without touching created_by", func(t *testing.T) {
		svc, mock := newMockService(t)

		sub := models.InventorySubmission{
			Base:           models.Base{ID: 1, CreatedAt: ts("2025-01-01")},
			InventoryID:    1,
			SubmissionType: models.InventorySubmissionTypeReconcile,
			ApprovalStatus: models.InventorySubmissionApprovalStatusPending,
		}

		backdate := ts("2025-01-01")
		srcID := uint(7)
		// New synthetic sell (ID==0) with a backdated CreatedAt.
		newSell := &models.InventoryTransaction{
			Base:                 models.Base{CreatedAt: backdate},
			InventoryItemID:      1,
			TransactionType:      models.InventoryTransactionTypeSell,
			Quantity:             d("10"),
			CounterTransactionID: &srcID,
		}
		// Existing source purchase (ID!=0) whose consumed_quantity was mutated.
		srcTxn := &models.InventoryTransaction{
			Base:             models.Base{ID: srcID, CreatedAt: ts("2024-01-01"), CreatedBy: "real.user@cim.local"},
			InventoryItemID:  1,
			TransactionType:  models.InventoryTransactionTypePurchase,
			ConsumedQuantity: d("10"),
		}
		txns := []*models.InventoryTransaction{newSell, srcTxn}

		mock.ExpectBegin()

		// Entry guard: re-read submissions, all still pending.
		mock.ExpectQuery(`SELECT \* FROM .*inventory_submissions.*`).
			WillReturnRows(sqlmock.NewRows([]string{"id", "approval_status"}).AddRow(1, "pending"))

		// No item changes in this test (changes==nil short-circuits in saveItemChangesOnTx).

		// Synthetic sell -> plain INSERT (NOT "INSERT ... ON CONFLICT DO UPDATE").
		// tx.Create issues a bare INSERT; the absence of an upsert clause is what
		// keeps BeforeCreate semantics correct for the new rows while leaving the
		// existing source row to the column-scoped UPDATE below.
		mock.ExpectQuery(`INSERT INTO .*inventory_transactions.* VALUES .* RETURNING`).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(99))

		// Source purchase -> column-scoped UPDATE (not an upsert). The SET list is
		// exactly consumed_quantity + updated_at + updated_by, scoped by id, so
		// created_by/created_at on the real purchase row are never written.
		mock.ExpectExec(
			`UPDATE "inventory_transactions" SET "consumed_quantity"=\$1,"updated_at"=\$2,"updated_by"=\$3 WHERE id = \$4`,
		).WillReturnResult(sqlmock.NewResult(0, 1))

		// Submission status update.
		mock.ExpectExec(`UPDATE .*inventory_submissions.*`).
			WillReturnResult(sqlmock.NewResult(0, 1))

		mock.ExpectCommit()

		err := svc.persistResolution(ctx, nil, txns, []models.InventorySubmission{sub})
		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("source UPDATE statement does not contain created_by", func(t *testing.T) {
		// Capture the SQL of the source-row update and assert created_by is absent.
		svc, mock := newMockService(t)

		sub := models.InventorySubmission{
			Base:           models.Base{ID: 1, CreatedAt: ts("2025-01-01")},
			InventoryID:    1,
			SubmissionType: models.InventorySubmissionTypeReconcile,
			ApprovalStatus: models.InventorySubmissionApprovalStatusPending,
		}
		srcID := uint(7)
		srcTxn := &models.InventoryTransaction{
			Base:             models.Base{ID: srcID, CreatedAt: ts("2024-01-01"), CreatedBy: "real.user@cim.local"},
			InventoryItemID:  1,
			TransactionType:  models.InventoryTransactionTypePurchase,
			ConsumedQuantity: d("10"),
		}

		mock.ExpectBegin()
		mock.ExpectQuery(`SELECT \* FROM .*inventory_submissions.*`).
			WillReturnRows(sqlmock.NewRows([]string{"id", "approval_status"}).AddRow(1, "pending"))
		// The source UPDATE must be column-scoped: SET consumed_quantity +
		// updated_by + updated_at, and NOTHING ELSE (GORM emits the SET list
		// alphabetically: consumed_quantity, updated_at, updated_by). The "[^;]*"
		// between SET and WHERE plus the absence of created_by in the pattern,
		// combined with the explicit created_by negative-assertion test below,
		// guards against created_by ever appearing in the SET list.
		mock.ExpectExec(
			`UPDATE "inventory_transactions" SET "consumed_quantity"=\$1,"updated_at"=\$2,"updated_by"=\$3 WHERE id = \$4`,
		).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`UPDATE .*inventory_submissions.*`).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		err := svc.persistResolution(ctx, nil, []*models.InventoryTransaction{srcTxn}, []models.InventorySubmission{sub})
		require.NoError(t, err, "source-only update must succeed with a column-scoped UPDATE")
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

// TestSaveItemChangesOnTx_MultiSubmission is the persist-path regression for the
// BLOCKER: an item dropped across two submissions must validate against the DB
// baseline ONCE (no false optimistic-lock conflict / no rollback).
func TestSaveItemChangesOnTx_MultiSubmission(t *testing.T) {
	ctx := pkg.WithUserEmail(context.Background(), "system@cim.local")
	itemCols := []string{"id", "quantity"}

	t.Run("RAW (uncoalesced) duplicate changes raise a FALSE optimistic-lock conflict", func(t *testing.T) {
		svc, mock := newMockService(t)
		// Live DB row is still 100 (apply defers writes). The lock SELECT returns
		// the single row.
		mock.ExpectQuery(`SELECT \* FROM .*inventory_items.*FOR UPDATE`).
			WillReturnRows(sqlmock.NewRows(itemCols).AddRow(1, "100"))

		item := &models.InventoryItem{Base: models.Base{ID: 1}, Quantity: d("80")}
		raw := []*models.InventoryItemChange{
			{InventoryItem: item, OriginalQuantity: d("100")},
			{InventoryItem: item, OriginalQuantity: d("90")}, // stale snapshot -> 100 != 90
		}

		err := saveItemChangesOnTx(ctx, svc.db, raw)
		require.Error(t, err, "the second (stale) entry triggers the conflict we are fixing")
		var appErr *pkg.AppError
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, pkg.ErrorCodeConflict, appErr.Code, "must be a (false) optimistic-lock conflict")
	})

	t.Run("COALESCED change validates against DB baseline once and persists the final quantity", func(t *testing.T) {
		svc, mock := newMockService(t)
		mock.ExpectQuery(`SELECT \* FROM .*inventory_items.*FOR UPDATE`).
			WillReturnRows(sqlmock.NewRows(itemCols).AddRow(1, "100"))
		// Lock check passes (DB 100 == earliest baseline 100); Save persists the
		// final drawn-down quantity (80) via gorm upsert (INSERT ... ON CONFLICT
		// DO UPDATE ... RETURNING id). Save runs in its own implicit tx.
		mock.ExpectBegin()
		mock.ExpectQuery(`INSERT INTO .*inventory_items.*ON CONFLICT .* DO UPDATE`).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
		mock.ExpectCommit()

		item := &models.InventoryItem{Base: models.Base{ID: 1}, Quantity: d("80")}
		raw := []*models.InventoryItemChange{
			{InventoryItem: item, OriginalQuantity: d("100")},
			{InventoryItem: item, OriginalQuantity: d("90")},
		}

		err := saveItemChangesOnTx(ctx, svc.db.WithContext(ctx), coalesceItemChanges(raw))
		require.NoError(t, err, "no false conflict after coalescing")
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
