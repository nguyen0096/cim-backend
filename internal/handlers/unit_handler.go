package handlers

import (
	"cim-backend/internal/models"
	"cim-backend/internal/services"
	"cim-backend/pkg"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

// UnitHandler handles HTTP requests for measurement units
type UnitHandler struct {
	unitService services.UnitService
}

// NewUnitHandler creates a new UnitHandler instance
func NewUnitHandler(unitService services.UnitService) *UnitHandler {
	return &UnitHandler{
		unitService: unitService,
	}
}

// UnitListRequest defines query parameters for listing units
type UnitListRequest struct {
	Limit    int    `query:"limit"`
	Page     int    `query:"page"`
	Sort     string `query:"sort"`
	Order    string `query:"order"`
	UnitType string `query:"unit_type"`
	Query    string `query:"q"`
	BaseOnly bool   `query:"base_only"`
}

func (r *UnitListRequest) setDefaults() {
	if r.Limit <= 0 {
		r.Limit = 20
	}
	if r.Page <= 0 {
		r.Page = 1
	}
	if strings.TrimSpace(r.Sort) == "" {
		r.Sort = "created_at"
	}
	if strings.TrimSpace(r.Order) == "" {
		r.Order = "desc"
	}
}

// ListUnits godoc
// @Summary List measurement units
// @Description Retrieve units with pagination, optional filtering by type, and search query
// @Tags units
// @Accept json
// @Produce json
// @Param limit query int false "Number of items per page" default(20)
// @Param page query int false "Page number" default(1)
// @Param sort query string false "Sort field" default("created_at")
// @Param order query string false "Sort order (asc/desc)" default("desc")
// @Param unit_type query string false "Filter by unit type (e.g., mass, volume)"
// @Param q query string false "Search units by name or symbol"
// @Param base_only query bool false "Filter to show only base units" default(false)
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /units [get]
func (h *UnitHandler) ListUnits(c echo.Context) error {
	var request UnitListRequest
	if err := c.Bind(&request); err != nil {
		return pkg.ErrInvalidRequestBody(err)
	}

	request.setDefaults()
	offset := (request.Page - 1) * request.Limit

	var (
		units []models.Unit
		err   error
		total int64
	)

	if strings.TrimSpace(request.Query) != "" {
		units, err = h.unitService.SearchUnits(c.Request().Context(), request.Query, request.Limit, offset, request.Sort, request.Order, request.UnitType, request.BaseOnly)
		if err != nil {
			return err
		}
		total, err = h.unitService.CountSearchUnits(c.Request().Context(), request.Query, request.UnitType, request.BaseOnly)
		if err != nil {
			return err
		}
	} else {
		units, err = h.unitService.ListUnits(c.Request().Context(), request.Limit, offset, request.Sort, request.Order, request.UnitType, request.BaseOnly)
		if err != nil {
			return err
		}
		total, err = h.unitService.CountUnits(c.Request().Context(), request.UnitType, request.BaseOnly)
		if err != nil {
			return err
		}
	}

	totalPages := int((total + int64(request.Limit) - 1) / int64(request.Limit))

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data":       units,
		"total":      total,
		"page":       request.Page,
		"limit":      request.Limit,
		"totalPages": totalPages,
	})
}

// GetUnit godoc
// @Summary Get unit by ID
// @Description Retrieve a single unit by its ID
// @Tags units
// @Produce json
// @Param id path int true "Unit ID"
// @Success 200 {object} models.Unit
// @Failure 404 {object} map[string]string
// @Security BearerAuth
// @Router /units/{id} [get]
func (h *UnitHandler) GetUnit(c echo.Context) error {
	idParam := c.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		return pkg.ErrValidation("unit id must be a positive integer", err)
	}

	unit, err := h.unitService.GetUnitByID(c.Request().Context(), uint(id))
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, unit)
}

type createUnitRequest struct {
	UnitType         string  `json:"unit_type"`
	Name             string  `json:"name" validate:"required"`
	Symbol           string  `json:"symbol"`
	BaseUnitID       *uint   `json:"base_unit_id,omitempty"`
	ConversionFactor float64 `json:"conversion_factor,omitempty" validate:"required,gt=0"`
}

// CreateUnit godoc
// @Summary Create a new unit
// @Description Create a measurement unit definition
// @Tags units
// @Accept json
// @Produce json
// @Param unit body createUnitRequest true "Unit information"
// @Success 201 {object} models.Unit
// @Failure 400 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /units [post]
func (h *UnitHandler) CreateUnit(c echo.Context) error {
	var request createUnitRequest
	if err := c.Bind(&request); err != nil {
		return pkg.ErrInvalidRequestBody(err)
	}

	if err := pkg.Validator.Struct(request); err != nil {
		return pkg.ErrValidation(err.Error(), err)
	}

	unit := &models.Unit{
		UnitType:         request.UnitType,
		Name:             request.Name,
		Symbol:           request.Symbol,
		BaseUnitID:       request.BaseUnitID,
		ConversionFactor: request.ConversionFactor,
	}

	if unit.Symbol == "" {
		unit.Symbol = unit.Name
	}

	if err := h.unitService.CreateUnit(c.Request().Context(), unit); err != nil {
		return err
	}

	return c.JSON(http.StatusCreated, unit)
}

// UpdateUnit godoc
// @Summary Update an existing unit
// @Description Update a measurement unit definition
// @Tags units
// @Accept json
// @Produce json
// @Param id path int true "Unit ID"
// @Param unit body createUnitRequest true "Updated unit information"
// @Success 200 {object} models.Unit
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /units/{id} [put]
func (h *UnitHandler) UpdateUnit(c echo.Context) error {
	id, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	var request createUnitRequest
	if err := c.Bind(&request); err != nil {
		return pkg.ErrInvalidRequestBody(err)
	}

	unit := &models.Unit{
		Base: models.Base{
			ID: id,
		},
		UnitType:         request.UnitType,
		Name:             request.Name,
		Symbol:           request.Symbol,
		BaseUnitID:       request.BaseUnitID,
		ConversionFactor: request.ConversionFactor,
	}

	if err := h.unitService.UpdateUnit(c.Request().Context(), unit); err != nil {
		return err
	}

	return c.JSON(http.StatusOK, unit)
}

// DeleteUnit godoc
// @Summary Delete a unit
// @Description Soft delete a measurement unit
// @Tags units
// @Produce json
// @Param id path int true "Unit ID"
// @Success 204 "Unit deleted successfully"
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /units/{id} [delete]
func (h *UnitHandler) DeleteUnit(c echo.Context) error {
	id, err := pkg.ExtractIDParam(c)
	if err != nil {
		return err
	}

	if err := h.unitService.DeleteUnit(c.Request().Context(), id); err != nil {
		return err
	}

	return c.NoContent(http.StatusNoContent)
}
