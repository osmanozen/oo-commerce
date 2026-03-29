package persistence

// PaginationParams holds pagination request parameters.
type PaginationParams struct {
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
}

// NewPaginationParams creates validated pagination parameters.
func NewPaginationParams(page, pageSize int) PaginationParams {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 12
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return PaginationParams{Page: page, PageSize: pageSize}
}

// Offset calculates the SQL OFFSET value.
func (p PaginationParams) Offset() int {
	return (p.Page - 1) * p.PageSize
}

// Limit returns the SQL LIMIT value.
func (p PaginationParams) Limit() int {
	return p.PageSize
}

// PagedResult is a generic paginated response.
type PagedResult[T any] struct {
	Items      []T `json:"items"`
	TotalCount int `json:"totalCount"`
	Page       int `json:"page"`
	PageSize   int `json:"pageSize"`
	TotalPages int `json:"totalPages"`
}

// NewPagedResult creates a paginated result with calculated total pages.
func NewPagedResult[T any](items []T, totalCount int, params PaginationParams) PagedResult[T] {
	totalPages := totalCount / params.PageSize
	if totalCount%params.PageSize > 0 {
		totalPages++
	}
	return PagedResult[T]{
		Items:      items,
		TotalCount: totalCount,
		Page:       params.Page,
		PageSize:   params.PageSize,
		TotalPages: totalPages,
	}
}
