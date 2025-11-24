package fixture

import (
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"

	"cim-backend/internal/models"
)

// WithSupplier creates a test supplier and returns it
func WithSupplier(db *gorm.DB, supplier models.Supplier) *models.Supplier {
	// reset the id incase the client set it.
	// let the DB generate the id.
	supplier.ID = 0

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
	// MATCHING APP CREATION LOGIC
	// uppercase the unit name
	unit.Name = strings.ToUpper(unit.Name)

	// reset the id incase the client set it.
	// let the DB generate the id.
	unit.ID = 0

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
	// MATCHING APP CREATION LOGIC
	// uppercase the unit name
	for i := range units {
		units[i].Name = strings.ToUpper(units[i].Name)
	}

	// reset the id incase the client set it.
	// let the DB generate the id.
	for i := range units {
		units[i].ID = 0
	}

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
	// reset the id incase the client set it.
	// let the DB generate the id.
	product.ID = 0

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
	// reset the id incase the client set it.
	// let the DB generate the id.
	for i := range products {
		products[i].ID = 0
	}

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
	// reset the id incase the client set it.
	// let the DB generate the id.
	inventory.ID = 0

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
	// reset the id incase the client set it.
	// let the DB generate the id.
	purchaseOrder.ID = 0

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

// CreatePurchaseOrderExcelFile creates a test Excel file for purchase order import
// The file is saved to the repository root and will be automatically cleaned up after the test
func CreatePurchaseOrderExcelFile(
	tempDir string,
	supplier *models.Supplier,
	product *models.Product,
	unit *models.Unit,
	inventory *models.Inventory,
) string {
	// Create new Excel file
	f := excelize.NewFile()
	sheetName := "Sheet1"

	// Set cell A1: Inventory Name
	if err := f.SetCellValue(sheetName, "A1", inventory.Name); err != nil {
		panic(fmt.Sprintf("failed to set inventory name: %v", err))
	}

	// Set cell E1: Date in format "NGÀY: DD/MM/YYYY"
	currentDate := time.Now().Format("02/01/2006")
	dateValue := fmt.Sprintf("NGÀY: %s", currentDate)
	if err := f.SetCellValue(sheetName, "E1", dateValue); err != nil {
		panic(fmt.Sprintf("failed to set date: %v", err))
	}

	// Set cell A3: Supplier Name
	if err := f.SetCellValue(sheetName, "A3", supplier.Name); err != nil {
		panic(fmt.Sprintf("failed to set supplier name: %v", err))
	}

	// Set row 4 headers: STT, DIỄN GIẢI, ĐVT, SỐ LƯỢNG, ĐƠN GIÁ, TT
	headers := []string{"STT", "DIỄN GIẢI", "ĐVT", "SỐ LƯỢNG", "ĐƠN GIÁ", "TT"}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 4)
		if err := f.SetCellValue(sheetName, cell, header); err != nil {
			panic(fmt.Sprintf("failed to set header %s: %v", header, err))
		}
	}

	// Set row 5 with one valid data row
	// Columns: Number, Product Name, Unit, Quantity, Unit Price, Total (formula)
	row5Data := []interface{}{
		1,            // STT
		product.Name, // DIỄN GIẢI
		unit.Name,    // ĐVT
		10,           // SỐ LƯỢNG
		1000,         // ĐƠN GIÁ
		"=D5*E5",     // TT (formula: quantity * unit price)
	}
	for i, value := range row5Data {
		cell, _ := excelize.CoordinatesToCellName(i+1, 5)
		if i == 5 { // Formula column
			if err := f.SetCellFormula(sheetName, cell, value.(string)); err != nil {
				panic(fmt.Sprintf("failed to set formula: %v", err))
			}
		} else {
			if err := f.SetCellValue(sheetName, cell, value); err != nil {
				panic(fmt.Sprintf("failed to set cell value: %v", err))
			}
		}
	}

	filename := fmt.Sprintf("test-po-import-%s.xlsx", uuid.New().String())
	filepath := path.Join(tempDir, filename)

	// Save file
	if err := f.SaveAs(filepath); err != nil {
		panic(fmt.Sprintf("failed to save Excel file: %v", err))
	}

	// Print file path to console
	GinkgoWriter.Printf("Created test Excel file: %s\n", filepath)

	// Register cleanup to delete file after test
	DeferCleanup(func() {
		GinkgoWriter.Printf("Cleaning up test Excel file: %s\n", filepath)
		if err := os.Remove(filepath); err != nil {
			GinkgoWriter.Printf("Warning: failed to delete test file %s: %v\n", filepath, err)
		}
	})

	return filepath
}
