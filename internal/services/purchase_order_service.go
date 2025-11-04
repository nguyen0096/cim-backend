package services

import (
	"cim-backend/internal/config"
	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

//go:generate mockery --name=PurchaseOrderService --structname=PurchaseOrderService --output=../mocks/servicemocks --outpkg=servicemocks
type PurchaseOrderService interface {
	CreatePurchaseOrder(ctx context.Context, purchaseOrder *models.PurchaseOrder) error
	GetPurchaseOrderByID(id uint) (*models.PurchaseOrder, error)
	UpdatePurchaseOrderStatus(ctx context.Context, id uint, status models.PurchaseOrderStatus) error
	DeletePurchaseOrder(id uint) error
	ListPurchaseOrders(ctx context.Context, params models.ListParams) (*models.PaginationResult[models.PurchaseOrder], error)
	GetPurchaseOrdersByStatus(status string) ([]models.PurchaseOrder, error)
	ReceivePurchaseOrder(ctx context.Context, id uint) error
	UpdatePurchaseOrderItemStatus(ctx context.Context, purchaseOrderID, itemID uint, status models.PurchaseOrderItemStatus) (*dto.UpdatePurchaseOrderItemStatusResponse, error)

	// V1
	UpdatePurchaseOrder(ctx context.Context, id uint, req dto.UpdatePurchaseOrderRequest) (*models.PurchaseOrder, error)
	ReceiveInventory(ctx context.Context, req dto.UpdatePurchaseOrderDeliveryStatusRequest) (*models.PurchaseOrder, error)
}

type purchaseOrderService struct {
	purchaseOrderRepo          repository.PurchaseOrderRepository
	paymentReceiptFormRepo     repository.PaymentReceiptFormRepository
	inventoryService           InventoryService
	excelService               ExcelService
	settingsService            SettingsService
	db                         *gorm.DB
	logger                     *logrus.Logger
	revenueExpenseRequestQueue chan revenueExpenseRequest
}

// revenueExpenseRequest represents a queued request to process revenue expense
type revenueExpenseRequest struct {
	ctx                  context.Context
	paymentReceiptFormID uint
}

func NewPurchaseOrderService(
	purchaseOrderRepo repository.PurchaseOrderRepository,
	paymentReceiptFormRepo repository.PaymentReceiptFormRepository,
	inventoryService InventoryService,
	excelService ExcelService,
	settingsService SettingsService,
	db *gorm.DB,
	logger *logrus.Logger,
) PurchaseOrderService {
	// Create a buffered channel to queue revenue expense requests
	// Buffer size of 100 allows queuing up to 100 requests without blocking
	requestQueue := make(chan revenueExpenseRequest, 100)

	service := &purchaseOrderService{
		purchaseOrderRepo:          purchaseOrderRepo,
		paymentReceiptFormRepo:     paymentReceiptFormRepo,
		inventoryService:           inventoryService,
		excelService:               excelService,
		settingsService:            settingsService,
		db:                         db,
		logger:                     logger,
		revenueExpenseRequestQueue: requestQueue,
	}

	// Start the worker goroutine to process revenue expense requests serially
	go service.revenueExpenseWorker()

	logger.Info("Purchase order service initialized with revenue expense worker")

	return service
}

// generatePurchaseOrderNumber generates a unique purchase order number
func (s *purchaseOrderService) generatePurchaseOrderNumber() (string, error) {
	now := time.Now()

	// Generate 2-character random alphanumeric suffix
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	suffix := make([]byte, 2)
	if _, err := rand.Read(suffix); err != nil {
		return "", fmt.Errorf("failed to generate random suffix: %w", err)
	}

	for i := range suffix {
		suffix[i] = charset[suffix[i]%byte(len(charset))]
	}

	// Format: PO-YYMMDD-HHMMSS-XX
	return fmt.Sprintf("PO-%s-%s",
		now.Format("060102-150405"),
		string(suffix)), nil
}

// CreatePurchaseOrder creates a new purchase order with auto-generated order number
func (s *purchaseOrderService) CreatePurchaseOrder(ctx context.Context, purchaseOrder *models.PurchaseOrder) error {
	s.logger.WithFields(logrus.Fields{
		"operation":    "CreatePurchaseOrder",
		"order_number": purchaseOrder.OrderNumber,
	}).Info("Creating new purchase order")

	// Generate order number if not provided
	if purchaseOrder.OrderNumber == "" {
		orderNumber, err := s.generatePurchaseOrderNumber()
		if err != nil {
			s.logger.WithFields(logrus.Fields{
				"operation": "CreatePurchaseOrder",
				"error":     err,
			}).Error("Failed to generate purchase order number")
			return fmt.Errorf("failed to generate purchase order number: %w", err)
		}
		purchaseOrder.OrderNumber = orderNumber
		s.logger.WithFields(logrus.Fields{
			"operation":    "CreatePurchaseOrder",
			"order_number": orderNumber,
		}).Info("Generated purchase order number")
	}

	// Set status to order_placed for new purchase orders
	purchaseOrder.Status = models.PurchaseOrderStatusOrderPlaced

	err := s.purchaseOrderRepo.Create(ctx, purchaseOrder)
	if err != nil {
		s.logger.WithFields(logrus.Fields{
			"operation":    "CreatePurchaseOrder",
			"order_number": purchaseOrder.OrderNumber,
			"error":        err,
		}).Error("Failed to create purchase order")
		return err
	}

	s.logger.WithFields(logrus.Fields{
		"operation":         "CreatePurchaseOrder",
		"order_number":      purchaseOrder.OrderNumber,
		"purchase_order_id": purchaseOrder.ID,
	}).Info("Successfully created purchase order")

	return nil
}

func (s *purchaseOrderService) GetPurchaseOrderByID(id uint) (*models.PurchaseOrder, error) {
	s.logger.WithFields(logrus.Fields{
		"operation":         "GetPurchaseOrderByID",
		"purchase_order_id": id,
	}).Info("Retrieving purchase order by ID")

	purchaseOrder, err := s.purchaseOrderRepo.GetByID(id)
	if err != nil {
		s.logger.WithFields(logrus.Fields{
			"operation":         "GetPurchaseOrderByID",
			"purchase_order_id": id,
			"error":             err,
		}).Error("Failed to retrieve purchase order")
		return nil, err
	}

	// Calculate total amount based on items
	purchaseOrder.TotalAmount = purchaseOrder.CalculateTotalAmount()

	s.logger.WithFields(logrus.Fields{
		"operation":         "GetPurchaseOrderByID",
		"purchase_order_id": id,
		"order_number":      purchaseOrder.OrderNumber,
		"total_amount":      purchaseOrder.TotalAmount,
	}).Info("Successfully retrieved purchase order")

	return purchaseOrder, nil
}

func (s *purchaseOrderService) UpdatePurchaseOrderStatus(ctx context.Context, id uint, status models.PurchaseOrderStatus) error {
	s.logger.WithFields(logrus.Fields{
		"operation":         "UpdatePurchaseOrderStatus",
		"purchase_order_id": id,
		"status":            status,
	}).Info("Updating purchase order status")

	if status == models.PurchaseOrderStatusCompleted {
		// Check if there is an approved payment receipt form before completing
		approvedForm, err := s.paymentReceiptFormRepo.GetLatestPaymentReceiptForm(ctx, id, models.PaymentReceiptFormStatusApproved)
		if err != nil {
			s.logger.WithFields(logrus.Fields{
				"operation":         "UpdatePurchaseOrderStatus",
				"purchase_order_id": id,
				"error":             err,
			}).Error("Failed to check for approved payment receipt form")
			return fmt.Errorf("failed to check for approved payment receipt form: %w", err)
		}

		if approvedForm == nil {
			s.logger.WithFields(logrus.Fields{
				"operation":         "UpdatePurchaseOrderStatus",
				"purchase_order_id": id,
			}).Warn("Cannot complete purchase order: no approved payment receipt form found")
			return pkg.NewAppError(pkg.ErrorCodeValidation, "Cannot complete purchase order: no approved payment receipt form found", nil)
		}

		go s.queueRevenueExpenseRequest(approvedForm.ID)
	}

	err := s.purchaseOrderRepo.UpdateStatus(ctx, id, status)
	if err != nil {
		s.logger.WithFields(logrus.Fields{
			"operation":         "UpdatePurchaseOrderStatus",
			"purchase_order_id": id,
			"status":            status,
			"error":             err,
		}).Error("Failed to update purchase order status")
		return err
	}

	s.logger.WithFields(logrus.Fields{
		"operation":         "UpdatePurchaseOrderStatus",
		"purchase_order_id": id,
		"status":            status,
	}).Info("Successfully updated purchase order status")

	return nil
}

func (s *purchaseOrderService) DeletePurchaseOrder(id uint) error {
	s.logger.WithFields(logrus.Fields{
		"operation":         "DeletePurchaseOrder",
		"purchase_order_id": id,
	}).Info("Deleting purchase order")

	err := s.purchaseOrderRepo.Delete(id)
	if err != nil {
		s.logger.WithFields(logrus.Fields{
			"operation":         "DeletePurchaseOrder",
			"purchase_order_id": id,
			"error":             err,
		}).Error("Failed to delete purchase order")
		return err
	}

	s.logger.WithFields(logrus.Fields{
		"operation":         "DeletePurchaseOrder",
		"purchase_order_id": id,
	}).Info("Successfully deleted purchase order")

	return nil
}

// ListPurchaseOrders retrieves purchase orders with search and pagination
func (s *purchaseOrderService) ListPurchaseOrders(ctx context.Context, params models.ListParams) (*models.PaginationResult[models.PurchaseOrder], error) {
	s.logger.WithFields(logrus.Fields{
		"operation": "ListPurchaseOrders",
		"page":      params.Page,
		"limit":     params.Limit,
		"search":    params.Search,
		"sort":      params.Sort,
		"order":     params.Order,
	}).Info("Listing purchase orders with pagination")

	// Validate and set defaults for pagination parameters
	params.ValidateAndSetDefaults()

	// Get data and count from repository
	purchaseOrders, total, err := s.purchaseOrderRepo.List(ctx, params)
	if err != nil {
		s.logger.WithFields(logrus.Fields{
			"operation": "ListPurchaseOrders",
			"error":     err,
		}).Error("Failed to list purchase orders")
		return nil, err
	}

	// Calculate total amount for each purchase order based on items
	for i := range purchaseOrders {
		purchaseOrders[i].TotalAmount = purchaseOrders[i].CalculateTotalAmount()
	}

	// Create pagination result
	result := models.NewPaginationResult(purchaseOrders, total, params.Page, params.Limit)

	s.logger.WithFields(logrus.Fields{
		"operation":      "ListPurchaseOrders",
		"total_count":    total,
		"returned_count": len(purchaseOrders),
		"page":           params.Page,
		"limit":          params.Limit,
	}).Info("Successfully listed purchase orders")

	return result, nil
}

func (s *purchaseOrderService) GetPurchaseOrdersByStatus(status string) ([]models.PurchaseOrder, error) {
	s.logger.WithFields(logrus.Fields{
		"operation": "GetPurchaseOrdersByStatus",
		"status":    status,
	}).Info("Retrieving purchase orders by status")

	purchaseOrders, err := s.purchaseOrderRepo.GetByStatus(status)
	if err != nil {
		s.logger.WithFields(logrus.Fields{
			"operation": "GetPurchaseOrdersByStatus",
			"status":    status,
			"error":     err,
		}).Error("Failed to retrieve purchase orders by status")
		return nil, err
	}

	// Calculate total amount for each purchase order based on items
	for i := range purchaseOrders {
		purchaseOrders[i].TotalAmount = purchaseOrders[i].CalculateTotalAmount()
	}

	s.logger.WithFields(logrus.Fields{
		"operation": "GetPurchaseOrdersByStatus",
		"status":    status,
		"count":     len(purchaseOrders),
	}).Info("Successfully retrieved purchase orders by status")

	return purchaseOrders, nil
}

func (s *purchaseOrderService) ReceivePurchaseOrder(ctx context.Context, id uint) error {
	s.logger.WithFields(logrus.Fields{
		"operation":         "ReceivePurchaseOrder",
		"purchase_order_id": id,
	}).Info("Receiving purchase order")

	purchaseOrder, err := s.purchaseOrderRepo.GetByID(id)
	if err != nil {
		s.logger.WithFields(logrus.Fields{
			"operation":         "ReceivePurchaseOrder",
			"purchase_order_id": id,
			"error":             err,
		}).Error("Failed to get purchase order for receiving")
		return err
	}

	// Update status to received
	purchaseOrder.Status = models.PurchaseOrderStatusCompleted
	if err := s.purchaseOrderRepo.Update(ctx, purchaseOrder); err != nil {
		s.logger.WithFields(logrus.Fields{
			"operation":         "ReceivePurchaseOrder",
			"purchase_order_id": id,
			"error":             err,
		}).Error("Failed to update purchase order status to completed")
		return fmt.Errorf("failed to update purchase order: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"operation":         "ReceivePurchaseOrder",
		"purchase_order_id": id,
		"items_count":       len(purchaseOrder.Items),
	}).Info("Adding inventory for received items")

	// Add inventory for each item
	for _, item := range purchaseOrder.Items {
		if err := s.inventoryService.AddInventory(
			ctx,
			*item.ProductID,
			item.Quantity,
			purchaseOrder.ID,
			"purchase_order",
			"Received from purchase order",
		); err != nil {
			s.logger.WithFields(logrus.Fields{
				"operation":         "ReceivePurchaseOrder",
				"purchase_order_id": id,
				"product_id":        *item.ProductID,
				"quantity":          item.Quantity,
				"error":             err,
			}).Error("Failed to add inventory for received item")
			return err
		}
	}

	s.logger.WithFields(logrus.Fields{
		"operation":         "ReceivePurchaseOrder",
		"purchase_order_id": id,
		"order_number":      purchaseOrder.OrderNumber,
		"items_count":       len(purchaseOrder.Items),
	}).Info("Successfully received purchase order and added inventory")

	return nil
}

// UpdatePurchaseOrderItemStatus updates the status of a purchase order item
func (s *purchaseOrderService) UpdatePurchaseOrderItemStatus(ctx context.Context, purchaseOrderID, itemID uint, status models.PurchaseOrderItemStatus) (*dto.UpdatePurchaseOrderItemStatusResponse, error) {
	s.logger.WithFields(logrus.Fields{
		"operation":         "UpdatePurchaseOrderItemStatus",
		"purchase_order_id": purchaseOrderID,
		"item_id":           itemID,
		"new_status":        status,
	}).Info("Updating purchase order item status")

	var result *dto.UpdatePurchaseOrderItemStatusResponse

	// Wrap the operations in a transaction
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Update the item status using the transaction
		err := tx.WithContext(ctx).Model(&models.PurchaseOrderItem{}).
			Where("id = ? AND purchase_order_id = ?", itemID, purchaseOrderID).
			Update("status", status).Error
		if err != nil {
			s.logger.WithFields(logrus.Fields{
				"operation":         "UpdatePurchaseOrderItemStatus",
				"purchase_order_id": purchaseOrderID,
				"item_id":           itemID,
				"error":             err,
			}).Error("Failed to update purchase order item status in transaction")
			return fmt.Errorf("failed to update purchase order item status: %w", err)
		}

		// Determine the order status based on item status
		var orderStatus models.PurchaseOrderStatus = models.PurchaseOrderStatusPartiallyDelivered
		// if status == models.PurchaseOrderItemStatusDelivered {
		// 	if s.purchaseOrderRepo.AnyDeliveringItem(ctx, purchaseOrderID) {
		// 		orderStatus = models.PurchaseOrderStatusPartiallyDelivered
		// 		err = s.purchaseOrderRepo.UpdateStatus(ctx, purchaseOrderID, orderStatus)
		// 	} else {
		// 		orderStatus = models.PurchaseOrderStatusFullyDelivered
		// 		err = s.purchaseOrderRepo.UpdateStatus(ctx, purchaseOrderID, orderStatus)
		// 	}
		// 	if err != nil {
		// 		return nil, fmt.Errorf("failed to update purchase order status: %w", err)
		// 	}
		// }

		// Update the purchase order status using the transaction
		err = tx.WithContext(ctx).Model(&models.PurchaseOrder{}).
			Where("id = ?", purchaseOrderID).
			Update("status", orderStatus).Error
		if err != nil {
			s.logger.WithFields(logrus.Fields{
				"operation":         "UpdatePurchaseOrderItemStatus",
				"purchase_order_id": purchaseOrderID,
				"order_status":      orderStatus,
				"error":             err,
			}).Error("Failed to update purchase order status in transaction")
			return fmt.Errorf("failed to update purchase order status: %w", err)
		}

		// Set the result
		result = &dto.UpdatePurchaseOrderItemStatusResponse{
			ItemStatus:  status,
			OrderStatus: orderStatus,
		}

		// Return nil to commit the transaction
		return nil
	})

	if err != nil {
		s.logger.WithFields(logrus.Fields{
			"operation":         "UpdatePurchaseOrderItemStatus",
			"purchase_order_id": purchaseOrderID,
			"item_id":           itemID,
			"error":             err,
		}).Error("Transaction failed for purchase order item status update")
		return nil, err
	}

	s.logger.WithFields(logrus.Fields{
		"operation":         "UpdatePurchaseOrderItemStatus",
		"purchase_order_id": purchaseOrderID,
		"item_id":           itemID,
		"item_status":       result.ItemStatus,
		"order_status":      result.OrderStatus,
	}).Info("Successfully updated purchase order item status")

	return result, nil
}

// revenueExpenseWorker processes revenue expense requests from the queue serially
func (s *purchaseOrderService) revenueExpenseWorker() {
	s.logger.Info("Revenue expense worker started")

	for req := range s.revenueExpenseRequestQueue {
		s.logger.WithFields(logrus.Fields{
			"operation":               "revenueExpenseWorker",
			"payment_receipt_form_id": req.paymentReceiptFormID,
			"queue_length":            len(s.revenueExpenseRequestQueue),
		}).Info("Processing revenue expense request from queue")

		// Process the request with retry mechanism
		s.handleRevenueExpenseAsyncBySettingsWithRetry(req.paymentReceiptFormID)
	}

	s.logger.Warn("Revenue expense worker stopped - channel closed")
}

// queueRevenueExpenseRequest queues a revenue expense request for serial processing
func (s *purchaseOrderService) queueRevenueExpenseRequest(paymentReceiptFormID uint) {
	req := revenueExpenseRequest{
		paymentReceiptFormID: paymentReceiptFormID,
	}

	s.revenueExpenseRequestQueue <- req
}

// handleRevenueExpenseAsyncBySettingsWithRetry wraps the async handler with retry mechanism
func (s *purchaseOrderService) handleRevenueExpenseAsyncBySettingsWithRetry(paymentReceiptFormID uint) {
	const (
		maxRetries    = 3
		initialDelay  = 10 * time.Second
		maxDelay      = 60 * time.Second
		backoffFactor = 2.0
	)

	var lastErr error
	delay := initialDelay

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Recover from panics to prevent goroutine crashes
		func() {
			defer func() {
				if r := recover(); r != nil {
					lastErr = fmt.Errorf("panic recovered: %v", r)
					s.logger.WithFields(logrus.Fields{
						"operation":               "handleRevenueExpenseAsyncBySettingsWithRetry",
						"payment_receipt_form_id": paymentReceiptFormID,
						"attempt":                 attempt,
						"max_retries":             maxRetries,
						"panic":                   r,
						"stack_trace":             string(debug.Stack()),
					}).Error("Panic occurred in revenue expense handler")
				}
			}()

			s.logger.WithFields(logrus.Fields{
				"operation":               "handleRevenueExpenseAsyncBySettingsWithRetry",
				"payment_receipt_form_id": paymentReceiptFormID,
				"attempt":                 attempt,
				"max_retries":             maxRetries,
			}).Info("Attempting to process revenue expense")

			s.handleRevenueExpenseAsyncBySettings(context.Background(), paymentReceiptFormID)
			lastErr = nil // Success - clear any previous error
		}()

		// If successful, exit
		if lastErr == nil {
			s.logger.WithFields(logrus.Fields{
				"operation":               "handleRevenueExpenseAsyncBySettingsWithRetry",
				"payment_receipt_form_id": paymentReceiptFormID,
				"attempt":                 attempt,
			}).Info("Successfully processed revenue expense")
			return
		}

		// If this was the last attempt, log final failure
		if attempt == maxRetries {
			s.logger.WithFields(logrus.Fields{
				"operation":               "handleRevenueExpenseAsyncBySettingsWithRetry",
				"payment_receipt_form_id": paymentReceiptFormID,
				"attempts":                attempt,
				"error":                   lastErr,
			}).Error("Failed to process revenue expense after all retry attempts")
			return
		}

		// Log retry attempt with delay
		s.logger.WithFields(logrus.Fields{
			"operation":               "handleRevenueExpenseAsyncBySettingsWithRetry",
			"payment_receipt_form_id": paymentReceiptFormID,
			"attempt":                 attempt,
			"next_attempt":            attempt + 1,
			"delay_seconds":           delay.Seconds(),
			"error":                   lastErr,
		}).Warn("Retrying revenue expense processing after failure")

		// Wait before retrying with exponential backoff
		time.Sleep(delay)

		// Increase delay for next retry with exponential backoff
		delay = time.Duration(float64(delay) * backoffFactor)
		if delay > maxDelay {
			delay = maxDelay
		}
	}
}

// handleRevenueExpenseAsyncBySettings determines the file type from settings and calls the appropriate async handler
func (s *purchaseOrderService) handleRevenueExpenseAsyncBySettings(ctx context.Context, paymentReceiptFormID uint) {
	// Get settings to determine file type
	settings, err := s.settingsService.GetSetting(ctx, config.RevenueExpenseExcelSettingsKey)
	if err != nil {
		s.logger.WithFields(logrus.Fields{
			"operation":               "handleRevenueExpenseAsyncBySettings",
			"payment_receipt_form_id": paymentReceiptFormID,
			"error":                   err,
		}).Error("Failed to get revenue expense settings")
		return
	}

	if settings == nil {
		s.logger.WithFields(logrus.Fields{
			"operation":               "handleRevenueExpenseAsyncBySettings",
			"payment_receipt_form_id": paymentReceiptFormID,
		}).Error("Revenue expense settings not configured")
		return
	}

	var settingsValue map[string]interface{}
	if err := json.Unmarshal([]byte(settings.Value), &settingsValue); err != nil {
		s.logger.WithFields(logrus.Fields{
			"operation":               "handleRevenueExpenseAsyncBySettings",
			"payment_receipt_form_id": paymentReceiptFormID,
			"error":                   err,
		}).Error("Failed to parse revenue expense settings")
		return
	}

	filePath, ok := settingsValue["filePath"].(string)
	if !ok || filePath == "" {
		s.logger.WithFields(logrus.Fields{
			"operation":               "handleRevenueExpenseAsyncBySettings",
			"payment_receipt_form_id": paymentReceiptFormID,
		}).Error("filePath not found in revenue expense settings")
		return
	}

	// Detect if filePath is a Google Sheets URL or local file path
	isGoogleSheets := strings.Contains(filePath, "docs.google.com/spreadsheets")

	if isGoogleSheets {
		s.logger.WithFields(logrus.Fields{
			"operation":               "handleRevenueExpenseAsyncBySettings",
			"payment_receipt_form_id": paymentReceiptFormID,
			"file_type":               "google_sheets",
		}).Info("Detected Google Sheets, calling handleRevenueExpenseGoogleSheetsAsync")
		s.handleRevenueExpenseGoogleSheetsAsync(ctx, paymentReceiptFormID, settingsValue)
	} else {
		s.logger.WithFields(logrus.Fields{
			"operation":               "handleRevenueExpenseAsyncBySettings",
			"payment_receipt_form_id": paymentReceiptFormID,
			"file_type":               "local_file",
		}).Info("Detected local file, calling handleRevenueExpenseAsync")
		s.handleRevenueExpenseAsync(ctx, paymentReceiptFormID, settingsValue)
	}
}

// handleRevenueExpenseAsync handles revenue expense excel file operations asynchronously
func (s *purchaseOrderService) handleRevenueExpenseAsync(ctx context.Context, paymentReceiptFormID uint, settingsValue map[string]interface{}) {
	startTime := time.Now()

	paymentReceiptForm, err := s.paymentReceiptFormRepo.GetByIDFull(ctx, paymentReceiptFormID)
	if err != nil {
		s.logger.WithFields(logrus.Fields{
			"operation":               "handleRevenueExpenseAsync",
			"payment_receipt_form_id": paymentReceiptFormID,
			"error":                   err,
		}).Error("Failed to get purchase order")
		return
	}
	logger := s.logger.WithFields(logrus.Fields{
		"operation":               "handleRevenueExpenseAsync",
		"payment_receipt_form_id": paymentReceiptForm.ID,
		"form_number":             paymentReceiptForm.FormNumber,
	})
	logger.Info("Starting async revenue expense processing")

	// Get and validate settings
	filePath, ok := settingsValue["filePath"].(string)
	if !ok || filePath == "" {
		s.logger.WithFields(logrus.Fields{
			"operation":               "handleRevenueExpenseAsync",
			"payment_receipt_form_id": paymentReceiptFormID,
		}).Error("filePath not found in revenue expense settings")
		return
	}
	sheetName, ok := settingsValue["sheetName"].(string)
	if !ok || sheetName == "" {
		s.logger.WithFields(logrus.Fields{
			"operation":               "handleRevenueExpenseAsync",
			"payment_receipt_form_id": paymentReceiptFormID,
		}).Error("sheetName not found in revenue expense settings")
		return
	}

	// Initialize excel file
	if err := s.excelService.InitializeRevenueExpenseFile(ctx, filePath); err != nil {
		duration := time.Since(startTime)
		logger.WithFields(logrus.Fields{
			"file_path":   filePath,
			"sheet_name":  sheetName,
			"duration_ms": duration.Milliseconds(),
			"error":       err,
		}).Error("Failed to initialize revenue expense excel file")
		return
	}

	// Create expense data and add to excel
	expensesData, cellColors := s.createExpenseData(paymentReceiptForm)
	if err := s.excelService.AddExpenses(ctx, sheetName, expensesData, cellColors); err != nil {
		duration := time.Since(startTime)
		logger.WithFields(logrus.Fields{
			"file_path":   filePath,
			"sheet_name":  sheetName,
			"duration_ms": duration.Milliseconds(),
			"error":       err,
		}).Error("Failed to add expense to revenue expense excel file")
		return
	}

	duration := time.Since(startTime)
	logger.WithFields(logrus.Fields{
		"duration_ms": duration.Milliseconds(),
	}).Info("Successfully added expense to revenue expense excel file")
}

// handleRevenueExpenseGoogleSheetsAsync handles revenue expense Google Sheets operations asynchronously
func (s *purchaseOrderService) handleRevenueExpenseGoogleSheetsAsync(ctx context.Context, paymentReceiptFormID uint, settingsValue map[string]interface{}) {
	startTime := time.Now()

	paymentReceiptForm, err := s.paymentReceiptFormRepo.GetByIDFull(ctx, paymentReceiptFormID)
	if err != nil {
		s.logger.WithFields(logrus.Fields{
			"operation":               "handleRevenueExpenseGoogleSheetsAsync",
			"payment_receipt_form_id": paymentReceiptFormID,
			"error":                   err,
		}).Error("Failed to get purchase order")
		return
	}

	logger := s.logger.WithFields(logrus.Fields{
		"operation":               "handleRevenueExpenseGoogleSheetsAsync",
		"payment_receipt_form_id": paymentReceiptForm.ID,
		"form_number":             paymentReceiptForm.FormNumber,
	})
	logger.Info("Starting async Google Sheets revenue expense processing")

	filePath, ok := settingsValue["filePath"].(string)
	if !ok || filePath == "" {
		s.logger.WithFields(logrus.Fields{
			"operation":               "handleRevenueExpenseGoogleSheetsAsync",
			"payment_receipt_form_id": paymentReceiptFormID,
		}).Error("filePath not found in revenue expense settings")
		return
	}

	spreadsheetID := extractSpreadsheetID(filePath)
	if spreadsheetID == "" {
		s.logger.WithFields(logrus.Fields{
			"operation":               "handleRevenueExpenseGoogleSheetsAsync",
			"payment_receipt_form_id": paymentReceiptFormID,
		}).Error("spreadsheetID not found in revenue expense settings")
		return
	}

	sheetName, ok := settingsValue["sheetName"].(string)
	if !ok || sheetName == "" {
		s.logger.WithFields(logrus.Fields{
			"operation":               "handleRevenueExpenseGoogleSheetsAsync",
			"payment_receipt_form_id": paymentReceiptFormID,
		}).Error("sheetName not found in revenue expense settings")
		return
	}

	// Initialize Google Sheets repository
	if err := s.excelService.InitializeRevenueExpenseGoogleSheets(ctx, spreadsheetID); err != nil {
		duration := time.Since(startTime)
		logger.WithFields(logrus.Fields{
			"spreadsheet_id": spreadsheetID,
			"sheet_name":     sheetName,
			"duration_ms":    duration.Milliseconds(),
			"error":          err,
		}).Error("Failed to initialize revenue expense Google Sheets")
		return
	}

	// Create expense data and add to Google Sheets
	logger.WithFields(logrus.Fields{
		"items_count":             len(paymentReceiptForm.PurchaseOrder.Items),
		"spreadsheet_id":          spreadsheetID,
		"sheet_name":              sheetName,
		"payment_receipt_form_id": paymentReceiptFormID,
	}).Info("Creating expense data from purchase order items")

	expensesData, cellColors := s.createExpenseData(paymentReceiptForm)
	if err := s.excelService.AddExpensesToGoogleSheets(ctx, sheetName, expensesData, cellColors); err != nil {
		duration := time.Since(startTime)
		logger.WithFields(logrus.Fields{
			"spreadsheet_id": spreadsheetID,
			"sheet_name":     sheetName,
			"duration_ms":    duration.Milliseconds(),
			"error":          err,
		}).Error("Failed to add expense to revenue expense Google Sheets")
		return
	}

	duration := time.Since(startTime)
	logger.WithFields(logrus.Fields{
		"duration_ms":    duration.Milliseconds(),
		"spreadsheet_id": spreadsheetID,
		"sheet_name":     sheetName,
	}).Info("Successfully added expense to revenue expense Google Sheets")
}

// extractSpreadsheetID extracts the spreadsheet ID from a URL or returns the input if it's already an ID
func extractSpreadsheetID(input string) string {
	// Add safety check for empty input
	if input == "" {
		return ""
	}

	// Check if input contains Google Sheets URL pattern
	if strings.Contains(input, "docs.google.com/spreadsheets") {
		// Extract ID from URL pattern: https://docs.google.com/spreadsheets/d/{SPREADSHEET_ID}/...
		input = strings.TrimLeft(input, "https://")
		input = strings.TrimLeft(input, "docs.google.com/spreadsheets/d/")
		parts := strings.Split(input, "/")
		if len(parts) >= 1 && parts[0] != "" {
			return parts[0]
		}
	}

	return ""
}

// createExpenseData creates expense data and cell colors from purchase order items
func (s *purchaseOrderService) createExpenseData(paymentReceiptForm *models.PaymentReceiptForm) ([]map[string]interface{}, []string) {
	expensesData := make([]map[string]interface{}, 1)
	cellColors := make([]string, 1)

	expensesData[0] = map[string]interface{}{
		pkg.RevenueExpenseColumnName: paymentReceiptForm.PurchaseOrder.Items[0].Supplier.Name,
	}
	productType := paymentReceiptForm.PurchaseOrder.Items[0].Product.ProductType
	header, color := s.getHeaderAndColorFromProductType(productType)
	expensesData[0][header] = paymentReceiptForm.TotalAmount
	cellColors[0] = color
	ordinalNumber, err := strconv.Atoi(strings.Split(*paymentReceiptForm.FormNumber, "-")[2])
	if err != nil {
		s.logger.WithFields(logrus.Fields{
			"operation": "createExpenseData",
			"error":     err,
		}).Error("Failed to convert form number to ordinal number to int")
		return nil, nil
	}

	expensesData[0][pkg.RevenueExpenseColumnOrdinalNumber] = ordinalNumber

	// for i, item := range items {
	// 	// Add nil checks to prevent panics
	// 	if item == nil {
	// 		s.logger.WithFields(logrus.Fields{
	// 			"operation":  "createExpenseData",
	// 			"item_index": i,
	// 		}).Warn("Skipping nil purchase order item")
	// 		continue
	// 	}

	// 	productName := "Unknown Product"
	// 	productType := "Unknown"

	// 	if item.Product != nil {
	// 		productName = item.Product.Name
	// 		productType = item.Product.ProductType
	// 	}

	// 	expensesData[i] = map[string]interface{}{
	// 		pkg.RevenueExpenseColumnName: productName,
	// 	}

	// 	itemTotalPrice := item.CalculateTotalAmount()
	// 	header, color := s.getHeaderAndColorFromProductType(productType)
	// 	expensesData[i][header] = itemTotalPrice
	// 	cellColors[i] = color
	// }

	return expensesData, cellColors
}

// mapProductTypeToExpense maps product type to expense category and color
func (s *purchaseOrderService) getHeaderAndColorFromProductType(productType string) (header string, color string) {
	switch productType {
	case "Cơm":
		header = pkg.RevenueExpenseColumnSnackAndRice
		color = pkg.RevenueExpenseColumnSnackAndRiceColor
	case "Ăn nhẹ":
		header = pkg.RevenueExpenseColumnSnackAndRice
		color = pkg.RevenueExpenseColumnSnackAndRiceColor
	case "Nước":
		header = pkg.RevenueExpenseColumnWater
		color = pkg.RevenueExpenseColumnWaterColor
	}

	return
}

// UpdatePurchaseOrder updates a purchase order while preserving ReceivedQuantity
func (s *purchaseOrderService) UpdatePurchaseOrder(ctx context.Context, id uint, req dto.UpdatePurchaseOrderRequest) (*models.PurchaseOrder, error) {
	s.logger.WithFields(logrus.Fields{
		"operation":         "UpdatePurchaseOrder",
		"purchase_order_id": id,
	}).Info("Updating purchase order")

	po, err := s.purchaseOrderRepo.UpdatePurchaseOrder(ctx, id, req)
	if err != nil {
		s.logger.WithFields(logrus.Fields{
			"operation":         "UpdatePurchaseOrder",
			"purchase_order_id": id,
			"error":             err,
		}).Error("Failed to update purchase order")
		return nil, err
	}

	// Calculate total amount based on items
	po.TotalAmount = po.CalculateTotalAmount()

	s.logger.WithFields(logrus.Fields{
		"operation":         "UpdatePurchaseOrder",
		"purchase_order_id": id,
		"total_amount":      po.TotalAmount,
	}).Info("Successfully updated purchase order")

	return po, nil
}

func (s *purchaseOrderService) ReceiveInventory(
	ctx context.Context,
	req dto.UpdatePurchaseOrderDeliveryStatusRequest,
) (*models.PurchaseOrder, error) {
	po, err := s.purchaseOrderRepo.ReceiveInventory(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to persist delivery update: %w", err)
	}

	po.TotalAmount = po.CalculateTotalAmount()
	return po, nil
}
