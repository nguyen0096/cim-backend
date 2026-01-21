package handlers

import (
	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/internal/services"
	"cim-backend/pkg"
	"cim-backend/pkg/log"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
)

type RevenueExpenseHandler struct {
	excelService                   services.ExcelService
	settingsService                services.SettingsService
	revenueExpenseFinalizationRepo repository.RevenueExpenseFinalizationRepository
}

// NewRevenueExpenseHandler creates a new RevenueExpenseHandler
func NewRevenueExpenseHandler(excelService services.ExcelService, settingsService services.SettingsService, revenueExpenseFinalizationRepo repository.RevenueExpenseFinalizationRepository) *RevenueExpenseHandler {
	return &RevenueExpenseHandler{
		excelService:                   excelService,
		settingsService:                settingsService,
		revenueExpenseFinalizationRepo: revenueExpenseFinalizationRepo,
	}
}

// FinalizeRevenueExpenseRequest represents the request to finalize revenue expense
type FinalizeRevenueExpenseRequest struct {
	Date string `json:"date"`
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

	if req.Date == "" {
		req.Date = time.Now().Format("2006-01-02")
	}

	parsedDate, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		return pkg.ErrInvalidRequestBodyI18n(ctx, err)
	}

	// Create finalization record without status first
	finalization := &models.RevenueExpenseFinalization{
		FinalizedDate: parsedDate,
		Status:        nil,
	}
	if err := h.revenueExpenseFinalizationRepo.Create(ctx, finalization); err != nil {
		return fmt.Errorf("failed to create finalization record: %w", err)
	}

	// Update status to failed on error, success on completion
	var finalizationErr *pkg.AppError
	defer func() {
		if finalizationErr != nil {
			repoCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
			defer cancel()
			// Update status to failed and write error reason
			failedStatus := models.RevenueExpenseFinalizationStatusFailed
			finalization.Status = &failedStatus
			reason := finalizationErr.Error()
			finalization.Reason = &reason
			if updateErr := h.revenueExpenseFinalizationRepo.Update(repoCtx, finalization); updateErr != nil {
				log.WithFields(logrus.Fields{
					"error":   updateErr.Error(),
					"details": "Failed to update finalization status to failed",
				}).Error("Failed to update finalization status")
			}
		}
	}()

	// Get last successful finalization date from database
	lastSuccessfulFinalization, err := h.revenueExpenseFinalizationRepo.GetLastSuccessful(ctx)
	lastFinalizedDate := time.Now().Truncate(24 * time.Hour) // Default to today
	if err != nil {
		// If error occurred (other than not found), log it but continue with fallback
		log.WithFields(logrus.Fields{
			"error":   err.Error(),
			"details": "Failed to get last successful finalization, using today's date",
		}).Warn("Failed to get last successful finalization")
	} else if lastSuccessfulFinalization != nil {
		// Use the finalized_date from the last successful finalization
		lastFinalizedDate = lastSuccessfulFinalization.FinalizedDate.Truncate(24 * time.Hour)
	}

	// Call service to finalize
	err = h.excelService.FinalizeRevenueExpense(ctx, lastFinalizedDate)
	if err != nil {
		// Check if error is already an AppError, return it directly
		var appErr *pkg.AppError
		if errors.As(err, &appErr) {
			finalizationErr = appErr
			return err
		}
		finalizationErr = pkg.ErrFailedToFinalizeRevenueExpense(ctx, err)
		return finalizationErr
	}

	// Update finalization status to success
	successStatus := models.RevenueExpenseFinalizationStatusSuccess
	finalization.Status = &successStatus
	if err := h.revenueExpenseFinalizationRepo.Update(ctx, finalization); err != nil {
		return fmt.Errorf("failed to update finalization status: %w", err)
	}

	response := FinalizeRevenueExpenseResponse{
		Message: "Revenue expense finalized successfully",
		Date:    lastFinalizedDate.Format("2006-01-02"),
		NextDay: time.Now().AddDate(0, 0, 1).Format("2006-01-02"),
	}

	return c.JSON(http.StatusOK, response)
}

// ListFinalizedDates lists revenue expense finalizations with pagination
// @Summary List finalized dates
// @Description Get a paginated list of revenue expense finalizations (finalized dates)
// @Tags revenue-expenses
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(20)
// @Success 200 {object} map[string]interface{} "List of finalized dates"
// @Failure 500 {object} map[string]string "Internal server error"
// @Security BearerAuth
// @Router /revenue-expenses/finalized-dates [get]
func (h *RevenueExpenseHandler) ListFinalizedDates(c echo.Context) error {
	ctx := c.Request().Context()

	// Parse query parameters
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	page, _ := strconv.Atoi(c.QueryParam("page"))

	// Set defaults
	if limit == 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if page == 0 {
		page = 1
	}

	// Calculate offset
	offset := (page - 1) * limit

	// Get finalizations and total count
	finalizations, total, err := h.revenueExpenseFinalizationRepo.List(ctx, limit, offset)
	if err != nil {
		log.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Error("Failed to list finalized dates")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch finalized dates"})
	}

	// Calculate total pages
	totalPages := int((total + int64(limit) - 1) / int64(limit))

	// Create response
	response := map[string]interface{}{
		"data":       finalizations,
		"total":      total,
		"page":       page,
		"limit":      limit,
		"totalPages": totalPages,
	}

	return c.JSON(http.StatusOK, response)
}

// FinalizeRevenueExpenseByDate finalizes revenue expense by date from path parameter
// @Summary Finalize revenue expense by date
// @Description Creates a new row with the next day's date in the revenue-expense excel/sheet based on the provided date in path parameter
// @Tags revenue-expenses
// @Accept json
// @Produce json
// @Param date path string true "Date in YYYY-MM-DD format"
// @Success 200 {object} FinalizeRevenueExpenseResponse "Successfully finalized"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 500 {object} map[string]string "Internal server error"
// @Security BearerAuth
// @Router /revenue-expenses/finalize/{date} [post]
func (h *RevenueExpenseHandler) FinalizeRevenueExpenseByDate(c echo.Context) error {
	ctx := c.Request().Context()

	// Get date from path parameter
	dateStr := c.Param("date")
	if dateStr == "" {
		return pkg.ErrValidationI18n(ctx, errors.New("date parameter is required"))
	}

	// Parse date
	finalizeDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return pkg.ErrValidationI18n(ctx, fmt.Errorf("invalid date format, expected YYYY-MM-DD: %w", err))
	}

	// Truncate to start of day
	finalizeDate = finalizeDate.Truncate(24 * time.Hour)
	today := time.Now().Truncate(24 * time.Hour)

	// Call service to finalize
	finalizedDate, err := h.excelService.FinalizeRevenueExpense(ctx, finalizeDate, today)
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
		Date:    finalizeDate.Format("2006-01-02"),
		NextDay: finalizedDate.Format("2006-01-02"),
	}

	return c.JSON(http.StatusOK, response)
}
