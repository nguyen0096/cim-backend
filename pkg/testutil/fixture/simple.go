package fixture

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	"gorm.io/gorm"

	"cim-backend/internal/models"
)

// WithSupplier creates a test supplier and returns it
func WithSupplier(db *gorm.DB, supplier models.Supplier) *models.Supplier {
	if err := db.Create(&supplier).Error; err != nil {
		panic(fmt.Sprintf("failed to create test supplier: %v", err))
	}
	DeferCleanup(func() {
		db.Exec("DELETE FROM suppliers WHERE id = ?", supplier.ID)
	})
	return &supplier
}

// WithUnit creates a test unit and returns it
func WithUnit(db *gorm.DB, unit models.Unit) *models.Unit {
	if err := db.Create(&unit).Error; err != nil {
		panic(fmt.Sprintf("failed to create test unit: %v", err))
	}
	DeferCleanup(func() {
		db.Exec("DELETE FROM units WHERE id = ?", unit.ID)
	})
	return &unit
}

// WithUnits creates multiple test units and returns them
func WithUnits(db *gorm.DB, units []models.Unit) []models.Unit {
	if err := db.Create(&units).Error; err != nil {
		panic(fmt.Sprintf("failed to create test units: %v", err))
	}
	DeferCleanup(func() {
		// Delete in reverse order to avoid foreign key constraint violations
		for i := len(units) - 1; i >= 0; i-- {
			db.Exec("DELETE FROM units WHERE id = ?", units[i].ID)
		}
	})
	return units
}

// WithProduct creates a test product and returns it
func WithProduct(db *gorm.DB, product models.Product) *models.Product {
	if err := db.Create(&product).Error; err != nil {
		panic(fmt.Sprintf("failed to create test product: %v", err))
	}
	DeferCleanup(func() {
		db.Exec("DELETE FROM products WHERE id = ?", product.ID)
	})
	return &product
}

// WithProducts creates multiple test products and returns them
func WithProducts(db *gorm.DB, products []models.Product) []models.Product {
	if err := db.Create(&products).Error; err != nil {
		panic(fmt.Sprintf("failed to create test products: %v", err))
	}
	DeferCleanup(func() {
		for _, product := range products {
			db.Exec("DELETE FROM products WHERE id = ?", product.ID)
		}
	})
	return products
}

// WithInventory creates a test inventory and returns it
func WithInventory(db *gorm.DB, inventory models.Inventory) *models.Inventory {
	if err := db.Create(&inventory).Error; err != nil {
		panic(fmt.Sprintf("failed to create test inventory: %v", err))
	}
	DeferCleanup(func() {
		db.Exec("DELETE FROM inventories WHERE id = ?", inventory.ID)
	})
	return &inventory
}

// WithPurchaseOrder creates a test purchase order and returns it
func WithPurchaseOrder(db *gorm.DB, purchaseOrder models.PurchaseOrder) *models.PurchaseOrder {
	if err := db.Create(&purchaseOrder).Error; err != nil {
		panic(fmt.Sprintf("failed to create test purchase order: %v", err))
	}
	DeferCleanup(func() {
		// Delete items first due to foreign key constraints
		db.Exec("DELETE FROM purchase_order_items WHERE purchase_order_id = ?", purchaseOrder.ID)
		db.Exec("DELETE FROM purchase_orders WHERE id = ?", purchaseOrder.ID)
	})
	return &purchaseOrder
}
