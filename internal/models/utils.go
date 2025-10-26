package models

var (
	_ IDGetter = (*InventoryItem)(nil)
	_ IDGetter = (*InventoryItemChange)(nil)
)

// IDGetter interface defines a method to get an ID as string
type IDGetter interface {
	GetID() uint
}

// GetIDs extracts IDs from a slice of items by casting them to IDGetter interface
func GetIDs[T any](items []T) []uint {
	ids := make([]uint, 0, len(items))

	for _, item := range items {
		// Try to cast item to IDGetter interface
		getter, ok := any(item).(IDGetter)
		if !ok {
			// Skip if item doesn't implement IDGetter
			continue
		}

		id := getter.GetID()
		if id == 0 {
			continue
		}
		ids = append(ids, id)
	}

	return ids
}

// BuildIDMap builds a map of items by ID.
func BuildIDMap[T any](items []T) map[uint]T {
	idMap := make(map[uint]T)
	for _, item := range items {
		getter, ok := any(item).(IDGetter)
		if !ok {
			continue
		}

		id := getter.GetID()
		if id == 0 {
			continue
		}

		idMap[id] = item
	}
	return idMap
}
