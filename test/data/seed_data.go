package data

import (
	"import-export-backend/internal/models"
	"time"
)

// Suppliers contains all test supplier data
func Suppliers() []models.Supplier {
	now := time.Now()

	return []models.Supplier{
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:         "Tech Electronics Inc",
			ContactEmail: "contact@techelectronics.com",
			ContactPhone: "+1-555-0123",
			Address:      "123 Silicon Valley Blvd, San Jose, CA 95110",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:         "Office Supply Co",
			ContactEmail: "sales@officesupply.com",
			ContactPhone: "+1-555-0456",
			Address:      "456 Business Park Dr, Dallas, TX 75201",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:         "Global Parts Ltd",
			ContactEmail: "orders@globalparts.com",
			ContactPhone: "+1-555-0789",
			Address:      "789 Industrial Way, Seattle, WA 98101",
		},
	}
}

// Products contains all test product data
func Products(supplierIDs []uint) []models.Product {
	now := time.Now()

	return []models.Product{
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "MacBook Pro 16-inch M3",
			Description: "Professional laptop with M3 chip, 32GB RAM, 1TB SSD",
			ProductType: "laptop",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "LG UltraGear 27\" 4K Gaming Monitor",
			Description: "27-inch 4K UHD gaming monitor with 144Hz refresh rate",
			ProductType: "monitor",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Keychron K8 Mechanical Keyboard",
			Description: "Wireless mechanical keyboard with RGB backlight and hot-swappable switches",
			ProductType: "keyboard",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Logitech MX Master 3S",
			Description: "Advanced wireless mouse with precision scrolling and customizable buttons",
			ProductType: "mouse",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Herman Miller Aeron Chair",
			Description: "Ergonomic office chair with lumbar support and breathable mesh",
			ProductType: "chair",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "UPLIFT Standing Desk 60x30",
			Description: "Height-adjustable standing desk with bamboo top and memory settings",
			ProductType: "desk",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "CalDigit TS4 Thunderbolt 4 Hub",
			Description: "18-port Thunderbolt 4 hub with 98W charging and 40Gbps data transfer",
			ProductType: "hub",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Logitech Brio 4K Webcam",
			Description: "Ultra HD 4K webcam with HDR and Windows Hello support",
			ProductType: "headphones",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Sony WH-1000XM5 Headphones",
			Description: "Industry-leading noise cancelling wireless headphones with 30-hour battery",
			ProductType: "headphones",
			Status:      "active",
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "iPad Pro 12.9\" M2",
			Description: "12.9-inch iPad Pro with M2 chip, 256GB storage, and Liquid Retina XDR display",
			ProductType: "tablet",
			Status:      "active",
		},
	}
}

// InventoryData represents inventory configuration for a product
type InventoryData struct {
	ProductID    uint
	SupplierID   uint
	UnitPrice    float64
	UnitType     string
	Quantity     int
	ReorderLevel int
	Location     string
}

// InventoryConfigs contains inventory configuration for all products
var InventoryConfigs = []InventoryData{
	{1, 1, 2999.99, "piece", 15, 5, "Warehouse A"},
	{2, 1, 599.99, "piece", 30, 10, "Warehouse B"},
	{3, 1, 89.99, "piece", 45, 15, "Warehouse C"},
	{4, 1, 99.99, "piece", 25, 8, "Warehouse A"},
	{5, 2, 1395.00, "piece", 8, 3, "Warehouse B"},
	{6, 2, 799.00, "box", 12, 5, "Warehouse C"},
	{7, 3, 399.95, "piece", 20, 8, "Warehouse A"},
	{8, 3, 199.99, "piece", 35, 12, "Warehouse B"},
	{9, 1, 399.99, "piece", 18, 6, "Warehouse C"},
	{10, 1, 1099.99, "device", 22, 8, "Warehouse A"},
}

// Inventory contains all test inventory data
func Inventory(productIDs []uint) []models.Inventory {
	now := time.Now()

	// Create inventory locations
	inventories := []models.Inventory{
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Main Warehouse A",
			Description: "Primary storage facility for electronics and office supplies",
			Location:    "123 Industrial Blvd, San Francisco, CA 94107",
			Status:      models.InventoryStatusActive,
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Secondary Warehouse B",
			Description: "Secondary storage facility for bulk items",
			Location:    "456 Storage Way, Oakland, CA 94607",
			Status:      models.InventoryStatusActive,
		},
		{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Distribution Center C",
			Description: "Distribution center for fast-moving items",
			Location:    "789 Logistics Ave, San Jose, CA 95110",
			Status:      models.InventoryStatusActive,
		},
	}

	return inventories
}

// InventoryItems contains all test inventory item data
func InventoryItems(productIDs []uint) []models.InventoryItem {
	now := time.Now()

	items := make([]models.InventoryItem, len(InventoryConfigs))
	for i, config := range InventoryConfigs {
		// Map product to inventory based on location
		var inventoryID uint
		switch config.Location {
		case "Warehouse A":
			inventoryID = 1
		case "Warehouse B":
			inventoryID = 2
		case "Warehouse C":
			inventoryID = 3
		default:
			inventoryID = 1 // Default to first inventory
		}

		items[i] = models.InventoryItem{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			InventoryID:   inventoryID,
			ProductID:     config.ProductID,
			SupplierID:    config.SupplierID,
			UnitPrice:     config.UnitPrice,
			UnitType:      config.UnitType,
			Quantity:      config.Quantity,
			ReorderLevel:  config.ReorderLevel,
			MaxStockLevel: config.Quantity * 3, // Set max stock to 3x current quantity
			Status:        models.InventoryItemStatusActive,
		}
	}

	return items
}
