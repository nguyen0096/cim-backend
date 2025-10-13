package main

import (
	"cim-backend/internal/auth"
	"cim-backend/internal/config"
	"cim-backend/internal/database"
	"cim-backend/internal/handlers"
	"cim-backend/internal/middleware"
	"cim-backend/internal/repository"
	"cim-backend/internal/services"
	"log"

	_ "cim-backend/docs" // Import generated docs

	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/sirupsen/logrus"
	echoSwagger "github.com/swaggo/echo-swagger"
)

// @title Import Export Backend API
// @version 1.0
// @description This is an Import Export Backend API server for managing products, inventory, suppliers, and orders.
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize logger
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	// Initialize Firebase Auth
	firebaseAuth, err := auth.NewFirebaseAuthService(cfg.Firebase.ServiceAccountPath)
	if err != nil {
		log.Fatal("Failed to initialize Firebase Auth:", err)
	}

	// Initialize database
	db, err := database.Initialize(cfg.Database)
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}

	// Run migrations
	if err := database.Migrate(db); err != nil {
		log.Fatal("Failed to run migrations:", err)
	}

	// Initialize Casbin service for authorization
	casbinService, err := auth.NewCasbinService(db)
	if err != nil {
		log.Fatal("Failed to initialize Casbin service:", err)
	}

	// Initialize repositories
	userRepo := repository.NewUserRepository(db)
	supplierRepo := repository.NewSupplierRepository(db)
	productRepo := repository.NewProductRepository(db)
	inventoryRepo := repository.NewInventoryRepository(db)
	inventoryItemRepo := repository.NewInventoryItemRepository(db)
	purchaseOrderRepo := repository.NewPurchaseOrderRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)

	// Initialize services
	userService := services.NewUserService(userRepo, casbinService)
	supplierService := services.NewSupplierService(supplierRepo)
	productService := services.NewProductService(productRepo, supplierRepo)
	inventoryService := services.NewInventoryService(inventoryRepo, inventoryItemRepo, productRepo)
	inventoryItemService := services.NewInventoryItemService(inventoryItemRepo, inventoryRepo, productRepo)
	excelService := services.NewExcelService(productRepo, inventoryRepo)
	settingsService := services.NewSettingsService(settingsRepo)
	purchaseOrderService := services.NewPurchaseOrderService(purchaseOrderRepo, inventoryService, excelService, settingsService, db, logger)

	// Initialize handlers
	userHandler := handlers.NewUserHandler(userService, firebaseAuth)
	supplierHandler := handlers.NewSupplierHandler(supplierService)
	productHandler := handlers.NewProductHandler(productService, logger)
	inventoryHandler := handlers.NewInventoryHandler(inventoryService)
	inventoryItemHandler := handlers.NewInventoryItemHandler(inventoryItemService)
	purchaseOrderHandler := handlers.NewPurchaseOrderHandler(purchaseOrderRepo, purchaseOrderService, logger)
	excelHandler := handlers.NewExcelHandler(excelService)
	settingsHandler := handlers.NewSettingsHandler(settingsService)

	// Initialize Echo
	e := echo.New()

	// Set custom error handler
	e.HTTPErrorHandler = middleware.CustomErrorHandler

	// Middleware
	e.Use(echoMiddleware.Logger())
	e.Use(echoMiddleware.Recover())
	e.Use(echoMiddleware.CORSWithConfig(echoMiddleware.CORSConfig{
		AllowOrigins: []string{"http://localhost:3000", "http://localhost:3001"},
		AllowMethods: []string{echo.GET, echo.POST, echo.PUT, echo.DELETE, echo.OPTIONS},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
	}))

	// Swagger documentation with persistent authorization
	e.GET("/swagger/*", echoSwagger.EchoWrapHandler(func(c *echoSwagger.Config) {
		c.DeepLinking = true
		c.InstanceName = "swagger"
		c.PersistAuthorization = true
	}))

	// Health check
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(200, map[string]string{"status": "ok"})
	})

	// API routes
	api := e.Group("/api/v1")

	api.Use(middleware.AuthMiddleware(firebaseAuth))
	api.Use(middleware.AuthorizationMiddleware(casbinService, userRepo))

	// Authentication routes
	authGroup := api.Group("/auth")
	authGroup.POST("/verify-token", userHandler.VerifyToken)
	authGroup.GET("/profile", userHandler.GetProfile)

	// User management routes (admin only)
	users := api.Group("/users")
	users.GET("", userHandler.ListUsers)
	users.GET("/role/:role", userHandler.GetUsersByRole)
	users.GET("/permissions", userHandler.GetUserPermissions)
	users.PUT("/:uid/role", userHandler.UpdateUserRole)
	users.DELETE("/:id", userHandler.DeleteUser)

	// Product routes
	products := api.Group("/products")

	products.GET("", productHandler.GetProducts)
	products.GET("/search", productHandler.SearchProducts)
	products.POST("", productHandler.CreateProduct)
	products.POST("/import-csv", productHandler.ImportProductsCSV)
	products.GET("/:id", productHandler.GetProduct)
	products.PUT("/:id", productHandler.UpdateProduct)
	products.PUT("/:id/status", productHandler.UpdateProductStatus)
	products.DELETE("/:id", productHandler.DeleteProduct)
	products.POST("/:id/restore", productHandler.RestoreProduct)
	products.GET("/:id/inventory", productHandler.GetProductInventory)

	// Inventory routes
	inventories := api.Group("/inventories")
	inventories.GET("", inventoryHandler.ListInventory)
	inventories.POST("", inventoryHandler.CreateInventory)
	inventories.GET("/:id", inventoryHandler.GetInventory)
	inventories.PUT("/:id", inventoryHandler.UpdateInventory)
	inventories.DELETE("/:id", inventoryHandler.DeleteInventory)
	inventories.GET("/last-purchase-prices", inventoryHandler.GetLastPurchasePrices)
	inventories.PUT("/:id/dispose", inventoryHandler.DisposeInventoryItems)
	inventories.PUT("/:id/reconcile", inventoryHandler.ReconcileInventory)

	// Nested inventory items routes
	inventories.GET("/:id/inventory-items", inventoryItemHandler.GetInventoryItemsByInventoryID)
	inventories.POST("/:id/inventory-items", inventoryItemHandler.CreateInventoryItem)
	inventories.GET("/:id/inventory-items/:item_id", inventoryItemHandler.GetInventoryItem)
	inventories.PUT("/:id/inventory-items/:item_id", inventoryItemHandler.UpdateInventoryItem)
	inventories.DELETE("/:id/inventory-items/:item_id", inventoryItemHandler.DeleteInventoryItem)
	inventories.PUT("/:id/inventory-items/:item_id/adjust", inventoryItemHandler.AdjustInventoryItemQuantity)

	// Standalone inventory item routes (for backward compatibility and specific use cases)
	inventoryItems := api.Group("/inventory-items")
	inventoryItems.GET("", inventoryItemHandler.ListInventoryItems)
	inventoryItems.GET("/product/:product_id", inventoryItemHandler.GetInventoryItemByProductID)
	inventoryItems.GET("/low-stock", inventoryItemHandler.GetLowStockItems)

	// Supplier routes
	suppliers := api.Group("/suppliers")
	suppliers.GET("", supplierHandler.GetSuppliers)
	suppliers.GET("/search", supplierHandler.SearchSuppliers)
	suppliers.POST("", supplierHandler.CreateSupplier)
	suppliers.GET("/:id", supplierHandler.GetSupplier)
	suppliers.PUT("/:id", supplierHandler.UpdateSupplier)
	suppliers.PUT("/:id/status", supplierHandler.UpdateSupplierStatus)
	suppliers.DELETE("/:id", supplierHandler.DeleteSupplier)
	suppliers.POST("/:id/restore", supplierHandler.RestoreSupplier)

	// Purchase Order routes
	purchaseOrders := api.Group("/purchase-orders")
	purchaseOrders.GET("", purchaseOrderHandler.ListPurchaseOrders)
	purchaseOrders.POST("", purchaseOrderHandler.CreatePurchaseOrder)
	purchaseOrders.PUT("/:id/receive", purchaseOrderHandler.ReceiveInventory)

	// Excel routes
	excel := api.Group("/excel")
	excel.POST("/import-products", excelHandler.ImportProducts)
	excel.POST("/import-inventory", excelHandler.ImportInventory)
	excel.GET("/template-products", excelHandler.GetProductTemplate)
	excel.GET("/template-inventory", excelHandler.GetInventoryTemplate)
	excel.POST("/verify", excelHandler.VerifyFileAndSheet)

	// Settings routes
	settings := api.Group("/settings")
	settings.GET("", settingsHandler.GetAllSettings)
	settings.GET("/:key", settingsHandler.GetSetting)
	settings.POST("/:key", settingsHandler.SetSetting)
	settings.DELETE("/:key", settingsHandler.DeleteSetting)

	// Reports routes
	reports := api.Group("/reports")
	reports.GET("/inventory-summary", inventoryHandler.GetInventorySummary)
	reports.GET("/low-stock", inventoryItemHandler.GetLowStockItems)
	reports.GET("/purchase-summary", purchaseOrderHandler.GetPurchaseSummary)

	// Start server
	logger.Infof("Server starting on %s:%s", cfg.Server.Host, cfg.Server.Port)
	if err := e.Start(cfg.Server.Host + ":" + cfg.Server.Port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
