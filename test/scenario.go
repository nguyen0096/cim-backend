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

// WithProduct creates a test product and returns it
func (t *TestEnv) WithProduct(product models.Product) *models.Product {
	ctx := t.DefaultContext
	if err := t.DB.WithContext(ctx).Create(&product).Error; err != nil {
		panic(fmt.Sprintf("failed to create test product: %v", err))
	}
	DeferCleanup(func(ctx context.Context) {
		t.DB.WithContext(ctx).Exec("DELETE FROM products WHERE id = ?", product.ID)
	})
	return &product
}

// WithProducts creates multiple test products and returns them
func (t *TestEnv) WithProducts(products []models.Product) []models.Product {
	ctx := t.DefaultContext
	if err := t.DB.WithContext(ctx).Create(&products).Error; err != nil {
		panic(fmt.Sprintf("failed to create test products: %v", err))
	}
	DeferCleanup(func(ctx context.Context) {
		for _, product := range products {
			t.DB.WithContext(ctx).Exec("DELETE FROM products WHERE id = ?", product.ID)
		}
	})
	return products
}

// WithInventory creates a test inventory and returns it
func (t *TestEnv) WithInventory(inventory models.Inventory) *models.Inventory {
	ctx := t.DefaultContext
	if err := t.DB.WithContext(ctx).Create(&inventory).Error; err != nil {
		panic(fmt.Sprintf("failed to create test inventory: %v", err))
	}
	DeferCleanup(func(ctx context.Context) {
		t.DB.WithContext(ctx).Exec("DELETE FROM inventories WHERE id = ?", inventory.ID)
	})
	return &inventory
}

// WithPurchaseOrder creates a test purchase order and returns it
func (t *TestEnv) WithPurchaseOrder(purchaseOrder models.PurchaseOrder) *models.PurchaseOrder {
	ctx := t.DefaultContext
	if err := t.DB.WithContext(ctx).Create(&purchaseOrder).Error; err != nil {
		panic(fmt.Sprintf("failed to create test purchase order: %v", err))
	}
	DeferCleanup(func(ctx context.Context) {
		// Delete items first due to foreign key constraints
		t.DB.WithContext(ctx).Exec("DELETE FROM purchase_order_items WHERE purchase_order_id = ?", purchaseOrder.ID)
		t.DB.WithContext(ctx).Exec("DELETE FROM purchase_orders WHERE id = ?", purchaseOrder.ID)
	})
	return &purchaseOrder
}
