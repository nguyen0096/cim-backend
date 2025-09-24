package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/xuri/excelize/v2"
)

type ExcelHandler struct {
	excelService ExcelService
}

type ExcelService interface {
	ExportProducts() (*excelize.File, error)
	ExportInventory() (*excelize.File, error)
	ImportProducts(file *excelize.File) error
	ImportInventory(file *excelize.File) error
	GetProductTemplate() (*excelize.File, error)
	GetInventoryTemplate() (*excelize.File, error)
}

func NewExcelHandler(excelService ExcelService) *ExcelHandler {
	return &ExcelHandler{
		excelService: excelService,
	}
}

func (h *ExcelHandler) ExportProducts(c echo.Context) error {
	file, err := h.excelService.ExportProducts()
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
	file, err := h.excelService.ExportInventory()
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

	if err := h.excelService.ImportProducts(excelFile); err != nil {
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

	if err := h.excelService.ImportInventory(excelFile); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to import inventory"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Inventory imported successfully"})
}

func (h *ExcelHandler) GetProductTemplate(c echo.Context) error {
	file, err := h.excelService.GetProductTemplate()
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
	file, err := h.excelService.GetInventoryTemplate()
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
