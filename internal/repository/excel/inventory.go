package excel

import (
	"fmt"
	"import-export-backend/internal/models"
	"time"
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
	UpsertInventoryChangeByDate(p *models.Product, quantity int, date time.Time) error
	// InitializeWithFile(ctx context.Context, filePath string, sheetNames ...string) error
	// AddInventoryEntry(ctx context.Context, sheetName string, inventoryData map[string]interface{}) error
	// GetLastInventoryEntry(ctx context.Context, sheetName string) (map[string]interface{}, error)
	// GetLastTransactionDate(ctx context.Context, sheetName string) (time.Time, error)
	// GetSchema(ctx context.Context) *models.FileMetadata
	// Close() error
	// ForceCacheRefresh()
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

func (r *inventoryRepository) UpsertInventoryChangeByDate(p *models.Product, quantity int, date time.Time) error {
	return nil
}
