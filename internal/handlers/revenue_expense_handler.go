package handlers

import (
	"cim-backend/internal/config"
	"cim-backend/internal/services"
	"cim-backend/pkg"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
)

type RevenueExpenseHandler struct {
	excelService    services.ExcelService
	settingsService services.SettingsService
	logger          *logrus.Logger
}

// NewRevenueExpenseHandler creates a new RevenueExpenseHandler
func NewRevenueExpenseHandler(excelService services.ExcelService, settingsService services.SettingsService, logger *logrus.Logger) *RevenueExpenseHandler {
	return &RevenueExpenseHandler{
		excelService:    excelService,
		settingsService: settingsService,
		logger:          logger,
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

	var lastFinalizedDate time.Time
	if err := h.settingsService.GetSettingValue(c.Request().Context(), config.LastFinalizedDateSettingsKey, &lastFinalizedDate); err != nil {
		h.logger.WithFields(logrus.Fields{
			"error":   err.Error(),
			"details": "Failed to get last finalized date",
		}).Error("Failed to get last finalized date")
		// Continue with the current date
	}

	if lastFinalizedDate.IsZero() {
		lastFinalizedDate = time.Now()
	}

	// Call service to finalize
	if err := h.excelService.FinalizeRevenueExpense(c.Request().Context(), lastFinalizedDate); err != nil {
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

	nextDay := lastFinalizedDate.AddDate(0, 0, 1)
	if err := h.settingsService.SetSetting(c.Request().Context(), config.LastFinalizedDateSettingsKey, nextDay); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error":   "Failed to set last finalized date",
			"details": err.Error(),
		})
	}

	response := FinalizeRevenueExpenseResponse{
		Message: "Revenue expense finalized successfully",
		Date:    lastFinalizedDate.Format("2006-01-02"),
		NextDay: nextDay.Format("2006-01-02"),
	}

	return c.JSON(http.StatusOK, response)
}
