package apptest

import (
	"cim-backend/internal/models"
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
)

// WithSupplier creates a test supplier and returns it
func (t *TestEnv) WithSupplier(supplier models.Supplier) *models.Supplier {
	ctx := t.DefaultContext
	if err := t.DB.WithContext(ctx).Create(&supplier).Error; err != nil {
		panic(fmt.Sprintf("failed to create test supplier: %v", err))
	}
	DeferCleanup(func(ctx context.Context) {
		t.DB.WithContext(ctx).Exec("DELETE FROM suppliers WHERE id = ?", supplier.ID)
	})
	return &supplier
}
