package handlers

import (
	"cim-backend/internal/services"
	"cim-backend/pkg"
	"net/http"
	"os"
	"strings"

	"github.com/labstack/echo/v4"
)

type ExcelHandler struct {
	excelService services.ExcelService
}

func NewExcelHandler(excelService services.ExcelService) *ExcelHandler {
	return &ExcelHandler{
		excelService: excelService,
	}
}

// VerifyFileAndSheet verifies that the file (local or Google Sheets) and sheetname exist
// @Summary Verify file and sheet
// @Description Verifies that the specified file (local Excel or Google Sheets URL) and sheetname exist and are valid
// @Tags excel
// @Accept json
// @Produce json
// @Param request body object true "File verification request"
// @Success 200 {object} map[string]string "Verification successful"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 404 {object} map[string]string "File or sheet not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /excel/verify [post]
func (h *ExcelHandler) VerifyFileAndSheet(c echo.Context) error {
	var request struct {
		FilePath  string `json:"filepath"`
		SheetName string `json:"sheetname"`
	}

	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request format"})
	}

	// Manual validation
	if request.FilePath == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Filepath is required"})
	}

	if request.SheetName == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Sheet name is required"})
	}

	// Detect if filepath is a Google Sheets URL or local file path
	isGoogleSheets := strings.Contains(request.FilePath, "docs.google.com/spreadsheets")

	if isGoogleSheets {
		// Handle Google Sheets verification
		spreadsheetID, err := pkg.ExtractSpreadsheetID(request.FilePath)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid Google Sheets URL: " + err.Error()})
		}

		// Get service account file path from environment variable
		serviceAccountFilePath := os.Getenv("GOOGLE_SERVICE_ACCOUNT")
		if serviceAccountFilePath == "" {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Service account file path not configured"})
		}

		// Verify the spreadsheet and sheet
		if err := h.excelService.VerifyGoogleSheetAndSheet(c.Request().Context(), serviceAccountFilePath, spreadsheetID, request.SheetName); err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Google Sheets verification failed: " + err.Error()})
		}

		return c.JSON(http.StatusOK, map[string]string{
			"message":       "Google Sheets verification successful",
			"spreadsheetID": spreadsheetID,
			"type":          "google_sheets",
		})
	} else {
		// Handle local file verification
		if err := h.excelService.VerifyFileAndSheet(c.Request().Context(), request.FilePath, request.SheetName); err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Local file verification failed: " + err.Error()})
		}

		return c.JSON(http.StatusOK, map[string]string{
			"message":  "Local file verification successful",
			"filepath": request.FilePath,
			"type":     "local_file",
		})
	}
}
