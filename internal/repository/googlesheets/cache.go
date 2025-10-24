package googlesheets

// Cache holds all cache-related data for Google Sheets repository operations
type Cache struct {
	rows      [][]interface{}
	headerRow int
	valid     bool
}

// NewCache creates a new cache instance with default values
func NewCache() *Cache {
	return &Cache{
		rows:      nil,
		headerRow: -1,
		valid:     false,
	}
}
