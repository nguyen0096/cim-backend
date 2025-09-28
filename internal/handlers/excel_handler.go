package handlers

import (
	"import-export-backend/internal/services"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/xuri/excelize/v2"
)

type ExcelHandler struct {
	excelService services.ExcelService
}

func NewExcelHandler(excelService services.ExcelService) *ExcelHandler {
	return &ExcelHandler{
		excelService: excelService,
	}
}

func (h *ExcelHandler) ExportProducts(c echo.Context) error {
	file, err := h.excelService.ExportProducts(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to export products"})
	}

	c.Response().Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Response().Header().Set("Content-Disposition", "attachment; filename=products.xlsx")

	// Convert excelize.File to bytes
	buffer, err := file.WriteToBuffer()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to generate Excel file"})
	}

	return c.Blob(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buffer.Bytes())
}

func (h *ExcelHandler) ExportInventory(c echo.Context) error {
	file, err := h.excelService.ExportInventory(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to export inventory"})
	}

	c.Response().Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Response().Header().Set("Content-Disposition", "attachment; filename=inventory.xlsx")

	// Convert excelize.File to bytes
	buffer, err := file.WriteToBuffer()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to generate Excel file"})
	}

	return c.Blob(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buffer.Bytes())
}

func (h *ExcelHandler) ImportProducts(c echo.Context) error {
	file, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "No file uploaded"})
	}

	src, err := file.Open()
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Failed to open file"})
	}
	defer src.Close()

	// Read file content
	buffer := make([]byte, file.Size)
	_, err = src.Read(buffer)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Failed to read file"})
	}

	// Parse Excel file
	excelFile, err := excelize.OpenReader(src)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Failed to parse Excel file"})
	}

	if err := h.excelService.ImportProducts(c.Request().Context(), excelFile); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to import products"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Products imported successfully"})
}

func (h *ExcelHandler) ImportInventory(c echo.Context) error {
	file, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "No file uploaded"})
	}

	src, err := file.Open()
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Failed to open file"})
	}
	defer src.Close()

	// Parse Excel file
	excelFile, err := excelize.OpenReader(src)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Failed to parse Excel file"})
	}

	if err := h.excelService.ImportInventory(c.Request().Context(), excelFile); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to import inventory"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Inventory imported successfully"})
}

func (h *ExcelHandler) GetProductTemplate(c echo.Context) error {
	file, err := h.excelService.GetProductTemplate(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to generate product template"})
	}

	c.Response().Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Response().Header().Set("Content-Disposition", "attachment; filename=product_template.xlsx")

	// Convert excelize.File to bytes
	buffer, err := file.WriteToBuffer()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to generate Excel file"})
	}

	return c.Blob(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buffer.Bytes())
}

func (h *ExcelHandler) GetInventoryTemplate(c echo.Context) error {
	file, err := h.excelService.GetInventoryTemplate(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to generate inventory template"})
	}

	c.Response().Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Response().Header().Set("Content-Disposition", "attachment; filename=inventory_template.xlsx")

	// Convert excelize.File to bytes
	buffer, err := file.WriteToBuffer()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to generate Excel file"})
	}

	return c.Blob(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buffer.Bytes())
}

// VerifyFileAndSheet verifies that the filepath and sheetname exist
// @Summary Verify Excel file and sheet
// @Description Verifies that the specified filepath and sheetname exist and are valid
// @Tags excel
// @Accept json
// @Produce json
// @Param request body VerifyFileRequest true "File verification request"
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

	if err := h.excelService.VerifyFileAndSheet(c.Request().Context(), request.FilePath, request.SheetName); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "File or sheet verification failed: " + err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "File and sheet verification successful"})
}
