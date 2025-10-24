package handlers

import (
	"cim-backend/internal/services"
	"cim-backend/pkg"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

type RevenueExpenseHandler struct {
	excelService services.ExcelService
}

// NewRevenueExpenseHandler creates a new RevenueExpenseHandler
func NewRevenueExpenseHandler(excelService services.ExcelService) *RevenueExpenseHandler {
	return &RevenueExpenseHandler{
		excelService: excelService,
	}
}

// FinalizeRevenueExpenseRequest represents the request to finalize revenue expense
type FinalizeRevenueExpenseRequest struct {
	Date string `json:"date" validate:"required"`
}

// FinalizeRevenueExpenseResponse represents the response after finalizing
type FinalizeRevenueExpenseResponse struct {
	Message string `json:"message"`
	Date    string `json:"date"`
	NextDay string `json:"next_day"`
}

// FinalizeRevenueExpense creates a new row for the next day in revenue-expense excel/sheet
// @Summary Finalize revenue expense
// @Description Creates a new row with the next day's date in the revenue-expense excel/sheet based on the provided date
// @Tags revenue-expenses
// @Accept json
// @Produce json
// @Param request body FinalizeRevenueExpenseRequest true "Finalize request with date"
// @Success 200 {object} FinalizeRevenueExpenseResponse "Successfully finalized"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 500 {object} map[string]string "Internal server error"
// @Security BearerAuth
// @Router /revenue-expenses/finalize [post]
func (h *RevenueExpenseHandler) FinalizeRevenueExpense(c echo.Context) error {
	var req FinalizeRevenueExpenseRequest

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request format",
		})
	}

	// Validate request
	if err := pkg.Validator.Struct(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error":   "Validation failed",
			"details": err.Error(),
		})
	}

	// Parse date (expecting format: YYYY-MM-DD)
	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error":   "Invalid date format",
			"details": "Expected format: YYYY-MM-DD",
		})
	}

	// Call service to finalize
	if err := h.excelService.FinalizeRevenueExpense(c.Request().Context(), date); err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]string{
				"error": appErr.Message,
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error":   "Failed to finalize revenue expense",
			"details": err.Error(),
		})
	}

	nextDay := date.AddDate(0, 0, 1)

	response := FinalizeRevenueExpenseResponse{
		Message: "Revenue expense finalized successfully",
		Date:    req.Date,
		NextDay: nextDay.Format("2006-01-02"),
	}

	return c.JSON(http.StatusOK, response)
}
