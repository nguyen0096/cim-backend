package dto

// InventoryItemSortField represents sort field options for inventory items
type InventoryItemSortField string

const (
	// InventoryItemSortFieldProductName sorts by product name
	InventoryItemSortFieldProductName InventoryItemSortField = "product_name"
	// InventoryItemSortFieldUpdatedAt sorts by updated_at
	InventoryItemSortFieldUpdatedAt InventoryItemSortField = "updated_at"
	// InventoryItemSortFieldCreatedAt sorts by created_at
	InventoryItemSortFieldCreatedAt InventoryItemSortField = "created_at"
	// InventoryItemSortFieldQuantity sorts by quantity
	InventoryItemSortFieldQuantity InventoryItemSortField = "quantity"
)

// GetInventoryItemSortFields returns all available sort fields
func GetInventoryItemSortFields() []string {
	return []string{
		string(InventoryItemSortFieldUpdatedAt),
		string(InventoryItemSortFieldCreatedAt),
		string(InventoryItemSortFieldQuantity),
		string(InventoryItemSortFieldProductName),
	}
}
