package services

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/internal/services/dto"
	excelpkg "cim-backend/internal/services/excel"
	"cim-backend/pkg"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// InventoryInOutExportService orchestrates the inventory in/out Excel export:
//   1. fetch inventory + period & historical txns + PO selling-price info
//   2. validate every in-scope PO has a selling price
//   3. shape rows & render xlsx
//   4. upload to S3 & return a presigned URL
//
//go:generate mockery --name=InventoryInOutExportService --structname=InventoryInOutExportService --output=../mocks/servicemocks --outpkg=servicemocks
type InventoryInOutExportService interface {
	Export(ctx context.Context, req dto.InventoryInOutExportRequest) (*dto.InventoryInOutExportResponse, error)
}

type inventoryInOutExportService struct {
	inventoryRepo    repository.InventoryRepository
	sellingPriceRepo repository.SellingPriceRepository
	s3Client         S3Client
}

// NewInventoryInOutExportService constructs the service.
func NewInventoryInOutExportService(
	inventoryRepo repository.InventoryRepository,
	sellingPriceRepo repository.SellingPriceRepository,
	s3Client S3Client,
) InventoryInOutExportService {
	return &inventoryInOutExportService{
		inventoryRepo:    inventoryRepo,
		sellingPriceRepo: sellingPriceRepo,
		s3Client:         s3Client,
	}
}

func (s *inventoryInOutExportService) Export(ctx context.Context, req dto.InventoryInOutExportRequest) (*dto.InventoryInOutExportResponse, error) {
	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		return nil, pkg.ErrValidation("invalid start_date format, expected YYYY-MM-DD", err)
	}
	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		return nil, pkg.ErrValidation("invalid end_date format, expected YYYY-MM-DD", err)
	}
	if endDate.Before(startDate) {
		return nil, pkg.ErrValidation("end_date must be on or after start_date", nil)
	}
	endExclusive := endDate.AddDate(0, 0, 1)

	inventory, err := s.inventoryRepo.GetByID(ctx, req.InventoryID)
	if err != nil {
		return nil, fmt.Errorf("failed to get inventory: %w", err)
	}

	items := buildItemInfos(inventory)

	historicalTxns, err := s.inventoryRepo.GetTransactionsByInventoryIDs(ctx, req.InventoryID, nil, &startDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get historical transactions: %w", err)
	}
	periodTxns, err := s.inventoryRepo.GetTransactionsByInventoryIDsWithCounter(ctx, req.InventoryID, &startDate, &endExclusive)
	if err != nil {
		return nil, fmt.Errorf("failed to get period transactions: %w", err)
	}

	// Resolve historical consume txns to their source POI via the period
	// algorithm: walk all consume txns to surface CounterPOIID. For
	// historical (pre-window) consume txns we need an extra fetch — query
	// txns-with-counter for the open-ended range up to startDate.
	historicalConsumeWithCounter, err := s.inventoryRepo.GetTransactionsByInventoryIDsWithCounter(ctx, req.InventoryID, nil, &startDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get historical transactions with counter: %w", err)
	}

	// Build the canonical historical txn list with PurchaseOrderItemID set
	// to the resolved source POI for every txn (purchase: own; consume:
	// counter). The shaper consumes this directly.
	resolvedHistorical := resolveHistoricalPOIs(historicalTxns, historicalConsumeWithCounter)

	// Collect all POI IDs we need metadata for: every txn (period + historical)
	// that touches a POI, regardless of date predicate on the source PO.
	poItemIDs := collectPOItemIDs(resolvedHistorical, periodTxns)

	poInfo, err := s.sellingPriceRepo.GetPOItemsWithPriceByIDs(ctx, poItemIDs, req.InventoryID)
	if err != nil {
		return nil, fmt.Errorf("failed to get PO item info: %w", err)
	}

	// Fill in cross-inventory source POIs (e.g. transfer-in source POIs that
	// live in another inventory). These are filtered out by the inventory
	// scope above, but we still need their metadata to build the destination
	// inventory's row, since transfer-in rows inherit the source POI's
	// purchase / selling price.
	var missingPOIIDs []uint
	for _, id := range poItemIDs {
		if _, ok := poInfo[id]; !ok {
			missingPOIIDs = append(missingPOIIDs, id)
		}
	}
	if len(missingPOIIDs) > 0 {
		extra, err := s.sellingPriceRepo.GetPOItemsWithPriceByIDsAcrossInventories(ctx, missingPOIIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to get cross-inventory PO item info: %w", err)
		}
		for id, info := range extra {
			poInfo[id] = info
		}
	}

	// Precondition: every in-scope PO must have a selling price. Determine
	// in-scope POIs: those that will appear as a row (i.e. have window
	// activity OR positive ending stock at any point in the window).
	if missing := s.checkMissingSellingPrices(items, resolvedHistorical, periodTxns, poInfo); len(missing) > 0 {
		return nil, newMissingSellingPriceError(missing)
	}

	// Shape & render.
	exportRows := excelpkg.BuildExportRows(excelpkg.ShaperInput{
		StartDate:      startDate,
		EndDate:        endDate,
		InventoryID:    req.InventoryID,
		Items:          items,
		HistoricalTxns: resolvedHistorical,
		PeriodTxns:     periodTxns,
		POInfo:         poInfo,
	})

	generatedBy, _ := pkg.GetUserEmailFromContext(ctx)
	generatedAt := time.Now()
	xlsxBytes, err := excelpkg.WriteInOutExportToBuffer(exportRows, excelpkg.ExportContext{
		InventoryName: inventory.Name,
		GeneratedAt:   generatedAt,
		GeneratedBy:   generatedBy,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to render xlsx: %w", err)
	}

	filename := fmt.Sprintf("inventory-%s-%s-%s-%s.xlsx",
		excelpkg.SanitizeFilenameSegment(inventory.Name),
		generatedAt.Format("20060102_150405"),
		startDate.Format("20060102"),
		endDate.Format("20060102"),
	)

	// Upload + presign. Mirrors PopulateExportURL pattern.
	fileKey := fmt.Sprintf("exports/%d/%02d/%02d/%s.xlsx",
		generatedAt.Year(), generatedAt.Month(), generatedAt.Day(), uuid.New().String())
	contentType := "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	if err := s.s3Client.UploadFile(ctx, fileKey, xlsxBytes, contentType); err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}
	url, err := s.s3Client.GeneratePresignedURL(ctx, fileKey, 15*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return &dto.InventoryInOutExportResponse{
		DownloadURL: url,
		Filename:    filename,
	}, nil
}

// --- helpers --------------------------------------------------------------

func buildItemInfos(inventory *models.Inventory) []*excelpkg.ItemInfo {
	if inventory == nil {
		return nil
	}
	items := make([]*excelpkg.ItemInfo, 0, len(inventory.Items))
	for _, it := range inventory.Items {
		if it == nil || it.Product == nil {
			continue
		}
		unitName := ""
		if it.Unit != nil {
			unitName = it.Unit.Name
		}
		items = append(items, &excelpkg.ItemInfo{
			ItemID:      it.ID,
			ProductID:   it.Product.ID,
			ProductName: it.Product.Name,
			UnitName:    unitName,
		})
	}
	return items
}

// resolveHistoricalPOIs returns a list of historical txns where every txn's
// PurchaseOrderItemID points at the resolved source POI:
//   - for purchases / transfer-ins: their own PurchaseOrderItemID is unchanged
//   - for consumes (sell/disposal/transfer-out): copy the counter POI from
//     the with-counter query.
func resolveHistoricalPOIs(
	plain []*models.InventoryTransaction,
	withCounter []*repository.InventoryTransactionWithCounter,
) []*models.InventoryTransaction {
	counterByID := make(map[uint]*uint, len(withCounter))
	for _, w := range withCounter {
		if w == nil || w.InventoryTransaction == nil {
			continue
		}
		counterByID[w.ID] = w.CounterPOIID
	}

	out := make([]*models.InventoryTransaction, 0, len(plain))
	for _, t := range plain {
		if t == nil {
			continue
		}
		clone := *t
		switch t.TransactionType {
		case models.InventoryTransactionTypeSell,
			models.InventoryTransactionTypeDisposal,
			models.InventoryTransactionTypeTransferOut,
			models.InventoryTransactionTypeTransferIn:
			// Override with counter POI if available — for consumes this
			// is the resolved source; for transfer-in it's the source PO
			// in the originating inventory.
			if counterPOI, ok := counterByID[t.ID]; ok && counterPOI != nil {
				p := *counterPOI
				clone.PurchaseOrderItemID = &p
			}
		}
		out = append(out, &clone)
	}
	return out
}

func collectPOItemIDs(
	historical []*models.InventoryTransaction,
	period []*repository.InventoryTransactionWithCounter,
) []uint {
	seen := make(map[uint]struct{})
	for _, t := range historical {
		if t.PurchaseOrderItemID != nil && *t.PurchaseOrderItemID > 0 {
			seen[*t.PurchaseOrderItemID] = struct{}{}
		}
	}
	for _, t := range period {
		if t == nil || t.InventoryTransaction == nil {
			continue
		}
		if t.PurchaseOrderItemID != nil && *t.PurchaseOrderItemID > 0 {
			seen[*t.PurchaseOrderItemID] = struct{}{}
		}
		if t.CounterPOIID != nil && *t.CounterPOIID > 0 {
			seen[*t.CounterPOIID] = struct{}{}
		}
	}
	out := make([]uint, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// checkMissingSellingPrices identifies POs in scope whose POI lacks a selling
// price. "In scope" = a POI that would appear as an export row.
func (s *inventoryInOutExportService) checkMissingSellingPrices(
	items []*excelpkg.ItemInfo,
	historical []*models.InventoryTransaction,
	period []*repository.InventoryTransactionWithCounter,
	poInfo map[uint]*repository.POItemSellingPriceInfo,
) []dto.MissingSellingPricePO {
	itemIDs := make(map[uint]struct{}, len(items))
	for _, it := range items {
		itemIDs[it.ItemID] = struct{}{}
	}

	// Compute, per POI, an "in-scope" indicator following the inclusion rule
	// from the design comment: include if window-activity OR remaining stock
	// at any point in window. Stock math runs on decimal.Decimal (NOT float64)
	// so the precondition matches BuildExportRows' Decimal-based inclusion
	// check exactly — float rounding around fractional quantities can otherwise
	// leave tiny non-zero residues (or erase small values) and disagree with
	// the shaper around zero-balance POIs.
	type poiState struct {
		hasActivity bool
		begin       decimal.Decimal
		windowIn    decimal.Decimal
	}
	state := make(map[uint]*poiState)
	get := func(id uint) *poiState {
		if s, ok := state[id]; ok {
			return s
		}
		s := &poiState{}
		state[id] = s
		return s
	}

	for _, t := range historical {
		if t.PurchaseOrderItemID == nil {
			continue
		}
		if _, ok := itemIDs[t.InventoryItemID]; !ok {
			continue
		}
		st := get(*t.PurchaseOrderItemID)
		st.begin = st.begin.Add(t.TransactionType.StockDelta(t.Quantity))
	}
	for _, t := range period {
		if t == nil || t.InventoryTransaction == nil {
			continue
		}
		if _, ok := itemIDs[t.InventoryItemID]; !ok {
			continue
		}
		var poi uint
		switch t.TransactionType {
		case models.InventoryTransactionTypePurchase:
			if t.PurchaseOrderItemID != nil {
				poi = *t.PurchaseOrderItemID
			}
		default:
			if t.CounterPOIID != nil {
				poi = *t.CounterPOIID
			}
		}
		if poi == 0 {
			continue
		}
		st := get(poi)
		st.hasActivity = true
		switch t.TransactionType {
		case models.InventoryTransactionTypePurchase, models.InventoryTransactionTypeTransferIn:
			st.windowIn = st.windowIn.Add(t.Quantity)
		}
	}

	missingMap := make(map[uint]dto.MissingSellingPricePO) // dedupe by PO id
	for poi, st := range state {
		// Match BuildExportRows: a non-zero (positive or negative) carry-over
		// counts as in-scope. Pair with decimal.IsZero so we don't drop rows
		// whose begin is a fractional non-zero residue.
		hasCarryOver := !st.begin.IsZero()
		if !st.hasActivity && !hasCarryOver {
			continue
		}
		info, ok := poInfo[poi]
		if !ok {
			continue
		}
		if info.EffectivePrice == nil {
			missingMap[info.POID] = dto.MissingSellingPricePO{POID: info.POID, PONumber: info.PONumber}
		}
	}
	if len(missingMap) == 0 {
		return nil
	}
	out := make([]dto.MissingSellingPricePO, 0, len(missingMap))
	for _, m := range missingMap {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PONumber < out[j].PONumber })
	return out
}

// missingSellingPriceError is a structured error surfaced via the AppError
// JSON shape with a "missing_selling_prices" field.
type missingSellingPriceError struct {
	*pkg.AppError
	MissingPOs []dto.MissingSellingPricePO `json:"missing_selling_prices"`
}

func newMissingSellingPriceError(missing []dto.MissingSellingPricePO) *missingSellingPriceError {
	return &missingSellingPriceError{
		AppError: pkg.NewAppError(
			pkg.ErrorCodeValidation,
			"Một số đơn nhập chưa có giá bán; vui lòng cập nhật trước khi xuất file",
			nil,
		),
		MissingPOs: missing,
	}
}

func (e *missingSellingPriceError) MarshalJSON() ([]byte, error) {
	obj := map[string]interface{}{
		"code":                    e.Code.String(),
		"message":                 e.Message,
		"missing_selling_prices":  e.MissingPOs,
	}
	return json.Marshal(obj)
}

// Unwrap allows errors.As to find the embedded *pkg.AppError so callers
// using `errors.As(err, &appErr)` keep working with the structured shape.
func (e *missingSellingPriceError) Unwrap() error { return e.AppError }
