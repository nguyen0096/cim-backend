package excel

// Cache holds all cache-related data for Excel repository operations
type Cache struct {
	rows      [][]string
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
