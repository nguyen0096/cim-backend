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

	"github.com/shopspring/decimal"
)

// InventoryInOutExportService orchestrates the inventory in/out Excel export:
//  1. fetch inventory + period & historical txns + PO selling-price info
//  2. validate every in-scope PO has a selling price
//  3. shape rows & render xlsx
//  4. upload to S3 & return a presigned URL
//
//go:generate mockery --name=InventoryInOutExportService --structname=InventoryInOutExportService --output=../mocks/servicemocks --outpkg=servicemocks
type InventoryInOutExportService interface {
	Export(ctx context.Context, req dto.InventoryInOutExportRequest) (*dto.InventoryInOutExportResponse, error)
}

type inventoryInOutExportService struct {
	inventoryRepo    repository.InventoryRepository
	sellingPriceRepo repository.SellingPriceRepository
	s3Client         S3Client
	exportPrefix     string
}

// NewInventoryInOutExportService constructs the service. exportPrefix is the
// config-driven base object-key prefix (e.g. "exports") for generated files.
func NewInventoryInOutExportService(
	inventoryRepo repository.InventoryRepository,
	sellingPriceRepo repository.SellingPriceRepository,
	s3Client S3Client,
	exportPrefix string,
) InventoryInOutExportService {
	return &inventoryInOutExportService{
		inventoryRepo:    inventoryRepo,
		sellingPriceRepo: sellingPriceRepo,
		s3Client:         s3Client,
		exportPrefix:     exportPrefix,
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

	// Historical (pre-window) and period txns, both with the counter join so
	// the shaper can resolve source POIs (purchase: own; consume: counter) and
	// route found-stock (reconcile_stock_up + its consumes) to adjustment rows.
	historicalTxns, err := s.inventoryRepo.GetTransactionsByInventoryIDsWithCounter(ctx, req.InventoryID, nil, &startDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get historical transactions: %w", err)
	}
	periodTxns, err := s.inventoryRepo.GetTransactionsByInventoryIDsWithCounter(ctx, req.InventoryID, &startDate, &endExclusive)
	if err != nil {
		return nil, fmt.Errorf("failed to get period transactions: %w", err)
	}

	// Collect all POI IDs we need metadata for: every txn (period + historical)
	// that touches a POI, regardless of date predicate on the source PO.
	poItemIDs := collectPOItemIDs(historicalTxns, periodTxns)

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

	// Precondition: every in-scope PO (one that will appear as a row) must have
	// a selling price — unless the caller opts to ignore it, in which case the
	// export proceeds and uncomputable values render as "-".
	if !req.IgnoreMissingSellingPrice {
		if missing := s.checkMissingSellingPrices(items, historicalTxns, periodTxns, poInfo); len(missing) > 0 {
			return nil, newMissingSellingPriceError(missing)
		}
	}

	// Shape & render.
	exportRows := excelpkg.BuildExportRows(excelpkg.ShaperInput{
		StartDate:      startDate,
		EndDate:        endDate,
		InventoryID:    req.InventoryID,
		Items:          items,
		HistoricalTxns: historicalTxns,
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

	// Upload + presign. Mirrors PopulateExportURL pattern. The object key is
	// server-derived & per-inventory segregated (see BuildExportObjectKey).
	fileKey := BuildExportObjectKey(
		s.exportPrefix,
		req.InventoryID,
		generatedAt,
		inventory.Name,
		startDate.Format("20060102"),
		endDate.Format("20060102"),
	)
	contentType := "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	if err := s.s3Client.UploadFile(ctx, fileKey, xlsxBytes, contentType); err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}
	// Pass the meaningful filename so the download is named e.g.
	// inventory-<name>-<ts>-<from>-<to>.xlsx instead of the UUID storage key.
	url, err := s.s3Client.GeneratePresignedURL(ctx, fileKey, 15*time.Minute, filename)
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

func collectPOItemIDs(
	historical []*repository.InventoryTransactionWithCounter,
	period []*repository.InventoryTransactionWithCounter,
) []uint {
	seen := make(map[uint]struct{})
	add := func(txns []*repository.InventoryTransactionWithCounter) {
		for _, t := range txns {
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
	}
	add(historical)
	add(period)
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
	historical []*repository.InventoryTransactionWithCounter,
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
		"code":                   e.Code.String(),
		"message":                e.Message,
		"missing_selling_prices": e.MissingPOs,
	}
	return json.Marshal(obj)
}

// Unwrap allows errors.As to find the embedded *pkg.AppError so callers
// using `errors.As(err, &appErr)` keep working with the structured shape.
func (e *missingSellingPriceError) Unwrap() error { return e.AppError }
