package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cim-backend/internal/auth"
	"cim-backend/internal/config"
	"cim-backend/internal/handlers"
	"cim-backend/internal/middleware"
	"cim-backend/internal/repository"
	"cim-backend/internal/services"
	"cim-backend/pkg"
	"cim-backend/pkg/log"

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
) (*echo.Echo, error) {
	// Initialize Casbin service for authorization
	casbinService, err := auth.NewCasbinService(db, cfg.Casbin)
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
	menuRepo := repository.NewMenuRepository(db)
	menuItemRepo := repository.NewMenuItemRepository(db)
	saleOrderRepo := repository.NewSaleOrderRepository(db)

	// Initialize S3 client for R2
	s3Client, err := services.NewS3Client(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize S3 client: %w", err)
	}

	// Initialize services
	fileStorageService := services.NewFileStorageService(cfg, s3Client)
	userService := services.NewUserService(userRepo, casbinService)
	supplierService := services.NewSupplierService(supplierRepo)
	settingsService := services.NewSettingsService(settingsRepo)
	unitService := services.NewUnitService(unitRepo, productRepo)
	productService := services.NewProductService(productRepo, supplierRepo, unitRepo, unitService, settingsService)
	inventoryService := services.NewInventoryService(inventoryRepo, inventoryItemRepo, inventorySubmissionRepo, productRepo, fileStorageService, db)
	inventoryItemService := services.NewInventoryItemService(inventoryItemRepo, inventoryRepo, productRepo)
	excelService := services.NewExcelService(productRepo, inventoryRepo, paymentReceiptFormRepo, settingsService)
	purchaseOrderService := services.NewPurchaseOrderService(purchaseOrderRepo, paymentReceiptFormRepo, unitRepo, productRepo, inventoryService, excelService, settingsService, db, supplierRepo, inventoryRepo, unitService, productService)
	paymentReceiptFormService := services.NewPaymentReceiptFormService(paymentReceiptFormRepo, db, settingsService)
	menuService := services.NewMenuService(menuRepo, menuItemRepo, inventoryRepo)
	menuItemService := services.NewMenuItemService(menuItemRepo, menuRepo, productRepo)
	saleOrderService := services.NewSaleOrderService(saleOrderRepo, db)

	// Initialize handlers
	userHandler := handlers.NewUserHandler(userService, firebaseAuth)
	supplierHandler := handlers.NewSupplierHandler(supplierService)
	unitHandler := handlers.NewUnitHandler(unitService)
	productHandler := handlers.NewProductHandler(productService)
	inventoryHandler := handlers.NewInventoryHandler(inventoryService)
	inventoryItemHandler := handlers.NewInventoryItemHandler(inventoryItemService)
	purchaseOrderHandler := handlers.NewPurchaseOrderHandler(purchaseOrderRepo, purchaseOrderService, paymentReceiptFormService, fileStorageService)
	excelHandler := handlers.NewExcelHandler(excelService)
	settingsHandler := handlers.NewSettingsHandler(settingsService)
	paymentReceiptFormHandler := handlers.NewPaymentReceiptFormHandler(paymentReceiptFormService, purchaseOrderService, settingsService)
	revenueExpenseHandler := handlers.NewRevenueExpenseHandler(excelService, settingsService)
	menuHandler := handlers.NewMenuHandler(menuService)
	menuItemHandler := handlers.NewMenuItemHandler(menuItemService)
	saleOrderHandler := handlers.NewSaleOrderHandler(saleOrderService)

	// Initialize Echo
	e := echo.New()

	// Set custom error handler
	e.HTTPErrorHandler = middleware.CustomErrorHandler

	// Middleware
	e.Use(echoMiddleware.RequestLoggerWithConfig(echoMiddleware.RequestLoggerConfig{
		LogURI:    true,
		LogStatus: true,
		LogValuesFunc: func(c echo.Context, values echoMiddleware.RequestLoggerValues) error {
			req := c.Request()
			res := c.Response()

			fields := logrus.Fields{
				"time":       values.StartTime.Format(time.RFC3339),
				"remote_ip":  c.RealIP(),
				"host":       req.Host,
				"method":     values.Method,
				"uri":        values.URI,
				"path":       req.URL.Path,
				"route":      c.Path(),
				"protocol":   req.Proto,
				"status":     values.Status,
				"latency":    values.Latency.String(),
				"latency_ms": values.Latency.Milliseconds(),
				"bytes_in":   req.Header.Get(echo.HeaderContentLength),
				"bytes_out":  res.Size,
			}

			// Add optional fields if present
			if referer := req.Referer(); referer != "" {
				fields["referer"] = referer
			}
			if userAgent := req.UserAgent(); userAgent != "" {
				fields["user_agent"] = userAgent
			}
			if requestID := req.Header.Get(echo.HeaderXRequestID); requestID != "" {
				fields["request_id"] = requestID
			} else if requestID := res.Header().Get(echo.HeaderXRequestID); requestID != "" {
				fields["request_id"] = requestID
			}

			logger := log.WithFields(fields)

			if values.Error != nil || values.Status >= 400 {
				if values.Error != nil {
					logger.WithError(values.Error).Error("request error")
				} else {
					logger.Error("request error")
				}
			} else {
				logger.Info("request success")
			}
			return nil
		},
	}))
	e.Use(echoMiddleware.Recover())
	e.Use(echoMiddleware.CORSWithConfig(echoMiddleware.CORSConfig{
		AllowOrigins: cfg.Server.CORSAllowedOrigins,
		AllowMethods: []string{echo.GET, echo.POST, echo.PUT, echo.DELETE, echo.OPTIONS},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
	}))

	e.Use(middleware.LanguageMiddleware())

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
	products.GET("/export-csv", productHandler.ExportProductsCSV)
	products.GET("/export-excel", productHandler.ExportProductsExcel)
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
	inventories.GET("/:id/export/monthly-transaction", inventoryHandler.ExportMonthlyTransactionReport)

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
	purchaseOrders.GET("/:id", purchaseOrderHandler.GetPurchaseOrder)
	purchaseOrders.POST("", purchaseOrderHandler.CreatePurchaseOrder)
	purchaseOrders.PUT("/:id", purchaseOrderHandler.UpdatePurchaseOrder)
	purchaseOrders.PUT("/:id/receive", purchaseOrderHandler.ReceiveInventory)
	purchaseOrders.PUT("/:id/status", purchaseOrderHandler.UpdatePurchaseOrderStatus)
	purchaseOrders.POST("/:id/revenue-expense/retry", purchaseOrderHandler.RetryQueueRevenueExpenseRequest)
	purchaseOrders.POST("/upload", purchaseOrderHandler.UploadPurchaseOrderFile)
	purchaseOrders.POST("/upload-files/:uid/process", purchaseOrderHandler.ProcessImportPurchaseOrder)

	// Payment Receipt Form routes
	paymentReceiptForms := api.Group("/payment-receipt-forms")
	paymentReceiptForms.GET("", paymentReceiptFormHandler.ListPaymentReceiptForms)
	paymentReceiptForms.POST("", paymentReceiptFormHandler.CreatePaymentReceiptForm)
	paymentReceiptForms.GET("/:id", paymentReceiptFormHandler.GetPaymentReceiptForm)
	paymentReceiptForms.PUT("/:id", paymentReceiptFormHandler.UpdatePaymentReceiptForm)
	paymentReceiptForms.POST("/:id/submit", paymentReceiptFormHandler.SubmitPaymentReceiptForm)
	paymentReceiptForms.PUT("/:id/approve", paymentReceiptFormHandler.ApprovePaymentReceiptForm)
	paymentReceiptForms.PUT("/:id/reject", paymentReceiptFormHandler.RejectPaymentReceiptForm)
	paymentReceiptForms.DELETE("/:id", paymentReceiptFormHandler.DeletePaymentReceiptForm)
	paymentReceiptForms.GET("/pending", paymentReceiptFormHandler.LatestPendingPaymentReceiptFormStream)
	paymentReceiptForms.POST("/:id/notify", paymentReceiptFormHandler.NotifyPaymentReceiptForm)

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
	revenueExpenses.POST("/finalize/:date", revenueExpenseHandler.FinalizeRevenueExpenseByDate)

	// Reports routes
	reports := api.Group("/reports")
	reports.GET("/inventory-summary", inventoryHandler.GetInventorySummary)
	reports.GET("/low-stock", inventoryItemHandler.GetLowStockItems)
	reports.GET("/purchase-summary", purchaseOrderHandler.GetPurchaseSummary)

	// Menu routes
	menus := api.Group("/menus")
	menus.GET("", menuHandler.ListMenus)
	menus.POST("", menuHandler.CreateMenu)
	menus.GET("/:id", menuHandler.GetMenu)
	menus.PUT("/:id", menuHandler.UpdateMenu)
	menus.DELETE("/:id", menuHandler.DeleteMenu)

	// Menu item routes
	menuItems := api.Group("/menu-items")
	menuItems.GET("", menuItemHandler.ListMenuItems)
	menuItems.POST("", menuItemHandler.CreateMenuItem)
	menuItems.GET("/:id", menuItemHandler.GetMenuItem)
	menuItems.PUT("/:id", menuItemHandler.UpdateMenuItem)
	menuItems.DELETE("/:id", menuItemHandler.DeleteMenuItem)

	// Sale Order routes
	saleOrders := api.Group("/sale-orders")
	saleOrders.GET("", saleOrderHandler.ListSaleOrders)
	saleOrders.POST("", saleOrderHandler.CreateSaleOrder)
	saleOrders.GET("/:id", saleOrderHandler.GetSaleOrder)
	saleOrders.PUT("/:id", saleOrderHandler.UpdateSaleOrder)
	saleOrders.PUT("/:id/status", saleOrderHandler.UpdateSaleOrderStatus)

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
			c.Set(pkg.AuthContextKeyUserID.String(), token.UID)
			c.Set(pkg.AuthContextKeyUserEmail.String(), token.Claims["email"])

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
