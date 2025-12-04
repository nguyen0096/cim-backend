package handlers

import (
	"cim-backend/internal/config"
	"cim-backend/internal/services"
	"cim-backend/pkg"
	"cim-backend/pkg/log"
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
)

type RevenueExpenseHandler struct {
	excelService    services.ExcelService
	settingsService services.SettingsService
}

// NewRevenueExpenseHandler creates a new RevenueExpenseHandler
func NewRevenueExpenseHandler(excelService services.ExcelService, settingsService services.SettingsService) *RevenueExpenseHandler {
	return &RevenueExpenseHandler{
		excelService:    excelService,
		settingsService: settingsService,
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
	ctx := c.Request().Context()
	var req FinalizeRevenueExpenseRequest

	if err := c.Bind(&req); err != nil {
		return pkg.ErrInvalidRequestBodyI18n(ctx, err)
	}

	// Validate request
	if err := pkg.Validator.Struct(&req); err != nil {
		return pkg.ErrValidationI18n(ctx, err)
	}

	var lastFinalizedDate time.Time
	if err := h.settingsService.GetSettingValue(ctx, config.LastFinalizedDateSettingsKey, &lastFinalizedDate); err != nil {
		log.WithFields(logrus.Fields{
			"error":   err.Error(),
			"details": "Failed to get last finalized date",
		}).Error("Failed to get last finalized date")
		// Continue with the current date
	}

	if lastFinalizedDate.IsZero() {
		lastFinalizedDate = time.Now()
	}

	defer func() {
		settingsCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := h.settingsService.SetSetting(settingsCtx, config.LastFinalizedDateSettingsKey, time.Now()); err != nil {
			log.WithFields(logrus.Fields{
				"error":   err.Error(),
				"details": "Failed to set last finalized date to now",
			}).Error("Failed to set last finalized date")
		}
	}()

	// Call service to finalize
	finalizedDate, err := h.excelService.FinalizeRevenueExpense(ctx, lastFinalizedDate, time.Now())
	if err != nil {
		// Check if error is already an AppError, return it directly
		var appErr *pkg.AppError
		if errors.As(err, &appErr) {
			return err
		}
		return pkg.ErrFailedToFinalizeRevenueExpense(ctx, err)
	}

	response := FinalizeRevenueExpenseResponse{
		Message: "Revenue expense finalized successfully",
		Date:    lastFinalizedDate.Format("2006-01-02"),
		NextDay: finalizedDate.Format("2006-01-02"),
	}

	return c.JSON(http.StatusOK, response)
}
