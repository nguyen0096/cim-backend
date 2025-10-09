package main

import (
	"context"
	"fmt"
	"import-export-backend/internal/auth"
	"import-export-backend/internal/config"
	"import-export-backend/internal/database"
	"import-export-backend/internal/models"
	"import-export-backend/internal/repository"
	"import-export-backend/internal/services"
	"import-export-backend/pkg"
	"import-export-backend/test/data"
	"log"
	"time"

	"gorm.io/gorm"
)

// SeedData contains all the mock data with references
type SeedData struct {
	Suppliers      []models.Supplier
	Products       []models.Product
	Inventories    []models.Inventory
	InventoryItems []models.InventoryItem
	PurchaseOrders []models.PurchaseOrder
}

// seedDatabase populates the database with mock data
func seedDatabase() error {
	// Load configuration
	cfg := config.Load()

	// Initialize database
	db, err := database.Initialize(cfg.Database)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	// Create context with user email for CreatedBy field
	ctx := pkg.WithUserEmail(context.Background(), "seeder@test.com")

	// Generate seed data
	seedData := generateSeedData()

	// Start transaction
	tx := db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to start transaction: %w", tx.Error)
	}

	// Seed in correct order (respecting foreign key constraints)

	// 1. Suppliers
	var supplierIDs []uint
	for _, supplier := range seedData.Suppliers {
		if err := tx.Create(&supplier).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to create supplier: %w", err)
		}
		supplierIDs = append(supplierIDs, supplier.ID)
	}

	// 2. Products
	var productIDs []uint
	for _, product := range seedData.Products {
		if err := tx.Create(&product).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to create product: %w", err)
		}
		productIDs = append(productIDs, product.ID)
	}

	// 2.5. Create product-supplier relationships
	for i, productID := range productIDs {
		// Assign suppliers to products based on product index
		supplierIndex := i % len(supplierIDs) // Cycle through suppliers
		supplierID := supplierIDs[supplierIndex]

		// Create many-to-many relationship (junction table only has foreign keys)
		if err := tx.Exec("INSERT INTO product_suppliers (product_id, supplier_id) VALUES (?, ?)",
			productID, supplierID).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to create product-supplier relationship: %w", err)
		}
	}

	// 3. Inventories
	var inventoryIDs []uint
	for _, inventory := range seedData.Inventories {
		if err := tx.Create(&inventory).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to create inventory: %w", err)
		}
		inventoryIDs = append(inventoryIDs, inventory.ID)
	}

	// 4. Inventory Items - generate with actual IDs
	inventoryItems := generateInventoryItems(productIDs, inventoryIDs, supplierIDs)
	for _, item := range inventoryItems {
		if err := tx.Create(&item).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to create inventory item: %w", err)
		}
	}

	// 5. Purchase Orders - generate with actual product IDs
	purchaseOrders := generatePurchaseOrders(productIDs)
	for _, po := range purchaseOrders {
		if err := tx.Create(&po).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to create purchase order: %w", err)
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Seed users after database schema is populated
	if err := seedUsers(db, cfg); err != nil {
		return fmt.Errorf("failed to seed users: %w", err)
	}

	return nil
}

// generateInventoryItems creates inventory items with actual database IDs
func generateInventoryItems(productIDs, inventoryIDs, supplierIDs []uint) []models.InventoryItem {
	now := time.Now()

	// Inventory configurations with pricing data
	configs := []struct {
		ProductIndex  int
		SupplierIndex int
		UnitPrice     float64
		UnitType      string
		Quantity      int
		ReorderLevel  int
		Location      string
	}{
		{0, 0, 2999.99, "piece", 15, 5, "Warehouse A"},
		{1, 0, 599.99, "piece", 30, 10, "Warehouse B"},
		{2, 0, 89.99, "piece", 45, 15, "Warehouse C"},
		{3, 0, 99.99, "piece", 25, 8, "Warehouse A"},
		{4, 1, 1395.00, "piece", 8, 3, "Warehouse B"},
		{5, 1, 799.00, "box", 12, 5, "Warehouse C"},
		{6, 2, 399.95, "piece", 20, 8, "Warehouse A"},
		{7, 2, 199.99, "piece", 35, 12, "Warehouse B"},
		{8, 0, 399.99, "piece", 18, 6, "Warehouse C"},
		{9, 0, 1099.99, "device", 22, 8, "Warehouse A"},
	}

	items := make([]models.InventoryItem, len(configs))
	for i, config := range configs {
		// Map location to inventory ID
		var inventoryID uint
		switch config.Location {
		case "Warehouse A":
			inventoryID = inventoryIDs[0]
		case "Warehouse B":
			inventoryID = inventoryIDs[1]
		case "Warehouse C":
			inventoryID = inventoryIDs[2]
		default:
			inventoryID = inventoryIDs[0]
		}

		items[i] = models.InventoryItem{
			Base: models.Base{
				CreatedAt: now,
				UpdatedAt: now,
			},
			InventoryID:   inventoryID,
			ProductID:     productIDs[config.ProductIndex],
			SupplierID:    supplierIDs[config.SupplierIndex],
			UnitPrice:     config.UnitPrice,
			UnitType:      config.UnitType,
			Quantity:      config.Quantity,
			ReorderLevel:  config.ReorderLevel,
			MaxStockLevel: config.Quantity * 3,
			Status:        models.InventoryItemStatusActive,
		}
	}

	return items
}

// generatePurchaseOrders creates purchase orders with actual database IDs
func generatePurchaseOrders(productIDs []uint) []models.PurchaseOrder {
	now := time.Now()

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
					UnitPrice:        599.99,
					Quantity:         5,
					ReceivedQuantity: 0,
					Status:           models.PurchaseOrderItemStatusDelivering,
				},
			},
		},
	}
}

// generateSeedData creates the mock data using the centralized test data
func generateSeedData() SeedData {
	suppliers := data.Suppliers()
	products := data.Products(nil) // supplierIDs parameter is no longer needed

	return SeedData{
		Suppliers:      suppliers,
		Products:       products,
		Inventories:    data.Inventory(nil),      // productIDs parameter is no longer needed
		InventoryItems: data.InventoryItems(nil), // productIDs parameter is no longer needed
		PurchaseOrders: []models.PurchaseOrder{}, // Will be generated with actual IDs in seedDatabase
	}
}

// seedUsers populates the database with default users
func seedUsers(db *gorm.DB, cfg *config.Config) error {
	// Initialize Casbin service
	casbinService, err := auth.NewCasbinService(db)
	if err != nil {
		return fmt.Errorf("failed to initialize Casbin service: %w", err)
	}

	// Initialize user repository and service
	userRepo := repository.NewUserRepository(db)
	userService := services.NewUserService(userRepo, casbinService)

	ctx := context.Background()

	// Define default users
	defaultUsers := []struct {
		UID   string
		Email string
		Name  string
		Role  string
	}{
		{
			UID:   "demoAdminUid0000000000000000",
			Email: "test@cim.local",
			Name:  "Admin User",
			Role:  string(models.RoleAdmin),
		},
		{
			UID:   "demoRootAdminUid000000000000",
			Email: "admin@example.com",
			Name:  "Admin User",
			Role:  string(models.RoleAdmin),
		},
		{
			UID:   "demoRootAdminUid200000000000",
			Email: "admin2@example.com",
			Name:  "Admin User",
			Role:  string(models.RoleAdmin),
		},
		{
			UID:   "demoAccountantUid00000000000",
			Email: "accountant@cim.local",
			Name:  "Accountant User",
			Role:  string(models.RoleAccountant),
		},
		{
			UID:   "demoStaffUid0000000000000000",
			Email: "staff@cim.local",
			Name:  "Staff User",
			Role:  string(models.RoleStaff),
		},
	}

	// Seed each user
	for _, userData := range defaultUsers {
		// Check if user already exists
		existingUser, err := userService.GetUserByUID(ctx, userData.UID)
		if err != nil {
			log.Printf("Error checking existing user %s: %v", userData.Email, err)
			continue
		}

		if existingUser != nil {
			log.Printf("User %s already exists, skipping", userData.Email)
			continue
		}

		// Create user
		user, err := userService.CreateOrUpdateUser(ctx, userData.UID, userData.Email, userData.Name)
		if err != nil {
			log.Printf("Failed to create user %s: %v", userData.Email, err)
			continue
		}

		// Update role if different from default
		if user.Role != userData.Role {
			err = userService.UpdateUserRole(ctx, user.UID, userData.Role, "system")
			if err != nil {
				log.Printf("Failed to update role for user %s: %v", userData.Email, err)
				continue
			}
		}

		log.Printf("Created user: %s with role: %s", userData.Email, userData.Role)
	}

	log.Println("User seeding completed!")
	return nil
}
