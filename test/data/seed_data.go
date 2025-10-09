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
			InventoryID: inventoryID,
			ProductID:   config.ProductID,
			SupplierID:  config.SupplierID,
			UnitPrice:   config.UnitPrice,
			UnitType:    config.UnitType,
			Quantity:    config.Quantity,
			Status:      models.InventoryItemStatusActive,
		}
	}

	return items
}

// PurchaseOrders contains all test purchase order data
func PurchaseOrders(productIDs []uint, supplierIDs []uint) []models.PurchaseOrder {
	now := time.Now()

	// If no supplierIDs provided, use default supplier ID 1 for all
	if len(supplierIDs) == 0 {
		supplierIDs = []uint{1, 2, 3}
	}

	return []models.PurchaseOrder{
		{
			Base: models.Base{
				CreatedAt: now.AddDate(0, 0, -30), // 30 days ago
				UpdatedAt: now.AddDate(0, 0, -30),
			},
			OrderNumber: "PO-2024-001",
			Status:      models.PurchaseOrderStatusCompleted,
			Notes:       "Q1 electronics restock order",
			Items: []*models.PurchaseOrderItem{
				{
					Base: models.Base{
						CreatedAt: now.AddDate(0, 0, -30),
						UpdatedAt: now.AddDate(0, 0, -30),
					},
					ProductID:        &productIDs[0], // MacBook Pro
					SupplierID:       &supplierIDs[0],
					UnitPrice:        2999.99,
					Quantity:         5,
					ReceivedQuantity: 5,
					Status:           models.PurchaseOrderItemStatusDelivered,
				},
				{
					Base: models.Base{
						CreatedAt: now.AddDate(0, 0, -30),
						UpdatedAt: now.AddDate(0, 0, -30),
					},
					ProductID:        &productIDs[1], // LG Monitor
					SupplierID:       &supplierIDs[0],
					UnitPrice:        599.99,
					Quantity:         10,
					ReceivedQuantity: 10,
					Status:           models.PurchaseOrderItemStatusDelivered,
				},
			},
		},
		{
			Base: models.Base{
				CreatedAt: now.AddDate(0, 0, -15), // 15 days ago
				UpdatedAt: now.AddDate(0, 0, -5),
			},
			OrderNumber: "PO-2024-002",
			Status:      models.PurchaseOrderStatusPartiallyDelivered,
			Notes:       "Office furniture and accessories order",
			Items: []*models.PurchaseOrderItem{
				{
					Base: models.Base{
						CreatedAt: now.AddDate(0, 0, -15),
						UpdatedAt: now.AddDate(0, 0, -5),
					},
					ProductID:        &productIDs[4], // Herman Miller Chair
					SupplierID:       &supplierIDs[1],
					UnitPrice:        1395.00,
					Quantity:         3,
					ReceivedQuantity: 2,
					Status:           models.PurchaseOrderItemStatusDelivering,
				},
				{
					Base: models.Base{
						CreatedAt: now.AddDate(0, 0, -15),
						UpdatedAt: now.AddDate(0, 0, -5),
					},
					ProductID:        &productIDs[5], // UPLIFT Desk
					SupplierID:       &supplierIDs[1],
					UnitPrice:        799.00,
					Quantity:         2,
					ReceivedQuantity: 2,
					Status:           models.PurchaseOrderItemStatusDelivered,
				},
			},
		},
		{
			Base: models.Base{
				CreatedAt: now.AddDate(0, 0, -7), // 7 days ago
				UpdatedAt: now.AddDate(0, 0, -7),
			},
			OrderNumber: "PO-2024-003",
			Status:      models.PurchaseOrderStatusOrderPlaced,
			Notes:       "Peripheral devices for new office setup",
			Items: []*models.PurchaseOrderItem{
				{
					Base: models.Base{
						CreatedAt: now.AddDate(0, 0, -7),
						UpdatedAt: now.AddDate(0, 0, -7),
					},
					ProductID:        &productIDs[2], // Keychron Keyboard
					SupplierID:       &supplierIDs[0],
					UnitPrice:        89.99,
					Quantity:         20,
					ReceivedQuantity: 0,
					Status:           models.PurchaseOrderItemStatusDelivering,
				},
				{
					Base: models.Base{
						CreatedAt: now.AddDate(0, 0, -7),
						UpdatedAt: now.AddDate(0, 0, -7),
					},
					ProductID:        &productIDs[3], // Logitech Mouse
					SupplierID:       &supplierIDs[0],
					UnitPrice:        99.99,
					Quantity:         15,
					ReceivedQuantity: 0,
					Status:           models.PurchaseOrderItemStatusDelivering,
				},
				{
					Base: models.Base{
						CreatedAt: now.AddDate(0, 0, -7),
						UpdatedAt: now.AddDate(0, 0, -7),
					},
					ProductID:        &productIDs[8], // Sony Headphones
					SupplierID:       &supplierIDs[0],
					UnitPrice:        399.99,
					Quantity:         8,
					ReceivedQuantity: 0,
					Status:           models.PurchaseOrderItemStatusDelivering,
				},
			},
		},
		{
			Base: models.Base{
				CreatedAt: now.AddDate(0, 0, -3), // 3 days ago
				UpdatedAt: now.AddDate(0, 0, -1),
			},
			OrderNumber: "PO-2024-004",
			Status:      models.PurchaseOrderStatusFullyDelivered,
			Notes:       "Tech accessories for conference rooms",
			Items: []*models.PurchaseOrderItem{
				{
					Base: models.Base{
						CreatedAt: now.AddDate(0, 0, -3),
						UpdatedAt: now.AddDate(0, 0, -1),
					},
					ProductID:        &productIDs[6], // CalDigit Hub
					SupplierID:       &supplierIDs[2],
					UnitPrice:        399.95,
					Quantity:         4,
					ReceivedQuantity: 4,
					Status:           models.PurchaseOrderItemStatusDelivered,
				},
				{
					Base: models.Base{
						CreatedAt: now.AddDate(0, 0, -3),
						UpdatedAt: now.AddDate(0, 0, -1),
					},
					ProductID:        &productIDs[7], // Logitech Webcam
					SupplierID:       &supplierIDs[2],
					UnitPrice:        199.99,
					Quantity:         6,
					ReceivedQuantity: 6,
					Status:           models.PurchaseOrderItemStatusDelivered,
				},
			},
		},
		{
			Base: models.Base{
				CreatedAt: now.AddDate(0, 0, -1), // 1 day ago
				UpdatedAt: now.AddDate(0, 0, -1),
			},
			OrderNumber: "PO-2024-005",
			Status:      models.PurchaseOrderStatusCancelled,
			Notes:       "Cancelled due to budget constraints",
			Items: []*models.PurchaseOrderItem{
				{
					Base: models.Base{
						CreatedAt: now.AddDate(0, 0, -1),
						UpdatedAt: now.AddDate(0, 0, -1),
					},
					ProductID:        &productIDs[9], // iPad Pro
					SupplierID:       &supplierIDs[0],
					UnitPrice:        1099.99,
					Quantity:         2,
					ReceivedQuantity: 0,
					Status:           models.PurchaseOrderItemStatusDelivering,
				},
			},
		},
		{
			Base: models.Base{
				CreatedAt: now, // Today
				UpdatedAt: now,
			},
			OrderNumber: "PO-2024-006",
			Status:      models.PurchaseOrderStatusOrderPlaced,
			Notes:       "Emergency restock for high-demand items",
			Items: []*models.PurchaseOrderItem{
				{
					Base: models.Base{
						CreatedAt: now,
						UpdatedAt: now,
					},
					ProductID:        &productIDs[0], // MacBook Pro
					SupplierID:       &supplierIDs[0],
					UnitPrice:        2999.99,
					Quantity:         2,
					ReceivedQuantity: 0,
					Status:           models.PurchaseOrderItemStatusDelivering,
				},
				{
					Base: models.Base{
						CreatedAt: now,
						UpdatedAt: now,
					},
					ProductID:        &productIDs[1], // LG Monitor
					SupplierID:       &supplierIDs[0],
					UnitPrice:        599.99,
					Quantity:         5,
					ReceivedQuantity: 0,
					Status:           models.PurchaseOrderItemStatusDelivering,
				},
			},
		},
	}
}
