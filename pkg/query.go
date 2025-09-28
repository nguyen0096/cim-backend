package pkg

import (
	"strconv"

	"github.com/labstack/echo/v4"
)

type PaginationQuery struct {
	Limit int `json:"limit"`
	Page  int `json:"offset"`
}

// ExtractIDParam extracts and converts the "id" parameter from the request to uint
func ExtractIDParam(c echo.Context) (uint, error) {
	idStr := c.Param("id")
	idInt, err := strconv.Atoi(idStr)
	if err != nil {
		return 0, ErrValidation("Invalid ID format", err)
	}
	return uint(idInt), nil
}
