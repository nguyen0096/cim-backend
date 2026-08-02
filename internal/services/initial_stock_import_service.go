package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
	"cim-backend/pkg/log"
)

const (
	// initialStockUnitType matches the unit type the product import creates.
	initialStockUnitType = "general"
	// initialStockHeaderRow is the fixed 1-based row carrying the column headers.
	initialStockHeaderRow = 3
	// initialStockMaxDecimalPlaces caps accepted quantity scale independently of the
	// unit's decimal_places, which is settable to 10 through the units API while both
	// quantity columns are DECIMAL(10,2). A wider scale would let the item and the
	// transaction round apart and trip ValidateActivePurchaseTransactions for the
	// whole batch.
	initialStockMaxDecimalPlaces = 2
	// Column widths every row is validated against, so a dry-run cannot report a row
	// as loadable that would then fail the INSERT with a bodyless 500.
	// products.product_type varchar(20), products.name varchar(255),
	// units.name varchar(100), units.symbol varchar(20).
	initialStockProductTypeMaxLen = 20
	initialStockProductNameMaxLen = 255
	initialStockUnitNameMaxLen    = 100
	initialStockUnitSymbolMaxLen  = 20
	// initialStockMaxIntegerDigits is the integer precision of the DECIMAL(10,2)
	// quantity columns: 10 total digits minus the 2 fractional ones.
	initialStockMaxIntegerDigits = 8

	// Unzip limits: the real workbook expands to ~12.7 MiB with a single 12.3 MiB
	// sheet XML, so both limits must sit above that. UnzipXMLSizeLimit must not
	// exceed UnzipSizeLimit or excelize refuses to open the file at all.
	initialStockUnzipSizeLimit    = 32 << 20
	initialStockUnzipXMLSizeLimit = 16 << 20

	initialStockSheetReasonHeaderNotFound = "header_not_found"
)

// initialStockExpectedHeader is the required row-3 header, compared trimmed and
// upper-cased but never diacritic-folded.
var initialStockExpectedHeader = []string{"STT", "TÊN", "ĐVT", "SỐ LƯỢNG"}

//go:generate mockery --name=InitialStockImportService --structname=InitialStockImportService --output=../mocks/servicemocks --outpkg=servicemocks
type InitialStockImportService interface {
	ListSheets(ctx context.Context, data []byte) (*dto.InitialStockSheetsResponse, error)
	ListInventories(ctx context.Context) (*dto.InitialStockInventoriesResponse, error)
	Import(ctx context.Context, req dto.InitialStockImportRequest, data []byte) (*dto.InitialStockImportResponse, error)
}

type initialStockImportService struct {
	baseRepo          repository.BaseRepository
	inventoryRepo     repository.InventoryRepository
	inventoryItemRepo repository.InventoryItemRepository
	submissionRepo    repository.InventorySubmissionRepository
	productRepo       repository.ProductRepository
	unitRepo          repository.UnitRepository
	importRepo        repository.InitialStockImportRepository
	unitService       UnitService
	productService    ProductService
}

func NewInitialStockImportService(
	baseRepo repository.BaseRepository,
	inventoryRepo repository.InventoryRepository,
	inventoryItemRepo repository.InventoryItemRepository,
	submissionRepo repository.InventorySubmissionRepository,
	productRepo repository.ProductRepository,
	unitRepo repository.UnitRepository,
	importRepo repository.InitialStockImportRepository,
	unitService UnitService,
	productService ProductService,
) InitialStockImportService {
	return &initialStockImportService{
		baseRepo:          baseRepo,
		inventoryRepo:     inventoryRepo,
		inventoryItemRepo: inventoryItemRepo,
		submissionRepo:    submissionRepo,
		productRepo:       productRepo,
		unitRepo:          unitRepo,
		importRepo:        importRepo,
		unitService:       unitService,
		productService:    productService,
	}
}

// sheetRow is one raw data row of the source sheet.
type sheetRow struct {
	SheetRow    int
	Name        string
	Unit        string
	RawQuantity string
	ProductType string
}

// ListInventories returns the tool's own picker list. An empty list is a success.
func (s *initialStockImportService) ListInventories(ctx context.Context) (*dto.InitialStockInventoriesResponse, error) {
	inventories, err := s.inventoryRepo.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	options := make([]dto.InitialStockInventoryOption, 0, len(inventories))
	for i := range inventories {
		options = append(options, dto.InitialStockInventoryOption{
			ID:       inventories[i].ID,
			Name:     inventories[i].Name,
			Location: inventories[i].Location,
			Status:   string(inventories[i].Status),
		})
	}
	return &dto.InitialStockInventoriesResponse{Data: options}, nil
}

// ListSheets reports every worksheet with its header verdict and data row count so
// the frontend never needs its own xlsx parser.
func (s *initialStockImportService) ListSheets(ctx context.Context, data []byte) (*dto.InitialStockSheetsResponse, error) {
	f, err := openInitialStockWorkbook(data)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	names := f.GetSheetList()
	infos := make([]dto.InitialStockSheetInfo, 0, len(names))
	for _, name := range names {
		info := dto.InitialStockSheetInfo{Name: name}
		rows, parseErr := parseInitialStockSheet(f, name)
		if parseErr != nil {
			if !errors.Is(parseErr, errInitialStockHeaderMismatch) {
				return nil, parseErr
			}
			info.Reason = initialStockSheetReasonHeaderNotFound
		} else {
			info.HasExpectedHeader = true
			info.DataRowCount = len(rows)
		}
		infos = append(infos, info)
	}
	return &dto.InitialStockSheetsResponse{Sheets: infos}, nil
}

// Import previews or applies the load. Both modes return the same response shape;
// only an apply opens a transaction or writes anything.
func (s *initialStockImportService) Import(
	ctx context.Context,
	req dto.InitialStockImportRequest,
	data []byte,
) (*dto.InitialStockImportResponse, error) {
	// Deliberately no inventory-state check here: it describes CURRENT state, and a
	// committed key must replay its stored result even if the inventory has since been
	// deactivated. Both modes validate it below, after the receipt lookup.
	f, err := openInitialStockWorkbook(data)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if idx, sheetErr := f.GetSheetIndex(req.SheetName); sheetErr != nil || idx < 0 {
		return nil, pkg.ErrInitialStock(pkg.ErrorCodeValidation, pkg.ErrKeyInitialStockSheetNotFound, req.SheetName)
	}

	rows, err := parseInitialStockSheet(f, req.SheetName)
	if err != nil {
		if errors.Is(err, errInitialStockHeaderMismatch) {
			return nil, pkg.ErrInitialStock(pkg.ErrorCodeValidation, pkg.ErrKeyInitialStockHeaderMismatch, req.SheetName)
		}
		return nil, err
	}
	if len(rows) == 0 {
		return nil, pkg.ErrInitialStock(pkg.ErrorCodeValidation, pkg.ErrKeyInitialStockNoDataRows, req.SheetName)
	}

	if req.DryRun {
		return s.dryRun(ctx, req, rows)
	}
	return s.apply(ctx, req, rows)
}

func (s *initialStockImportService) dryRun(
	ctx context.Context,
	req dto.InitialStockImportRequest,
	rows []sheetRow,
) (*dto.InitialStockImportResponse, error) {
	if err := s.requireActiveInventory(ctx, req.InventoryID); err != nil {
		return nil, err
	}

	blocking, err := s.blockingConditions(ctx, req.InventoryID)
	if err != nil {
		return nil, err
	}

	plan, rowErrors, err := s.buildPlan(ctx, req.InventoryID, rows)
	if err != nil {
		return nil, err
	}

	resp := plan.response(req, blocking)
	resp.Errors = rowErrors
	resp.RowsFailed = len(rowErrors)
	resp.RowsOK = len(rows) - len(rowErrors)
	return resp, nil
}

func (s *initialStockImportService) apply(
	ctx context.Context,
	req dto.InitialStockImportRequest,
	rows []sheetRow,
) (*dto.InitialStockImportResponse, error) {
	var (
		resp         *dto.InitialStockImportResponse
		productTypes []string
	)

	txErr := s.baseRepo.WithinTx(ctx, func(txCtx context.Context) error {
		// Lock first: every other inventory write takes the same advisory lock, so
		// the guards below read a state no concurrent inventory write can change
		// under us. Under READ COMMITTED (the repo sets no isolation override) the
		// post-lock reads see a competing run's committed rows; promoting this tx to
		// REPEATABLE READ would silently break that.
		if err := s.submissionRepo.AcquireInventoryAdvisoryLock(txCtx, req.InventoryID); err != nil {
			return fmt.Errorf("failed to acquire inventory advisory lock: %w", err)
		}

		// Replay before any new-work guard: a committed key writes nothing, so the
		// guards below have nothing to protect against here. Checking them first would
		// turn a request that already succeeded but lost its response into an apparent
		// failure the client cannot distinguish from "not applied".
		if req.IdempotencyKey != "" {
			receipt, err := s.importRepo.GetReceipt(txCtx, req.InventoryID, req.IdempotencyKey)
			if err != nil {
				return err
			}
			if receipt != nil {
				replayed, err := s.replayReceipt(txCtx, receipt, req)
				if err != nil {
					return err
				}
				resp = replayed
				return nil
			}
		}

		// Current-state guards start here: everything above is either an input to the
		// receipt lookup or the lookup itself, so a committed key never reaches them.
		if err := s.requireActiveInventory(txCtx, req.InventoryID); err != nil {
			return err
		}

		// A reconcile snapshot captures on-hand at initiate and applies its surplus
		// against current on-hand, and drift detection only scans submission rows.
		// An import inside that window is invisible to both, landing on-hand at B+2X.
		pending, err := s.submissionRepo.ExistsActivePending(txCtx, req.InventoryID)
		if err != nil {
			return fmt.Errorf("failed to check pending reconcile: %w", err)
		}
		if pending {
			return pkg.ErrInitialStock(pkg.ErrorCodeConflict, pkg.ErrKeyInitialStockReconcileOpen, req.InventoryID)
		}

		// Scoped to the inventory, not the key. The frontend's key is in-memory only,
		// so browser Back, refresh or a file re-attach mints a fresh one; this guard is
		// the only thing standing between that and a second load.
		imported, err := s.importRepo.ExistsInitialForInventory(txCtx, req.InventoryID)
		if err != nil {
			return err
		}
		if imported {
			return pkg.ErrInitialStock(pkg.ErrorCodeConflict, pkg.ErrKeyInitialStockAlreadyImported, req.InventoryID)
		}

		plan, rowErrors, err := s.buildPlan(txCtx, req.InventoryID, rows)
		if err != nil {
			return err
		}
		if len(rowErrors) > 0 {
			// The 400 envelope keeps the shared BatchError shape: the sheet values ride
			// on the 200 response's errors array, which is what the preview renders.
			batch := pkg.ErrValidationBatchError()
			for _, rowErr := range rowErrors {
				batch.Locations = append(batch.Locations, pkg.NewRowLocation(rowErr.Row, rowErr.Message))
			}
			return batch
		}

		if err := s.persist(txCtx, plan); err != nil {
			return err
		}
		productTypes = plan.productTypes

		built := plan.response(req, []dto.InitialStockBlocking{})
		built.Errors = []dto.InitialStockImportError{}
		built.RowsOK = len(rows)

		payload, err := json.Marshal(built)
		if err != nil {
			return fmt.Errorf("failed to encode initial stock import result: %w", err)
		}
		createdBy, err := pkg.GetUserEmailFromContext(txCtx)
		if err != nil {
			return err
		}

		if req.IdempotencyKey != "" {
			recorded, err := s.importRepo.CreateReceipt(txCtx, &models.InitialStockImport{
				IdempotencyKey: req.IdempotencyKey,
				InventoryID:    req.InventoryID,
				SheetName:      req.SheetName,
				FileName:       req.FileName,
				FileSHA256:     req.FileSHA256,
				RowCount:       len(rows),
				ResultSummary:  payload,
				CreatedBy:      createdBy,
			})
			if err != nil {
				return err
			}
			// A concurrent run under the same (inventory, key) already recorded a
			// receipt, so this work is the duplicate: roll back and let the caller
			// retry into the replay path rather than double-loading.
			if !recorded {
				return pkg.ErrInitialStock(pkg.ErrorCodeConflict, pkg.ErrKeyInitialStockAlreadyImported, req.InventoryID)
			}
		}

		resp = built
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}

	// Runs after commit and never fails the load: product_types is a UI filter list,
	// and the existing product import treats the same write as non-fatal.
	if len(productTypes) > 0 {
		if err := s.productService.UpdateProductTypesInSettings(ctx, productTypes); err != nil {
			log.WithError(err).Warn("initial stock import: failed to update product_types setting")
		}
	}

	return resp, nil
}

// requireActiveInventory rejects a missing, soft-deleted or inactive inventory with a
// structured error rather than letting it surface as a foreign-key failure.
func (s *initialStockImportService) requireActiveInventory(ctx context.Context, inventoryID uint) error {
	inventory, err := s.inventoryRepo.GetByID(ctx, inventoryID)
	if err != nil || inventory == nil || inventory.Status != models.InventoryStatusActive {
		return pkg.ErrInitialStock(pkg.ErrorCodeNotFound, pkg.ErrKeyInitialStockInventoryNotFound, inventoryID)
	}
	return nil
}

// replayReceipt returns the stored response for a committed key, refusing when the
// key was used for a different file or sheet: a scoped index alone would otherwise
// report success for work never done against this payload.
func (s *initialStockImportService) replayReceipt(
	ctx context.Context,
	receipt *models.InitialStockImport,
	req dto.InitialStockImportRequest,
) (*dto.InitialStockImportResponse, error) {
	if receipt.FileSHA256 != req.FileSHA256 || receipt.SheetName != req.SheetName {
		return nil, pkg.ErrInitialStock(pkg.ErrorCodeConflict, pkg.ErrKeyInitialStockKeyPayloadMismatch)
	}
	var stored dto.InitialStockImportResponse
	if err := json.Unmarshal(receipt.ResultSummary, &stored); err != nil {
		return nil, fmt.Errorf("failed to decode stored initial stock import result: %w", err)
	}
	if err := s.restateLegacyReceiptRows(ctx, stored.Rows); err != nil {
		return nil, err
	}
	return &stored, nil
}

// restateLegacyReceiptRows repairs rows recorded before per-row warnings existed.
// Such a row holds the sheet's product_type — which a matched product never took —
// and no warning saying so, so replaying it verbatim would report an ignored value as
// effective. A nil Warnings slice is the marker: every row written since carries at
// least an empty array.
//
// A created product took the sheet's type, so its stored value is already the
// effective one and needs no lookup. Every other row is resolved against the product
// itself: a matched product's type is never rewritten by this tool, so reading it back
// yields what a fresh run would report. Rows whose actions identify neither case take
// the same path rather than being assumed created, since the receipt does not say
// whether the stored value was applied.
func (s *initialStockImportService) restateLegacyReceiptRows(ctx context.Context, rows []dto.InitialStockImportRow) error {
	unresolved := make([]int, 0, len(rows))
	ids := make([]uint, 0, len(rows))
	for i := range rows {
		if rows[i].Warnings != nil {
			continue
		}
		rows[i].Warnings = []string{}
		if slices.Contains(rows[i].Actions, dto.InitialStockActionCreateProduct) {
			continue
		}
		unresolved = append(unresolved, i)
		if rows[i].ProductID != 0 {
			ids = append(ids, rows[i].ProductID)
		}
	}
	if len(unresolved) == 0 {
		return nil
	}

	// Unscoped, so a soft-deleted product is still read and still answers for its type.
	found, err := s.productRepo.GetByIDsUnscoped(ctx, ids)
	if err != nil {
		return err
	}
	byID := make(map[uint]*models.Product, len(found))
	for i := range found {
		byID[found[i].ID] = &found[i]
	}

	for _, i := range unresolved {
		sheetType := rows[i].ProductType
		product, ok := byID[rows[i].ProductID]
		if !ok {
			rows[i].ProductType = ""
			rows[i].Warnings = append(rows[i].Warnings, unreadableProductTypeWarning(sheetType))
			continue
		}
		rows[i].ProductType = product.ProductType
		rows[i].Warnings = appendProductTypeWarning(rows[i].Warnings, sheetType, product.ProductType)
	}
	return nil
}

// unreadableProductTypeWarning states that no effective type can be shown, naming the
// sheet's value only when there was one. Never conditional on that value: an empty
// product_type with no warning is indistinguishable from a product that genuinely has
// no type, and a blank sheet cell says nothing about whether the product could be read.
func unreadableProductTypeWarning(sheetType string) string {
	if sheetType == "" {
		return pkg.RowMessage(pkg.WarnKeyInitialStockRowProductTypeUnreadable)
	}
	return pkg.RowMessage(pkg.WarnKeyInitialStockRowProductTypeIgnoredUnreadable, sheetType)
}

// blockingConditions reports the non-row conditions that would refuse an apply, so
// a dry-run surfaces them instead of erroring.
func (s *initialStockImportService) blockingConditions(ctx context.Context, inventoryID uint) ([]dto.InitialStockBlocking, error) {
	blocking := make([]dto.InitialStockBlocking, 0, 2)

	pending, err := s.submissionRepo.ExistsActivePending(ctx, inventoryID)
	if err != nil {
		return nil, fmt.Errorf("failed to check pending reconcile: %w", err)
	}
	if pending {
		blocking = append(blocking, dto.InitialStockBlocking{
			Key:     pkg.ErrKeyInitialStockReconcileOpen,
			Message: pkg.RowMessage(pkg.ErrKeyInitialStockReconcileOpen, inventoryID),
		})
	}

	imported, err := s.importRepo.ExistsInitialForInventory(ctx, inventoryID)
	if err != nil {
		return nil, err
	}
	if imported {
		blocking = append(blocking, dto.InitialStockBlocking{
			Key:     pkg.ErrKeyInitialStockAlreadyImported,
			Message: pkg.RowMessage(pkg.ErrKeyInitialStockAlreadyImported, inventoryID),
		})
	}

	return blocking, nil
}
