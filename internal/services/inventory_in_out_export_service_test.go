package services

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
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

// fakeS3 is a tiny in-test S3Client. Service tests can't import
// servicemocks (it would create an import cycle through services).
type fakeS3 struct {
	UploadCalled    bool
	PresignCalled   bool
	UploadErr       error
	URL             string
	PresignFilename string // captures the download filename passed for Content-Disposition
	UploadedKey     string // captures the object key passed to UploadFile
	PresignedKey    string // captures the object key passed to GeneratePresignedURL
}

func (f *fakeS3) UploadFile(_ context.Context, key string, _ []byte, _ string) error {
	f.UploadCalled = true
	f.UploadedKey = key
	return f.UploadErr
}
func (f *fakeS3) GeneratePresignedURL(_ context.Context, key string, _ time.Duration, downloadFilename ...string) (string, error) {
	f.PresignCalled = true
	f.PresignedKey = key
	if len(downloadFilename) > 0 {
		f.PresignFilename = downloadFilename[0]
	}
	if f.URL == "" {
		return "https://signed.example/file.xlsx", nil
	}
	return f.URL, nil
}

// testExportPrefix is the configured base prefix used across these tests.
const testExportPrefix = "exports"

const (
	ioInvID    = uint(10)
	ioProdID   = uint(1)
	ioItemID   = uint(100)
	ioUnitID   = uint(5)
	ioPOItemID = uint(200)
	ioPOID     = uint(50)
)

func ioInventory() *models.Inventory {
	return &models.Inventory{
		Base: models.Base{ID: ioInvID},
		Name: "Kho A",
		Items: []*models.InventoryItem{
			{
				Base:        models.Base{ID: ioItemID},
				InventoryID: ioInvID,
				ProductID:   ioProdID,
				Product:     &models.Product{Base: models.Base{ID: ioProdID}, Name: "Widget"},
				UnitID:      ioUnitID,
				Unit:        &models.Unit{Base: models.Base{ID: ioUnitID}, Name: "kg"},
			},
		},
	}
}

func ioReq() dto.InventoryInOutExportRequest {
	return dto.InventoryInOutExportRequest{
		InventoryID: ioInvID,
		StartDate:   "2026-04-01",
		EndDate:     "2026-04-30",
	}
}

func TestExport_HappyPath_UploadsAndReturnsURL(t *testing.T) {
	ctx := context.Background()
	invRepo := repositorymocks.NewInventoryRepository(t)
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	s3 := &fakeS3{}

	svc := NewInventoryInOutExportService(invRepo, spRepo, s3, testExportPrefix)

	poiID := uint(ioPOItemID)
	purchase := &repository.InventoryTransactionWithCounter{
		InventoryTransaction: &models.InventoryTransaction{
			Base:                models.Base{ID: 1, CreatedAt: time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)},
			InventoryItemID:     ioItemID,
			TransactionType:     models.InventoryTransactionTypePurchase,
			Quantity:            decimal.NewFromInt(10),
			Price:               5.0,
			PurchaseOrderItemID: &poiID,
		},
	}
	price := decimal.NewFromInt(8)
	poInfo := map[uint]*repository.POItemSellingPriceInfo{
		ioPOItemID: {
			POItemID: ioPOItemID, POID: ioPOID, PONumber: "PO-1",
			ProductID: ioProdID, EffectivePrice: &price,
		},
	}

	invRepo.On("GetByID", ctx, ioInvID).Return(ioInventory(), nil)
	invRepo.On("GetTransactionsByInventoryIDsWithCounter", ctx, ioInvID, mock.Anything, mock.Anything).
		Return([]*repository.InventoryTransactionWithCounter{purchase}, nil)
	invRepo.On("GetTransactionsByInventoryIDsWithCounter", ctx, ioInvID, (*time.Time)(nil), mock.Anything).
		Return([]*repository.InventoryTransactionWithCounter{}, nil)
	spRepo.On("GetPOItemsWithPriceByIDs", ctx, mock.Anything, ioInvID).
		Return(poInfo, nil)
	resp, err := svc.Export(ctx, ioReq())
	require.NoError(t, err)
	assert.Equal(t, "https://signed.example/file.xlsx", resp.DownloadURL)
	assert.Contains(t, resp.Filename, "inventory-kho-a-")
	assert.Contains(t, resp.Filename, "-20260401-20260430.xlsx")
	assert.True(t, s3.UploadCalled)
	assert.True(t, s3.PresignCalled)
	// The meaningful filename is passed to the presigned URL (Content-Disposition)
	// so the download isn't named after the UUID storage key.
	assert.Equal(t, resp.Filename, s3.PresignFilename)

	// The server-derived object key is config-prefixed, per-inventory
	// segregated, carries the sanitized name + period, and is never bare-root.
	require.NotEmpty(t, s3.UploadedKey)
	assert.True(t, strings.HasPrefix(s3.UploadedKey, testExportPrefix+"/inventory/10/"),
		"key %q must be prefixed and segregated by inventory id", s3.UploadedKey)
	assert.Contains(t, s3.UploadedKey, "-kho-a-")
	assert.Contains(t, s3.UploadedKey, "-20260401-20260430-")
	assert.True(t, strings.HasSuffix(s3.UploadedKey, ".xlsx"))
	assert.NotEqual(t, "/", string(s3.UploadedKey[0]), "key must never start at bucket root")
	// Upload and presign must target the identical key.
	assert.Equal(t, s3.UploadedKey, s3.PresignedKey)
}

func TestExport_PreconditionMissingSellingPrice(t *testing.T) {
	ctx := context.Background()
	invRepo := repositorymocks.NewInventoryRepository(t)
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	s3 := &fakeS3{}

	svc := NewInventoryInOutExportService(invRepo, spRepo, s3, testExportPrefix)

	poiID := uint(ioPOItemID)
	purchase := &repository.InventoryTransactionWithCounter{
		InventoryTransaction: &models.InventoryTransaction{
			Base:                models.Base{ID: 1, CreatedAt: time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)},
			InventoryItemID:     ioItemID,
			TransactionType:     models.InventoryTransactionTypePurchase,
			Quantity:            decimal.NewFromInt(10),
			Price:               5.0,
			PurchaseOrderItemID: &poiID,
		},
	}

	// EffectivePrice is nil → precondition failure.
	poInfo := map[uint]*repository.POItemSellingPriceInfo{
		ioPOItemID: {
			POItemID: ioPOItemID, POID: ioPOID, PONumber: "PO-MISS",
			ProductID: ioProdID, EffectivePrice: nil,
		},
	}

	invRepo.On("GetByID", ctx, ioInvID).Return(ioInventory(), nil)
	invRepo.On("GetTransactionsByInventoryIDsWithCounter", ctx, ioInvID, mock.Anything, mock.Anything).
		Return([]*repository.InventoryTransactionWithCounter{purchase}, nil)
	invRepo.On("GetTransactionsByInventoryIDsWithCounter", ctx, ioInvID, (*time.Time)(nil), mock.Anything).
		Return([]*repository.InventoryTransactionWithCounter{}, nil)
	spRepo.On("GetPOItemsWithPriceByIDs", ctx, mock.Anything, ioInvID).
		Return(poInfo, nil)

	resp, err := svc.Export(ctx, ioReq())
	require.Error(t, err)
	assert.Nil(t, resp)

	var appErr *pkg.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, pkg.ErrorCodeValidation, appErr.Code)

	// JSON should include the missing PO list.
	body, mErr := json.Marshal(err)
	require.NoError(t, mErr)
	assert.Contains(t, string(body), "PO-MISS")
	assert.Contains(t, string(body), "missing_selling_prices")

	// s3 should NOT have been called.
	assert.False(t, s3.UploadCalled, "UploadFile must not be called when precondition fails")
	assert.False(t, s3.PresignCalled, "GeneratePresignedURL must not be called when precondition fails")
}

func TestExport_IgnoreMissingSellingPriceBypassesPrecondition(t *testing.T) {
	// With IgnoreMissingSellingPrice=true the missing-selling-price precondition
	// is skipped: the export proceeds (uncomputable values render as "-" in the
	// sheet) and the file is uploaded & presigned.
	ctx := context.Background()
	invRepo := repositorymocks.NewInventoryRepository(t)
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	s3 := &fakeS3{}

	svc := NewInventoryInOutExportService(invRepo, spRepo, s3, testExportPrefix)

	poiID := uint(ioPOItemID)
	purchase := &repository.InventoryTransactionWithCounter{
		InventoryTransaction: &models.InventoryTransaction{
			Base:                models.Base{ID: 1, CreatedAt: time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)},
			InventoryItemID:     ioItemID,
			TransactionType:     models.InventoryTransactionTypePurchase,
			Quantity:            decimal.NewFromInt(10),
			Price:               5.0,
			PurchaseOrderItemID: &poiID,
		},
	}

	// EffectivePrice nil — would block without Confirm.
	poInfo := map[uint]*repository.POItemSellingPriceInfo{
		ioPOItemID: {
			POItemID: ioPOItemID, POID: ioPOID, PONumber: "PO-MISS",
			ProductID: ioProdID, EffectivePrice: nil,
		},
	}

	invRepo.On("GetByID", ctx, ioInvID).Return(ioInventory(), nil)
	invRepo.On("GetTransactionsByInventoryIDsWithCounter", ctx, ioInvID, mock.Anything, mock.Anything).
		Return([]*repository.InventoryTransactionWithCounter{purchase}, nil)
	invRepo.On("GetTransactionsByInventoryIDsWithCounter", ctx, ioInvID, (*time.Time)(nil), mock.Anything).
		Return([]*repository.InventoryTransactionWithCounter{}, nil)
	spRepo.On("GetPOItemsWithPriceByIDs", ctx, mock.Anything, ioInvID).
		Return(poInfo, nil)

	req := ioReq()
	req.IgnoreMissingSellingPrice = true
	resp, err := svc.Export(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "https://signed.example/file.xlsx", resp.DownloadURL)
	assert.True(t, s3.UploadCalled, "UploadFile must be called on a confirmed export")
	assert.True(t, s3.PresignCalled, "GeneratePresignedURL must be called on a confirmed export")
}

func TestExport_CrossInventoryTransferInResolvesSourcePOI(t *testing.T) {
	// A transfer-in into the destination inventory whose source POI lives in
	// another inventory must still be reflected in the export. The service
	// should call GetPOItemsWithPriceByIDsAcrossInventories for any POI not
	// returned by the inventory-scoped lookup.
	ctx := context.Background()
	invRepo := repositorymocks.NewInventoryRepository(t)
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	s3 := &fakeS3{}
	svc := NewInventoryInOutExportService(invRepo, spRepo, s3, testExportPrefix)

	const sourcePOIID = uint(900) // POI that lives in OTHER inventory
	srcPOI := uint(sourcePOIID)
	transferIn := &repository.InventoryTransactionWithCounter{
		InventoryTransaction: &models.InventoryTransaction{
			Base:            models.Base{ID: 1, CreatedAt: time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)},
			InventoryItemID: ioItemID,
			TransactionType: models.InventoryTransactionTypeTransferIn,
			Quantity:        decimal.NewFromInt(7),
			Price:           5.0,
		},
		CounterPOIID: &srcPOI,
	}

	// First (inventory-scoped) call returns NOTHING — the source POI is in
	// another inventory and the SQL filter excludes it.
	emptyInfo := map[uint]*repository.POItemSellingPriceInfo{}
	// Second (across-inventories) call returns the source POI metadata.
	srcPrice := decimal.NewFromInt(8)
	srcInfo := map[uint]*repository.POItemSellingPriceInfo{
		sourcePOIID: {
			POItemID: sourcePOIID, POID: 91, PONumber: "PO-SRC",
			ProductID: ioProdID, EffectivePrice: &srcPrice,
			PurchasePrice: decimal.NewFromInt(5),
		},
	}

	invRepo.On("GetByID", ctx, ioInvID).Return(ioInventory(), nil)
	invRepo.On("GetTransactionsByInventoryIDsWithCounter", ctx, ioInvID, mock.Anything, mock.Anything).
		Return([]*repository.InventoryTransactionWithCounter{transferIn}, nil)
	invRepo.On("GetTransactionsByInventoryIDsWithCounter", ctx, ioInvID, (*time.Time)(nil), mock.Anything).
		Return([]*repository.InventoryTransactionWithCounter{}, nil)
	spRepo.On("GetPOItemsWithPriceByIDs", ctx, mock.Anything, ioInvID).
		Return(emptyInfo, nil)
	spRepo.On("GetPOItemsWithPriceByIDsAcrossInventories", ctx, mock.Anything).
		Return(srcInfo, nil)

	resp, err := svc.Export(ctx, ioReq())
	require.NoError(t, err)
	assert.NotEmpty(t, resp.DownloadURL)
	assert.True(t, s3.UploadCalled, "transfer-in row should produce an export")
	// Calling expectations are validated via mockery's NewSellingPriceRepository(t).
}

func TestExport_FractionalCarryOverDoesNotConfusePrecondition(t *testing.T) {
	// Precondition math must run on decimal.Decimal so fractional historical
	// quantities don't drift via float64 conversion. POI 800 has begin
	// stock = 0.1 + 0.2 - 0.3 = 0 (exact in Decimal, but ≠ 0 in float64).
	// With Decimal math the POI must be treated as zero-balance and (with
	// no in-window activity) excluded from precondition; export succeeds.
	ctx := context.Background()
	invRepo := repositorymocks.NewInventoryRepository(t)
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	s3 := &fakeS3{}
	svc := NewInventoryInOutExportService(invRepo, spRepo, s3, testExportPrefix)

	const fracPOI = uint(800)
	poiID := uint(fracPOI)
	mkHist := func(id uint, ttype models.InventoryTransactionType, qty string, day int) *repository.InventoryTransactionWithCounter {
		t := &repository.InventoryTransactionWithCounter{
			InventoryTransaction: &models.InventoryTransaction{
				Base:            models.Base{ID: id, CreatedAt: time.Date(2026, 3, day, 12, 0, 0, 0, time.UTC)},
				InventoryItemID: ioItemID,
				TransactionType: ttype,
				Quantity:        decimal.RequireFromString(qty),
			},
		}
		// Purchases carry their own POI; consumes resolve the source POI via the counter.
		if ttype == models.InventoryTransactionTypePurchase {
			t.PurchaseOrderItemID = &poiID
		} else {
			t.CounterPOIID = &poiID
		}
		return t
	}
	historical := []*repository.InventoryTransactionWithCounter{
		mkHist(1, models.InventoryTransactionTypePurchase, "0.1", 1),
		mkHist(2, models.InventoryTransactionTypePurchase, "0.2", 2),
		mkHist(3, models.InventoryTransactionTypeSell, "0.3", 5),
	}

	// poInfo has the POI but no selling price (EffectivePrice nil). Even so,
	// because the POI's begin == 0 exactly and it has no in-window activity,
	// the precondition must NOT flag it as missing.
	poInfo := map[uint]*repository.POItemSellingPriceInfo{
		fracPOI: {
			POItemID: fracPOI, POID: 81, PONumber: "PO-FRAC",
			ProductID: ioProdID, EffectivePrice: nil,
		},
	}

	invRepo.On("GetByID", ctx, ioInvID).Return(ioInventory(), nil)
	invRepo.On("GetTransactionsByInventoryIDsWithCounter", ctx, ioInvID, (*time.Time)(nil), mock.Anything).
		Return(historical, nil)
	invRepo.On("GetTransactionsByInventoryIDsWithCounter", ctx, ioInvID, mock.MatchedBy(func(t *time.Time) bool { return t != nil }), mock.Anything).
		Return([]*repository.InventoryTransactionWithCounter{}, nil)
	spRepo.On("GetPOItemsWithPriceByIDs", ctx, mock.Anything, ioInvID).
		Return(poInfo, nil)

	resp, err := svc.Export(ctx, ioReq())
	require.NoError(t, err, "depleted-to-zero POI must not be flagged as missing selling price")
	assert.NotNil(t, resp)
}

func TestExport_BadDateRange(t *testing.T) {
	ctx := context.Background()
	invRepo := repositorymocks.NewInventoryRepository(t)
	spRepo := repositorymocks.NewSellingPriceRepository(t)
	s3 := &fakeS3{}
	svc := NewInventoryInOutExportService(invRepo, spRepo, s3, testExportPrefix)

	resp, err := svc.Export(ctx, dto.InventoryInOutExportRequest{
		InventoryID: ioInvID,
		StartDate:   "2026-04-30",
		EndDate:     "2026-04-01",
	})
	require.Error(t, err)
	assert.Nil(t, resp)
	var appErr *pkg.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, pkg.ErrorCodeValidation, appErr.Code)
}
