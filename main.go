package main

import (
	"import-export-backend/internal/auth"
	"import-export-backend/internal/config"
	"import-export-backend/internal/database"
	"import-export-backend/internal/handlers"
	"import-export-backend/internal/middleware"
	"import-export-backend/internal/repository"
	"import-export-backend/internal/services"
	"log"

	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/sirupsen/logrus"
)

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

	// Initialize repositories
	supplierRepo := repository.NewSupplierRepository(db)
	productRepo := repository.NewProductRepository(db)
	inventoryRepo := repository.NewInventoryRepository(db)
	purchaseOrderRepo := repository.NewPurchaseOrderRepository(db)
	orderRepo := repository.NewOrderRepository(db)

	// Initialize services
	supplierService := services.NewSupplierService(supplierRepo)
	productService := services.NewProductService(productRepo, inventoryRepo)
	inventoryService := services.NewInventoryService(inventoryRepo, productRepo)
	purchaseOrderService := services.NewPurchaseOrderService(purchaseOrderRepo, inventoryService)
	orderService := services.NewOrderService(orderRepo, inventoryService)
	excelService := services.NewExcelService(productRepo, inventoryRepo)

	// Initialize handlers
	userHandler := handlers.NewUserHandler()
	supplierHandler := handlers.NewSupplierHandler(supplierService)
	productHandler := handlers.NewProductHandler(productService)
	inventoryHandler := handlers.NewInventoryHandler(inventoryService)
	purchaseOrderHandler := handlers.NewPurchaseOrderHandler(purchaseOrderService)
	orderHandler := handlers.NewOrderHandler(orderService)
	excelHandler := handlers.NewExcelHandler(excelService)

	// Initialize Echo
	e := echo.New()

	// Middleware
	e.Use(echoMiddleware.Logger())
	e.Use(echoMiddleware.Recover())
	e.Use(echoMiddleware.CORSWithConfig(echoMiddleware.CORSConfig{
		AllowOrigins: []string{"http://localhost:3000"},
		AllowMethods: []string{echo.GET, echo.POST, echo.PUT, echo.DELETE, echo.OPTIONS},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
	}))

	// Health check
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(200, map[string]string{"status": "ok"})
	})

	// API routes
	api := e.Group("/api/v1")

	// Authentication routes
	authGroup := api.Group("/auth")
	authGroup.POST("/verify-token", userHandler.VerifyToken)
	authGroup.GET("/profile", userHandler.GetProfile, middleware.AuthMiddleware(firebaseAuth))

	// Product routes
	products := api.Group("/products", middleware.AuthMiddleware(firebaseAuth))
	products.GET("", productHandler.GetProducts)
	products.GET("/search", productHandler.SearchProducts)
	products.POST("", productHandler.CreateProduct)
	products.GET("/:id", productHandler.GetProduct)
	products.PUT("/:id", productHandler.UpdateProduct)
	products.DELETE("/:id", productHandler.DeleteProduct)
	products.POST("/:id/restore", productHandler.RestoreProduct)
	products.GET("/:id/inventory", productHandler.GetProductInventory)

	// Inventory routes
	inventory := api.Group("/inventory", middleware.AuthMiddleware(firebaseAuth))
	inventory.GET("", inventoryHandler.GetInventory)
	inventory.PUT("/:id", inventoryHandler.UpdateInventory)
	inventory.POST("/adjust", inventoryHandler.AdjustInventory)
	inventory.GET("/transactions", inventoryHandler.GetTransactions)
	inventory.GET("/low-stock", inventoryHandler.GetLowStock)

	// Supplier routes
	suppliers := api.Group("/suppliers", middleware.AuthMiddleware(firebaseAuth))
	suppliers.GET("", supplierHandler.GetSuppliers)
	suppliers.GET("/search", supplierHandler.SearchSuppliers)
	suppliers.POST("", supplierHandler.CreateSupplier)
	suppliers.GET("/:id", supplierHandler.GetSupplier)
	suppliers.PUT("/:id", supplierHandler.UpdateSupplier)
	suppliers.DELETE("/:id", supplierHandler.DeleteSupplier)

	// Purchase Order routes
	purchaseOrders := api.Group("/purchase-orders", middleware.AuthMiddleware(firebaseAuth))
	purchaseOrders.POST("", purchaseOrderHandler.CreatePurchaseOrder)
	// purchaseOrders.GET("", purchaseOrderHandler.GetPurchaseOrders)
	// purchaseOrders.GET("/:id", purchaseOrderHandler.GetPurchaseOrder)
	// purchaseOrders.PUT("/:id", purchaseOrderHandler.UpdatePurchaseOrder)
	// purchaseOrders.PUT("/:id/status", purchaseOrderHandler.UpdatePurchaseOrderStatus)
	// purchaseOrders.DELETE("/:id", purchaseOrderHandler.DeletePurchaseOrder)
	// purchaseOrders.POST("/:id/receive", purchaseOrderHandler.ReceivePurchaseOrder)

	// Order routes
	orders := api.Group("/orders", middleware.AuthMiddleware(firebaseAuth))
	orders.GET("", orderHandler.GetOrders)
	orders.POST("", orderHandler.CreateOrder)
	orders.GET("/:id", orderHandler.GetOrder)
	orders.PUT("/:id", orderHandler.UpdateOrder)
	orders.PUT("/:id/status", orderHandler.UpdateOrderStatus)
	orders.DELETE("/:id", orderHandler.DeleteOrder)
	orders.POST("/:id/complete", orderHandler.CompleteOrder)

	// Excel routes
	excel := api.Group("/excel", middleware.AuthMiddleware(firebaseAuth))
	excel.POST("/import-products", excelHandler.ImportProducts)
	excel.GET("/export-products", excelHandler.ExportProducts)
	excel.POST("/import-inventory", excelHandler.ImportInventory)
	excel.GET("/export-inventory", excelHandler.ExportInventory)
	excel.GET("/template-products", excelHandler.GetProductTemplate)
	excel.GET("/template-inventory", excelHandler.GetInventoryTemplate)

	// Reports routes
	reports := api.Group("/reports", middleware.AuthMiddleware(firebaseAuth))
	reports.GET("/inventory-summary", inventoryHandler.GetInventorySummary)
	reports.GET("/low-stock", inventoryHandler.GetLowStock)
	reports.GET("/purchase-summary", purchaseOrderHandler.GetPurchaseSummary)
	reports.GET("/order-summary", orderHandler.GetOrderSummary)

	// Start server
	logger.Infof("Server starting on %s:%s", cfg.Server.Host, cfg.Server.Port)
	if err := e.Start(cfg.Server.Host + ":" + cfg.Server.Port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
