package excel

import (
	"fmt"
)

func InventoryDefaultFileConfig() FileConfig {
	return FileConfig{
		FilePath: "",
		SheetConfigs: []SheetConfig{
			{InternalID: "inventory_change", NamePattern: "THANG {MM}", HeaderStartRow: 3, HeaderStartCol: 1, HeaderHeight: 3},
		},
	}
}

type InventoryRepository interface {
}

type inventoryRepository struct {
	file *File
}

func NewInventoryRepository(config FileConfig) (InventoryRepository, error) {
	file, err := NewFile(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create inventory repository: %w", err)
	}
	return &inventoryRepository{
		file: file,
	}, nil
}
