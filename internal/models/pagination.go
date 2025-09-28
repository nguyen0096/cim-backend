package models

const (
	// Pagination defaults
	DefaultPage      = 1
	DefaultLimit     = 20
	MaxLimit         = 100
	DefaultSortOrder = "asc"
)

// PaginationParams represents pagination and filtering parameters
type PaginationParams struct {
	Page      int    `json:"page" query:"page"`
	Limit     int    `json:"limit" query:"limit"`
	Search    string `json:"q" query:"q"`
	Sort      string `json:"sort" query:"sort"`
	Order     string `json:"order" query:"order"`
	StartDate string `json:"start_date" query:"start_date" example:"2023-01-01"`
	EndDate   string `json:"end_date" query:"end_date" example:"2023-12-31"`
}

// PaginationResult represents paginated response
// @Description Generic pagination result with data and metadata
type PaginationResult[T any] struct {
	Data       []T   `json:"data"`
	Total      int64 `json:"total" example:"100"`
	Page       int   `json:"page" example:"1"`
	Limit      int   `json:"limit" example:"20"`
	TotalPages int   `json:"totalPages" example:"5"`
}

// NewPaginationResult creates a new pagination result
func NewPaginationResult[T any](data []T, total int64, page, limit int) *PaginationResult[T] {
	totalPages := int((total + int64(limit) - 1) / int64(limit))
	return &PaginationResult[T]{
		Data:       data,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}
}

// ValidateAndSetDefaults validates and sets default values for pagination parameters
func (p *PaginationParams) ValidateAndSetDefaults() {
	if p.Page <= 0 {
		p.Page = DefaultPage
	}
	if p.Limit <= 0 {
		p.Limit = DefaultLimit
	}
	if p.Limit > MaxLimit {
		p.Limit = MaxLimit
	}
	if p.Order != "asc" && p.Order != "desc" {
		p.Order = DefaultSortOrder
	}
}

// GetOffset calculates the offset for database queries
func (p *PaginationParams) GetOffset() int {
	return (p.Page - 1) * p.Limit
}
