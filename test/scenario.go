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

// WithUnit creates a test unit and returns it
func (t *TestEnv) WithUnit(unit models.Unit) *models.Unit {
	ctx := t.DefaultContext
	if err := t.DB.WithContext(ctx).Create(&unit).Error; err != nil {
		panic(fmt.Sprintf("failed to create test unit: %v", err))
	}
	DeferCleanup(func(ctx context.Context) {
		t.DB.WithContext(ctx).Exec("DELETE FROM units WHERE id = ?", unit.ID)
	})
	return &unit
}

// WithUnits creates multiple test units and returns them
func (t *TestEnv) WithUnits(units []models.Unit) []models.Unit {
	ctx := t.DefaultContext
	if err := t.DB.WithContext(ctx).Create(&units).Error; err != nil {
		panic(fmt.Sprintf("failed to create test units: %v", err))
	}
	DeferCleanup(func(ctx context.Context) {
		// Delete in reverse order to avoid foreign key constraint violations
		for i := len(units) - 1; i >= 0; i-- {
			t.DB.WithContext(ctx).Exec("DELETE FROM units WHERE id = ?", units[i].ID)
		}
	})
	return units
}
