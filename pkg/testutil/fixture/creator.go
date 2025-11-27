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
func WithUnits(db *gorm.DB, units []*models.Unit) []*models.Unit {
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

	if err := db.Create(units).Error; err != nil {
		panic(fmt.Sprintf("failed to create test units: %v", err))
	}
	DeferCleanup(func() {
		ids := make([]uint, len(units))
		for i, unit := range units {
			ids[i] = unit.ID
		}
		// Delete product_suppliers for products that reference these units
		db.Exec("DELETE FROM product_suppliers WHERE product_id IN (SELECT id FROM products WHERE unit_id IN (?))", ids)
		// Note: purchase_order_items are deleted by WithPurchaseOrders cleanup
		// Delete products that reference these units
		db.Exec("DELETE FROM products WHERE unit_id IN (?)", ids)
		// Then delete units
		db.Exec("DELETE FROM units WHERE id IN (?)", ids)
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
func WithProducts(db *gorm.DB, products []*models.Product) []*models.Product {
	// reset the id incase the client set it.
	// let the DB generate the id.
	for i := range products {
		products[i].ID = 0
	}

	if err := db.Create(&products).Error; err != nil {
		panic(fmt.Sprintf("failed to create test products: %v", err))
	}
	DeferCleanup(func() {
		ids := make([]uint, len(products))
		for i, product := range products {
			ids[i] = product.ID
		}
		// Delete product_suppliers junction table first
		db.Exec("DELETE FROM product_suppliers WHERE product_id IN (?)", ids)
		// Note: purchase_order_items are deleted by WithPurchaseOrders cleanup
		// Then delete products (purchase_order_items should already be deleted)
		db.Exec("DELETE FROM products WHERE id IN (?)", ids)
	})

	return products
}

func WithSuppliers(db *gorm.DB, suppliers []*models.Supplier) []*models.Supplier {
	// reset the id incase the client set it.
	// let the DB generate the id.
	for i := range suppliers {
		suppliers[i].ID = 0
	}

	if err := db.Create(&suppliers).Error; err != nil {
		panic(fmt.Sprintf("failed to create test suppliers: %v", err))
	}
	DeferCleanup(func() {
		ids := make([]uint, len(suppliers))
		for i, supplier := range suppliers {
			ids[i] = supplier.ID
		}
		// Delete product_suppliers junction table first
		db.Exec("DELETE FROM product_suppliers WHERE supplier_id IN (?)", ids)
		// Note: purchase_order_items are deleted by WithPurchaseOrders cleanup
		// Then delete suppliers (purchase_order_items should already be deleted)
		db.Exec("DELETE FROM suppliers WHERE id IN (?)", ids)
	})
	return suppliers
}

func WithPurchaseOrders(db *gorm.DB, purchaseOrders []*models.PurchaseOrder) []*models.PurchaseOrder {
	// reset the id incase the client set it.
	// let the DB generate the id.
	for i := range purchaseOrders {
		purchaseOrders[i].ID = 0
	}

	if err := db.Create(&purchaseOrders).Error; err != nil {
		panic(fmt.Sprintf("failed to create test purchase orders: %v", err))
	}
	DeferCleanup(func() {
		ids := make([]uint, len(purchaseOrders))
		for i, purchaseOrder := range purchaseOrders {
			ids[i] = purchaseOrder.ID
		}
		// Delete purchase order items first (they reference purchase_orders)
		db.Exec("DELETE FROM purchase_order_items WHERE purchase_order_id IN (?)", ids)
		// Then delete purchase orders
		db.Exec("DELETE FROM purchase_orders WHERE id IN (?)", ids)
	})
	return purchaseOrders
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
		// Note: purchase_orders and purchase_order_items are deleted by WithPurchaseOrders cleanup
		// Just delete the inventory (purchase_orders should already be deleted)
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
		// Delete children first due to foreign key constraints
		db.Exec("DELETE FROM payment_receipt_forms WHERE purchase_order_id = ?", purchaseOrder.ID)
		db.Exec("DELETE FROM purchase_order_items WHERE purchase_order_id = ?", purchaseOrder.ID)
		db.Exec("DELETE FROM purchase_orders WHERE id = ?", purchaseOrder.ID)
	})
	return &purchaseOrder
}

// WithPaymentReceiptForm creates a test payment receipt form and returns it
func WithPaymentReceiptForms(db *gorm.DB, paymentReceiptForms []*models.PaymentReceiptForm) []*models.PaymentReceiptForm {
	if err := db.Create(&paymentReceiptForms).Error; err != nil {
		panic(fmt.Sprintf("failed to create test payment receipt form: %v", err))
	}
	DeferCleanup(func() {
		// Only clean up payment receipt forms
		// Nested entities (purchase orders, products, suppliers, etc.) are cleaned up by their own fixture functions
		ids := make([]uint, len(paymentReceiptForms))
		for i, paymentReceiptForm := range paymentReceiptForms {
			ids[i] = paymentReceiptForm.ID
		}
		db.Exec("DELETE FROM payment_receipt_forms WHERE id IN (?)", ids)
	})
	return paymentReceiptForms
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
