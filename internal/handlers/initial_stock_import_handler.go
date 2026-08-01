package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/labstack/echo/v4"

	"cim-backend/internal/services"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
)

const (
	// initialStockMaxUploadBytes bounds the upload in the handler. echo's BodyLimit
	// is unusable here: it returns an *echo.HTTPError, which HandleError passes back
	// unwritten, so CustomErrorHandler turns it into a generic 500 with no body the
	// UI can render. The frontend caps at 10 MB too, so this is the backstop.
	initialStockMaxUploadBytes = 10 << 20
	initialStockMaxUploadMB    = initialStockMaxUploadBytes >> 20
	// initialStockMaxKeyLen mirrors initial_stock_imports.idempotency_key VARCHAR(255).
	// Checked before the service runs, so an oversized key cannot surface as a
	// database error at the receipt insert after the whole plan has been built.
	// Counted in runes: VARCHAR(n) limits characters, not bytes, so counting bytes
	// would reject a key that fits the column.
	initialStockMaxKeyLen = 255
)

type InitialStockImportHandler struct {
	service services.InitialStockImportService
}

func NewInitialStockImportHandler(service services.InitialStockImportService) *InitialStockImportHandler {
	return &InitialStockImportHandler{service: service}
}

// ListInventories godoc
// @Summary List inventories for the initial-stock tool
// @Description Developer-only picker source: active, non-deleted inventories. An empty list is a success.
// @Tags tools
// @Produce json
// @Success 200 {object} dto.InitialStockInventoriesResponse
// @Failure 403 {object} map[string]interface{} "Forbidden"
// @Security BearerAuth
// @Router /tools/inventories [get]
func (h *InitialStockImportHandler) ListInventories(c echo.Context) error {
	resp, err := h.service.ListInventories(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, resp)
}

// ListSheets godoc
// @Summary List the worksheets of an initial-stock workbook
// @Description Returns every sheet with its header verdict and data row count so the client needs no spreadsheet parser.
// @Tags tools
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true ".xlsx workbook"
// @Success 200 {object} dto.InitialStockSheetsResponse
// @Failure 400 {object} map[string]interface{} "Bad request"
// @Failure 403 {object} map[string]interface{} "Forbidden"
// @Security BearerAuth
// @Router /tools/initial-stock/sheets [post]
func (h *InitialStockImportHandler) ListSheets(c echo.Context) error {
	if err := boundInitialStockBody(c); err != nil {
		return err
	}
	data, err := readInitialStockUpload(c)
	if err != nil {
		return err
	}
	resp, err := h.service.ListSheets(c.Request().Context(), data)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, resp)
}

// Import godoc
// @Summary Load pre-app opening stock from a workbook
// @Description Developer-only. dry_run=true previews and writes nothing; dry_run=false applies in one transaction. Send Idempotency-Key on apply only; a committed key replays the original result.
// @Tags tools
// @Accept multipart/form-data
// @Produce json
// @Param Idempotency-Key header string false "Apply-only idempotency key"
// @Param file formData file true ".xlsx workbook"
// @Param inventory_id formData int true "Target inventory"
// @Param sheet_name formData string true "Byte-exact sheet name from the sheet listing"
// @Param dry_run formData string true "Exactly \"true\" or \"false\""
// @Success 200 {object} dto.InitialStockImportResponse
// @Failure 400 {object} map[string]interface{} "Bad request or per-row validation errors"
// @Failure 403 {object} map[string]interface{} "Forbidden"
// @Failure 404 {object} map[string]interface{} "Unknown inventory"
// @Failure 409 {object} map[string]interface{} "Already imported, reconcile open, or key reuse"
// @Security BearerAuth
// @Router /tools/initial-stock/import [post]
func (h *InitialStockImportHandler) Import(c echo.Context) error {
	// Must precede the first FormValue/FormFile: either triggers multipart parsing,
	// which would read (and possibly spool to disk) the whole body before any bound
	// is installed.
	if err := boundInitialStockBody(c); err != nil {
		return err
	}

	dryRun, err := parseStrictBool(c.FormValue("dry_run"))
	if err != nil {
		return err
	}

	rawInventoryID := strings.TrimSpace(c.FormValue("inventory_id"))
	inventoryID, convErr := strconv.ParseUint(rawInventoryID, 10, 64)
	if rawInventoryID == "" || convErr != nil || inventoryID == 0 {
		return pkg.ErrInitialStock(pkg.ErrorCodeValidation, pkg.ErrKeyInitialStockInventoryRequired)
	}

	sheetName := c.FormValue("sheet_name")
	if strings.TrimSpace(sheetName) == "" {
		return pkg.ErrInitialStock(pkg.ErrorCodeValidation, pkg.ErrKeyInitialStockSheetNameRequired)
	}

	data, err := readInitialStockUpload(c)
	if err != nil {
		return err
	}

	sum := sha256.Sum256(data)
	req := dto.InitialStockImportRequest{
		InventoryID: uint(inventoryID),
		SheetName:   sheetName,
		DryRun:      dryRun,
		FileName:    initialStockUploadName(c),
		FileSHA256:  hex.EncodeToString(sum[:]),
	}
	// Apply only: an idempotency key on a preview would let a successful preview
	// retire a key held for an in-flight apply.
	if !dryRun {
		req.IdempotencyKey = strings.TrimSpace(c.Request().Header.Get("Idempotency-Key"))
		if utf8.RuneCountInString(req.IdempotencyKey) > initialStockMaxKeyLen {
			return pkg.ErrInitialStock(pkg.ErrorCodeValidation,
				pkg.ErrKeyInitialStockKeyTooLong, initialStockMaxKeyLen)
		}
	}

	resp, err := h.service.Import(c.Request().Context(), req, data)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, resp)
}

// parseStrictBool fails closed: anything other than the exact strings "true" and
// "false" is rejected, so a missing or malformed flag can never fall through to a write.
func parseStrictBool(value string) (bool, error) {
	switch value {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, pkg.ErrInitialStock(pkg.ErrorCodeValidation, pkg.ErrKeyInitialStockInvalidDryRun)
	}
}

func initialStockUploadName(c echo.Context) string {
	if header, err := c.FormFile("file"); err == nil && header != nil {
		return header.Filename
	}
	return ""
}

// boundInitialStockBody caps the request body and parses the multipart form under
// that cap. Call it before touching any form value, and exactly once per request:
// FormValue and FormFile both trigger parsing, which would otherwise read the whole
// body (and spool it to disk) before any bound applied.
func boundInitialStockBody(c echo.Context) error {
	req := c.Request()
	if req.ContentLength > initialStockMaxUploadBytes {
		return pkg.ErrInitialStock(pkg.ErrorCodeValidation, pkg.ErrKeyInitialStockFileTooLarge, initialStockMaxUploadMB)
	}
	req.Body = http.MaxBytesReader(c.Response(), req.Body, initialStockMaxUploadBytes)

	if err := req.ParseMultipartForm(initialStockMaxUploadBytes); err != nil {
		if exceedsBodyLimit(err) {
			return pkg.ErrInitialStock(pkg.ErrorCodeValidation, pkg.ErrKeyInitialStockFileTooLarge, initialStockMaxUploadMB)
		}
		return pkg.ErrInitialStock(pkg.ErrorCodeValidation, pkg.ErrKeyInitialStockFileRequired)
	}
	return nil
}

// exceedsBodyLimit reports whether err is the body-cap rejection. ParseMultipartForm
// sometimes wraps it in a multipart error, so the string is checked as well as the
// typed error.
func exceedsBodyLimit(err error) bool {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		return true
	}
	return strings.Contains(err.Error(), "request body too large")
}

// readInitialStockUpload validates and reads the upload fully into memory. Nothing
// is written to disk.
func readInitialStockUpload(c echo.Context) ([]byte, error) {
	header, err := c.FormFile("file")
	if err != nil || header == nil {
		return nil, pkg.ErrInitialStock(pkg.ErrorCodeValidation, pkg.ErrKeyInitialStockFileRequired)
	}
	// Narrower than pkg.IsAllowedFileTypes, which also accepts .csv and the BIFF
	// .xls excelize cannot read.
	if !strings.EqualFold(filepath.Ext(header.Filename), ".xlsx") {
		return nil, pkg.ErrInitialStock(pkg.ErrorCodeValidation, pkg.ErrKeyInitialStockInvalidFileType)
	}
	if header.Size > initialStockMaxUploadBytes {
		return nil, pkg.ErrInitialStock(pkg.ErrorCodeValidation, pkg.ErrKeyInitialStockFileTooLarge, initialStockMaxUploadMB)
	}

	src, err := header.Open()
	if err != nil {
		return nil, pkg.ErrInitialStock(pkg.ErrorCodeValidation, pkg.ErrKeyInitialStockParseFailed)
	}
	defer func(f multipart.File) { _ = f.Close() }(src)

	data, err := io.ReadAll(io.LimitReader(src, initialStockMaxUploadBytes+1))
	if err != nil {
		return nil, pkg.ErrInitialStock(pkg.ErrorCodeValidation, pkg.ErrKeyInitialStockParseFailed)
	}
	if len(data) > initialStockMaxUploadBytes {
		return nil, pkg.ErrInitialStock(pkg.ErrorCodeValidation, pkg.ErrKeyInitialStockFileTooLarge, initialStockMaxUploadMB)
	}
	if len(data) == 0 {
		return nil, pkg.ErrInitialStock(pkg.ErrorCodeValidation, pkg.ErrKeyInitialStockEmptyFile)
	}
	return data, nil
}
