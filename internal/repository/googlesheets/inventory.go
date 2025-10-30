package googlesheets

type InventoryGoogleSheetsRepository interface {
}

// inventoryGoogleSheetsRepository implements InventoryGoogleSheetsRepository
type inventoryGoogleSheetsRepository struct {
}

// NewInventoryGoogleSheetsRepository creates a new InventoryGoogleSheetsRepository
func NewInventoryGoogleSheetsRepository() InventoryGoogleSheetsRepository {
	return &inventoryGoogleSheetsRepository{}
}
