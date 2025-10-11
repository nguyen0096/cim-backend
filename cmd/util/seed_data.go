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

	"gorm.io/gorm"
)

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

	// Seed data will be generated directly from test/data functions

	// Start transaction
	tx := db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to start transaction: %w", tx.Error)
	}

	// Seed in correct order (respecting foreign key constraints)

	// 1. Suppliers
	suppliers := data.Suppliers()
	var supplierIDs []uint
	for _, supplier := range suppliers {
		// Use the fixed ID from test data
		if err := tx.Create(&supplier).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to create supplier: %w", err)
		}
		supplierIDs = append(supplierIDs, supplier.ID)
	}

	// 2. Products
	products := data.Products(nil)
	var productIDs []uint
	for _, product := range products {
		// Use the fixed ID from test data
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
	inventories := data.Inventory(productIDs)
	var inventoryIDs []uint
	for _, inventory := range inventories {
		// Use the fixed ID from test data
		if err := tx.Create(&inventory).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to create inventory: %w", err)
		}
		inventoryIDs = append(inventoryIDs, inventory.ID)
	}

	// 4. Inventory Items - use fixed IDs from test data
	inventoryItems := data.InventoryItems(inventoryIDs, productIDs, supplierIDs)
	var inventoryItemIDs []uint
	for _, item := range inventoryItems {
		// Use the fixed ID from test data
		if err := tx.Create(&item).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to create inventory item: %w", err)
		}
		inventoryItemIDs = append(inventoryItemIDs, item.ID)
	}

	// 5. Inventory Transactions - use fixed IDs from test data
	inventoryTransactions := data.InventoryTransactions(inventoryItemIDs)
	var transactionIDs []uint
	for _, transaction := range inventoryTransactions {
		// Use the fixed ID from test data
		if err := tx.Create(&transaction).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to create inventory transaction: %w", err)
		}
		transactionIDs = append(transactionIDs, transaction.ID)
	}

	// 5.5. Update inventory items with ConsumingTransactionID
	// Using fixed IDs from test data:
	// Transaction 2: MacBook Pro (20 days ago) - ID 2
	// Transaction 5: LG Monitor (25 days ago) - ID 5
	// Transaction 10: Logitech Mouse (35 days ago) - ID 10
	// Keychron Keyboard keeps ConsumingTransactionID as 0

	// Update inventory items with ConsumingTransactionID
	// Item 0 (MacBook Pro): set to transaction 2 (20 days ago) - ID 2
	if err := tx.Model(&models.InventoryItem{}).Where("id = ?", inventoryItemIDs[0]).Update("consuming_transaction_id", uint(2)).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update inventory item 0: %w", err)
	}

	// Item 1 (LG Monitor): set to transaction 5 (25 days ago) - ID 5
	if err := tx.Model(&models.InventoryItem{}).Where("id = ?", inventoryItemIDs[1]).Update("consuming_transaction_id", uint(5)).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update inventory item 1: %w", err)
	}

	// Item 2 (Keychron Keyboard): keep ConsumingTransactionID as 0

	// Item 3 (Logitech Mouse): set to transaction 10 (35 days ago) - ID 10
	if err := tx.Model(&models.InventoryItem{}).Where("id = ?", inventoryItemIDs[3]).Update("consuming_transaction_id", uint(10)).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update inventory item 3: %w", err)
	}

	// 6. Purchase Orders - generate with actual product IDs
	purchaseOrders := data.PurchaseOrders(productIDs, supplierIDs)
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
	if err := seedUsers(db); err != nil {
		return fmt.Errorf("failed to seed users: %w", err)
	}

	return nil
}

// seedUsers populates the database with default users
func seedUsers(db *gorm.DB) error {
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
