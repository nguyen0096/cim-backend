package data

import (
	"import-export-backend/internal/models"
	"time"

	"github.com/google/uuid"
)

// SupplierIDs contains predefined supplier UUIDs for consistent referencing
var SupplierIDs = struct {
	TechElectronics uuid.UUID
	OfficeSupply    uuid.UUID
	GlobalParts     uuid.UUID
}{
	TechElectronics: uuid.MustParse("550e8400-e29b-41d4-a716-446655440001"),
	OfficeSupply:    uuid.MustParse("550e8400-e29b-41d4-a716-446655440002"),
	GlobalParts:     uuid.MustParse("550e8400-e29b-41d4-a716-446655440003"),
}

// ProductIDs contains predefined product UUIDs for consistent referencing
var ProductIDs = struct {
	MacBookPro         uuid.UUID
	GamingMonitor      uuid.UUID
	MechanicalKeyboard uuid.UUID
	WirelessMouse      uuid.UUID
	OfficeChair        uuid.UUID
	StandingDesk       uuid.UUID
	USBCHub            uuid.UUID
	Webcam             uuid.UUID
	Headphones         uuid.UUID
	TabletPro          uuid.UUID
}{
	MacBookPro:         uuid.MustParse("660e8400-e29b-41d4-a716-446655440001"),
	GamingMonitor:      uuid.MustParse("660e8400-e29b-41d4-a716-446655440002"),
	MechanicalKeyboard: uuid.MustParse("660e8400-e29b-41d4-a716-446655440003"),
	WirelessMouse:      uuid.MustParse("660e8400-e29b-41d4-a716-446655440004"),
	OfficeChair:        uuid.MustParse("660e8400-e29b-41d4-a716-446655440005"),
	StandingDesk:       uuid.MustParse("660e8400-e29b-41d4-a716-446655440006"),
	USBCHub:            uuid.MustParse("660e8400-e29b-41d4-a716-446655440007"),
	Webcam:             uuid.MustParse("660e8400-e29b-41d4-a716-446655440008"),
	Headphones:         uuid.MustParse("660e8400-e29b-41d4-a716-446655440009"),
	TabletPro:          uuid.MustParse("660e8400-e29b-41d4-a716-446655440010"),
}

// Suppliers contains all test supplier data
func Suppliers() []models.Supplier {
	now := time.Now()

	return []models.Supplier{
		{
			ID:           SupplierIDs.TechElectronics,
			Name:         "Tech Electronics Inc",
			ContactEmail: "contact@techelectronics.com",
			ContactPhone: "+1-555-0123",
			Address:      "123 Silicon Valley Blvd, San Jose, CA 95110",
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           SupplierIDs.OfficeSupply,
			Name:         "Office Supply Co",
			ContactEmail: "sales@officesupply.com",
			ContactPhone: "+1-555-0456",
			Address:      "456 Business Park Dr, Dallas, TX 75201",
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           SupplierIDs.GlobalParts,
			Name:         "Global Parts Ltd",
			ContactEmail: "orders@globalparts.com",
			ContactPhone: "+1-555-0789",
			Address:      "789 Industrial Way, Seattle, WA 98101",
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}
}

// Products contains all test product data
func Products() []models.Product {
	now := time.Now()

	return []models.Product{
		{
			ID:          ProductIDs.MacBookPro,
			Name:        "MacBook Pro 16-inch M3",
			Description: "Professional laptop with M3 chip, 32GB RAM, 1TB SSD",
			SKU:         "LAPTOP-MBP-16-M3",
			SupplierID:  SupplierIDs.TechElectronics,
			UnitPrice:   2999.99,
			Status:      "active",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          ProductIDs.GamingMonitor,
			Name:        "LG UltraGear 27\" 4K Gaming Monitor",
			Description: "27-inch 4K UHD gaming monitor with 144Hz refresh rate",
			SKU:         "MONITOR-LG-27-4K",
			SupplierID:  SupplierIDs.TechElectronics,
			UnitPrice:   599.99,
			Status:      "active",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          ProductIDs.MechanicalKeyboard,
			Name:        "Keychron K8 Mechanical Keyboard",
			Description: "Wireless mechanical keyboard with RGB backlight and hot-swappable switches",
			SKU:         "KEYBOARD-KEYCHRON-K8",
			SupplierID:  SupplierIDs.TechElectronics,
			UnitPrice:   89.99,
			Status:      "active",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          ProductIDs.WirelessMouse,
			Name:        "Logitech MX Master 3S",
			Description: "Advanced wireless mouse with precision scrolling and customizable buttons",
			SKU:         "MOUSE-LOGITECH-MX3S",
			SupplierID:  SupplierIDs.TechElectronics,
			UnitPrice:   99.99,
			Status:      "active",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          ProductIDs.OfficeChair,
			Name:        "Herman Miller Aeron Chair",
			Description: "Ergonomic office chair with lumbar support and breathable mesh",
			SKU:         "CHAIR-HERMAN-AERON",
			SupplierID:  SupplierIDs.OfficeSupply,
			UnitPrice:   1395.00,
			Status:      "active",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          ProductIDs.StandingDesk,
			Name:        "UPLIFT Standing Desk 60x30",
			Description: "Height-adjustable standing desk with bamboo top and memory settings",
			SKU:         "DESK-UPLIFT-60X30",
			SupplierID:  SupplierIDs.OfficeSupply,
			UnitPrice:   799.00,
			Status:      "active",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          ProductIDs.USBCHub,
			Name:        "CalDigit TS4 Thunderbolt 4 Hub",
			Description: "18-port Thunderbolt 4 hub with 98W charging and 40Gbps data transfer",
			SKU:         "HUB-CALDIGIT-TS4",
			SupplierID:  SupplierIDs.GlobalParts,
			UnitPrice:   399.95,
			Status:      "active",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          ProductIDs.Webcam,
			Name:        "Logitech Brio 4K Webcam",
			Description: "Ultra HD 4K webcam with HDR and Windows Hello support",
			SKU:         "WEBCAM-LOGITECH-BRIO",
			SupplierID:  SupplierIDs.GlobalParts,
			UnitPrice:   199.99,
			Status:      "active",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          ProductIDs.Headphones,
			Name:        "Sony WH-1000XM5 Headphones",
			Description: "Industry-leading noise cancelling wireless headphones with 30-hour battery",
			SKU:         "HEADPHONES-SONY-XM5",
			SupplierID:  SupplierIDs.TechElectronics,
			UnitPrice:   399.99,
			Status:      "active",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          ProductIDs.TabletPro,
			Name:        "iPad Pro 12.9\" M2",
			Description: "12.9-inch iPad Pro with M2 chip, 256GB storage, and Liquid Retina XDR display",
			SKU:         "TABLET-IPAD-PRO-12",
			SupplierID:  SupplierIDs.TechElectronics,
			UnitPrice:   1099.99,
			Status:      "active",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
}

// InventoryData represents inventory configuration for a product
type InventoryData struct {
	ProductID    uuid.UUID
	Quantity     int
	ReorderLevel int
	Location     string
}

// InventoryConfigs contains inventory configuration for all products
var InventoryConfigs = []InventoryData{
	{ProductIDs.MacBookPro, 15, 5, "Warehouse A"},
	{ProductIDs.GamingMonitor, 30, 10, "Warehouse B"},
	{ProductIDs.MechanicalKeyboard, 45, 15, "Warehouse C"},
	{ProductIDs.WirelessMouse, 25, 8, "Warehouse A"},
	{ProductIDs.OfficeChair, 8, 3, "Warehouse B"},
	{ProductIDs.StandingDesk, 12, 5, "Warehouse C"},
	{ProductIDs.USBCHub, 20, 8, "Warehouse A"},
	{ProductIDs.Webcam, 35, 12, "Warehouse B"},
	{ProductIDs.Headphones, 18, 6, "Warehouse C"},
	{ProductIDs.TabletPro, 22, 8, "Warehouse A"},
}

// Inventory contains all test inventory data
func Inventory() []models.Inventory {
	now := time.Now()

	inventory := make([]models.Inventory, len(InventoryConfigs))
	for i, config := range InventoryConfigs {
		inventory[i] = models.Inventory{
			ID:           uuid.New(),
			ProductID:    config.ProductID,
			Quantity:     config.Quantity,
			ReorderLevel: config.ReorderLevel,
			Location:     config.Location,
			LastUpdated:  now,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
	}

	return inventory
}
