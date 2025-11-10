package server

import (
	"context"
	"strings"

	"cim-backend/internal/auth"
	"cim-backend/internal/config"
	"cim-backend/internal/handlers"
	"cim-backend/internal/middleware"
	"cim-backend/internal/repository"
	"cim-backend/internal/services"
	"cim-backend/pkg"

	_ "cim-backend/docs" // Import generated docs

	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/sirupsen/logrus"
	echoSwagger "github.com/swaggo/echo-swagger"
	"gorm.io/gorm"
)

// SetupServer creates and configures the Echo server with all routes and middleware
// Returns the configured Echo instance without starting it
func SetupServer(
	cfg *config.Config,
	db *gorm.DB,
	firebaseAuth auth.FirebaseAuthInterface,
	logger *logrus.Logger,
) (*echo.Echo, error) {
	// Initialize Casbin service for authorization
	casbinService, err := auth.NewCasbinService(db)
	if err != nil {
		return nil, err
	}

	// Initialize repositories
	userRepo := repository.NewUserRepository(db, cfg.Environment)
	supplierRepo := repository.NewSupplierRepository(db)
	unitRepo := repository.NewUnitRepository(db)
	productRepo := repository.NewProductRepository(db)
	inventoryRepo := repository.NewInventoryRepository(db)
	inventoryItemRepo := repository.NewInventoryItemRepository(db)
	inventorySubmissionRepo := repository.NewInventorySubmissionRepository(db)
	purchaseOrderRepo := repository.NewPurchaseOrderRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	paymentReceiptFormRepo := repository.NewPaymentReceiptFormRepository(db)

	// Initialize services
	userService := services.NewUserService(userRepo, casbinService)
	supplierService := services.NewSupplierService(supplierRepo)
	unitService := services.NewUnitService(unitRepo)
	settingsService := services.NewSettingsService(settingsRepo)
	productService := services.NewProductService(productRepo, supplierRepo, unitRepo, settingsService)
	inventoryService := services.NewInventoryService(inventoryRepo, inventoryItemRepo, inventorySubmissionRepo, productRepo)
	inventoryItemService := services.NewInventoryItemService(inventoryItemRepo, inventoryRepo, productRepo)
	excelService := services.NewExcelService(productRepo, inventoryRepo, settingsService)
	purchaseOrderService := services.NewPurchaseOrderService(purchaseOrderRepo, paymentReceiptFormRepo, inventoryService, excelService, settingsService, db, logger)
	paymentReceiptFormService := services.NewPaymentReceiptFormService(paymentReceiptFormRepo, db)

	// Initialize handlers
	userHandler := handlers.NewUserHandler(userService, firebaseAuth)
	supplierHandler := handlers.NewSupplierHandler(supplierService)
	unitHandler := handlers.NewUnitHandler(unitService)
	productHandler := handlers.NewProductHandler(productService, logger)
	inventoryHandler := handlers.NewInventoryHandler(inventoryService)
	inventoryItemHandler := handlers.NewInventoryItemHandler(inventoryItemService)
	purchaseOrderHandler := handlers.NewPurchaseOrderHandler(purchaseOrderRepo, purchaseOrderService, logger)
	excelHandler := handlers.NewExcelHandler(excelService)
	settingsHandler := handlers.NewSettingsHandler(settingsService)
	paymentReceiptFormHandler := handlers.NewPaymentReceiptFormHandler(paymentReceiptFormService, settingsService, logger)
	revenueExpenseHandler := handlers.NewRevenueExpenseHandler(excelService, settingsService, logger)

	// Initialize Echo
	e := echo.New()

	// Set custom error handler
	e.HTTPErrorHandler = middleware.CustomErrorHandler

	// Middleware
	e.Use(echoMiddleware.Logger())
	e.Use(echoMiddleware.Recover())
	e.Use(echoMiddleware.CORSWithConfig(echoMiddleware.CORSConfig{
		AllowOrigins: cfg.Server.CORSAllowedOrigins,
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

	// Convert interface to concrete type for middleware (if it's the real service)
	if realAuth, ok := firebaseAuth.(*auth.FirebaseAuthService); ok {
		api.Use(middleware.AuthMiddleware(realAuth))
	} else {
		// For tests with mock, we'll need to create a test-specific middleware
		// For now, we'll use a type assertion helper
		api.Use(createAuthMiddleware(firebaseAuth))
	}
	api.Use(middleware.AuthorizationMiddleware(casbinService, userService))

	// Authentication routes
	authGroup := api.Group("/auth")
	authGroup.GET("/profile", userHandler.GetProfile)

	// User management routes (admin only)
	users := api.Group("/users")
	users.GET("", userHandler.ListUsers)
	users.GET("/search", userHandler.SearchUsers)
	users.POST("", userHandler.CreateUser)
	users.GET("/role/:role", userHandler.GetUsersByRole)
	users.GET("/permissions", userHandler.GetUserPermissions)
	users.PUT("/:id", userHandler.UpdateUser)
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
	inventories.POST("/submissions/:id/process", inventoryHandler.ProcessSubmission)
	inventories.PUT("/submissions/:id", inventoryHandler.UpdateSubmission)
	inventories.GET("/:id/submissions", inventoryHandler.ListSubmissions)
	inventories.POST("/transfer", inventoryHandler.TransferInventory)
	inventories.POST("/:id/dispose", inventoryHandler.DisposeInventoryItems)
	inventories.POST("/:id/reconcile", inventoryHandler.ReconcileInventory)

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

	// Unit routes
	units := api.Group("/units")
	units.GET("", unitHandler.ListUnits)
	units.GET("/:id", unitHandler.GetUnit)
	units.POST("", unitHandler.CreateUnit)
	units.PUT("/:id", unitHandler.UpdateUnit)
	units.DELETE("/:id", unitHandler.DeleteUnit)

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
	purchaseOrders.PUT("/:id", purchaseOrderHandler.UpdatePurchaseOrder)
	purchaseOrders.PUT("/:id/receive", purchaseOrderHandler.ReceiveInventory)
	purchaseOrders.PUT("/:id/status", purchaseOrderHandler.UpdatePurchaseOrderStatus)

	// Payment Receipt Form routes
	paymentReceiptForms := api.Group("/payment-receipt-forms")
	paymentReceiptForms.GET("", paymentReceiptFormHandler.ListPaymentReceiptForms)
	paymentReceiptForms.POST("", paymentReceiptFormHandler.CreatePaymentReceiptForm)
	paymentReceiptForms.GET("/:id", paymentReceiptFormHandler.GetPaymentReceiptForm)
	paymentReceiptForms.POST("/:id/submit", paymentReceiptFormHandler.SubmitPaymentReceiptForm)
	paymentReceiptForms.PUT("/:id/approve", paymentReceiptFormHandler.ApprovePaymentReceiptForm)
	paymentReceiptForms.PUT("/:id/reject", paymentReceiptFormHandler.RejectPaymentReceiptForm)
	paymentReceiptForms.DELETE("/:id", paymentReceiptFormHandler.DeletePaymentReceiptForm)
	paymentReceiptForms.GET("/pending", paymentReceiptFormHandler.LatestPendingPaymentReceiptFormStream)

	// Excel routes
	excel := api.Group("/excel")
	excel.POST("/verify", excelHandler.VerifyFileAndSheet)

	// Settings routes
	settings := api.Group("/settings")
	settings.GET("", settingsHandler.GetAllSettings)
	settings.GET("/:key", settingsHandler.GetSetting)
	settings.POST("/:key", settingsHandler.SetSetting)
	settings.DELETE("/:key", settingsHandler.DeleteSetting)

	// Revenue Expense routes
	revenueExpenses := api.Group("/revenue-expenses")
	revenueExpenses.POST("/finalize", revenueExpenseHandler.FinalizeRevenueExpense)

	// Reports routes
	reports := api.Group("/reports")
	reports.GET("/inventory-summary", inventoryHandler.GetInventorySummary)
	reports.GET("/low-stock", inventoryItemHandler.GetLowStockItems)
	reports.GET("/purchase-summary", purchaseOrderHandler.GetPurchaseSummary)

	return e, nil
}

// createAuthMiddleware creates auth middleware that works with the interface
func createAuthMiddleware(firebaseAuth auth.FirebaseAuthInterface) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			requestToken := c.Request().URL.Query().Get("token")
			if requestToken == "" {
				requestToken = c.Request().Header.Get("Authorization")
			}

			if requestToken == "" {
				return c.JSON(401, map[string]string{"error": "Authorization header required"})
			}

			tokenString := strings.TrimPrefix(requestToken, "Bearer ")

			// Verify token using interface
			ctx := context.Background()
			token, err := firebaseAuth.VerifyToken(ctx, tokenString)
			if err != nil {
				return c.JSON(401, map[string]string{"error": "Invalid token"})
			}

			// Extract user information from token and set in context
			c.Set(pkg.AuthContextKeyUserID, token.UID)
			c.Set(pkg.AuthContextKeyUserEmail, token.Claims["email"])

			reqCtx := c.Request().Context()
			reqCtx = context.WithValue(reqCtx, pkg.AuthContextKeyUserID, token.UID)
			if email, ok := token.Claims["email"].(string); ok {
				reqCtx = pkg.WithUserEmail(reqCtx, email)
			}

			if name, ok := token.Claims["name"].(string); ok {
				c.Set("user_name", name)
				reqCtx = context.WithValue(reqCtx, pkg.AuthContextKeyUserName, name)
			}

			c.SetRequest(c.Request().WithContext(reqCtx))
			return next(c)
		}
	}
}
