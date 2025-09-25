package pkg

type PaginationQuery struct {
	Limit int `json:"limit"`
	Page  int `json:"offset"`
}
