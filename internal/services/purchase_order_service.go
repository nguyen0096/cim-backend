package services

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"import-export-backend/internal/config"
	"import-export-backend/internal/models"
	"import-export-backend/internal/repository"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

//go:generate mockery --name=PurchaseOrderService --structname=PurchaseOrderService --output=../mocks/services --outpkg=servicemocks
type PurchaseOrderService interface {
	CreatePurchaseOrder(ctx context.Context, purchaseOrder *models.PurchaseOrder) error
	GetPurchaseOrderByID(id uint) (*models.PurchaseOrder, error)
	UpdatePurchaseOrder(ctx context.Context, purchaseOrder *models.PurchaseOrder) error
	DeletePurchaseOrder(id uint) error
	ListPurchaseOrders(ctx context.Context, params models.ListParams) (*models.PaginationResult[models.PurchaseOrder], error)
	GetPurchaseOrdersByStatus(status string) ([]models.PurchaseOrder, error)
	UpdatePurchaseOrderStatus(ctx context.Context, id uint, status string) error
	ReceivePurchaseOrder(ctx context.Context, id uint) error
	UpdatePurchaseOrderItemStatus(ctx context.Context, purchaseOrderID, itemID uint, status models.PurchaseOrderItemStatus) (*models.UpdatePurchaseOrderItemStatusResponse, error)
}

type purchaseOrderService struct {
	purchaseOrderRepo repository.PurchaseOrderRepository
	inventoryService  InventoryService
	excelService      ExcelService
	settingsService   SettingsService
	db                *gorm.DB
	logger            *logrus.Logger
}

func NewPurchaseOrderService(
	purchaseOrderRepo repository.PurchaseOrderRepository,
	inventoryService InventoryService,
	excelService ExcelService,
	settingsService SettingsService,
	db *gorm.DB,
	logger *logrus.Logger,
) PurchaseOrderService {
	return &purchaseOrderService{
		purchaseOrderRepo: purchaseOrderRepo,
		inventoryService:  inventoryService,
		excelService:      excelService,
		settingsService:   settingsService,
		db:                db,
		logger:            logger,
	}
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

func (s *purchaseOrderService) UpdatePurchaseOrder(ctx context.Context, purchaseOrder *models.PurchaseOrder) error {
	s.logger.WithFields(logrus.Fields{
		"operation":         "UpdatePurchaseOrder",
		"purchase_order_id": purchaseOrder.ID,
		"order_number":      purchaseOrder.OrderNumber,
	}).Info("Updating purchase order")

	err := s.purchaseOrderRepo.Update(ctx, purchaseOrder)
	if err != nil {
		s.logger.WithFields(logrus.Fields{
			"operation":         "UpdatePurchaseOrder",
			"purchase_order_id": purchaseOrder.ID,
			"order_number":      purchaseOrder.OrderNumber,
			"error":             err,
		}).Error("Failed to update purchase order")
		return err
	}

	s.logger.WithFields(logrus.Fields{
		"operation":         "UpdatePurchaseOrder",
		"purchase_order_id": purchaseOrder.ID,
		"order_number":      purchaseOrder.OrderNumber,
	}).Info("Successfully updated purchase order")

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

// UpdatePurchaseOrderStatus updates the status of a purchase order
func (s *purchaseOrderService) UpdatePurchaseOrderStatus(ctx context.Context, id uint, status string) error {
	s.logger.WithFields(logrus.Fields{
		"operation":         "UpdatePurchaseOrderStatus",
		"purchase_order_id": id,
		"new_status":        status,
	}).Info("Updating purchase order status")

	purchaseOrder, err := s.purchaseOrderRepo.GetByID(id)
	if err != nil {
		s.logger.WithFields(logrus.Fields{
			"operation":         "UpdatePurchaseOrderStatus",
			"purchase_order_id": id,
			"error":             err,
		}).Error("Failed to get purchase order for status update")
		return fmt.Errorf("failed to get purchase order: %w", err)
	}

	// If setting status to delivered, check if all other items are also delivered
	if (status == string(models.PurchaseOrderStatusFullyDelivered) || status == string(models.PurchaseOrderStatusCompleted)) && s.purchaseOrderRepo.AnyDeliveringItem(ctx, id) {
		return fmt.Errorf("cannot set item status to %s: not all items in the purchase order are delivered", status)
	}

	if status == string(models.PurchaseOrderStatusCompleted) && !s.purchaseOrderRepo.AnyDeliveringItem(ctx, id) {
		// Start goroutine to handle revenue expense excel file operations
		go s.handleRevenueExpenseAsync(context.Background(), purchaseOrder)
	}

	purchaseOrder.Status = models.PurchaseOrderStatus(status)
	err = s.purchaseOrderRepo.Update(ctx, purchaseOrder)
	if err != nil {
		s.logger.WithFields(logrus.Fields{
			"operation":         "UpdatePurchaseOrderStatus",
			"purchase_order_id": id,
			"new_status":        status,
			"error":             err,
		}).Error("Failed to update purchase order status")
		return err
	}

	s.logger.WithFields(logrus.Fields{
		"operation":         "UpdatePurchaseOrderStatus",
		"purchase_order_id": id,
		"new_status":        status,
		"order_number":      purchaseOrder.OrderNumber,
	}).Info("Successfully updated purchase order status")

	return nil
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
func (s *purchaseOrderService) UpdatePurchaseOrderItemStatus(ctx context.Context, purchaseOrderID, itemID uint, status models.PurchaseOrderItemStatus) (*models.UpdatePurchaseOrderItemStatusResponse, error) {
	s.logger.WithFields(logrus.Fields{
		"operation":         "UpdatePurchaseOrderItemStatus",
		"purchase_order_id": purchaseOrderID,
		"item_id":           itemID,
		"new_status":        status,
	}).Info("Updating purchase order item status")

	// Validate status
	if status != models.PurchaseOrderItemStatusDelivering && status != models.PurchaseOrderItemStatusDelivered {
		s.logger.WithFields(logrus.Fields{
			"operation":         "UpdatePurchaseOrderItemStatus",
			"purchase_order_id": purchaseOrderID,
			"item_id":           itemID,
			"invalid_status":    status,
		}).Error("Invalid status provided for purchase order item")
		return nil, fmt.Errorf("invalid status: %s", status)
	}

	var result *models.UpdatePurchaseOrderItemStatusResponse

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
		result = &models.UpdatePurchaseOrderItemStatusResponse{
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

// handleRevenueExpenseAsync handles revenue expense excel file operations asynchronously
func (s *purchaseOrderService) handleRevenueExpenseAsync(ctx context.Context, purchaseOrder *models.PurchaseOrder) {
	s.logger.WithFields(logrus.Fields{
		"operation":         "handleRevenueExpenseAsync",
		"purchase_order_id": purchaseOrder.ID,
		"order_number":      purchaseOrder.OrderNumber,
	}).Info("Starting async revenue expense processing")

	// Get settings of revenue expense excel file
	revenueExpenseExcelSettings, err := s.settingsService.GetSetting(ctx, config.RevenueExpenseExcelSettingsKey)
	if err != nil {
		s.logger.WithFields(logrus.Fields{
			"operation":         "handleRevenueExpenseAsync",
			"purchase_order_id": purchaseOrder.ID,
			"error":             err,
		}).Error("Failed to get revenue expense excel settings")
		return
	}

	if revenueExpenseExcelSettings == nil {
		s.logger.WithFields(logrus.Fields{
			"operation":         "handleRevenueExpenseAsync",
			"purchase_order_id": purchaseOrder.ID,
		}).Error("Revenue expense excel settings not configured")
		return
	}

	// Parse the settings value to extract file path
	var settingsValue map[string]interface{}
	if err := json.Unmarshal([]byte(revenueExpenseExcelSettings.Value), &settingsValue); err != nil {
		s.logger.WithFields(logrus.Fields{
			"operation":         "handleRevenueExpenseAsync",
			"purchase_order_id": purchaseOrder.ID,
			"error":             err,
		}).Error("Failed to parse revenue expense excel settings")
		return
	}

	filePath, ok := settingsValue["filePath"].(string)
	if !ok || filePath == "" {
		s.logger.WithFields(logrus.Fields{
			"operation":         "handleRevenueExpenseAsync",
			"purchase_order_id": purchaseOrder.ID,
			"file_path":         filePath,
		}).Error("File path not found in revenue expense excel settings")
		return
	}

	sheetName, ok := settingsValue["sheetName"].(string)
	if !ok || sheetName == "" {
		s.logger.WithFields(logrus.Fields{
			"operation":         "handleRevenueExpenseAsync",
			"purchase_order_id": purchaseOrder.ID,
			"file_path":         filePath,
			"sheet_name":        sheetName,
		}).Error("Sheet name not found in revenue expense excel settings")
		return
	}

	if err := s.excelService.InitializeRevenueExpenseFile(ctx, filePath); err != nil {
		s.logger.WithFields(logrus.Fields{
			"operation":         "handleRevenueExpenseAsync",
			"purchase_order_id": purchaseOrder.ID,
			"file_path":         filePath,
			"sheet_name":        sheetName,
			"error":             err,
		}).Error("Failed to initialize revenue expense excel file")
		return
	}

	// Add expense to revenue expense excel file
	expensesData := make([]map[string]interface{}, len(purchaseOrder.Items))
	for i, item := range purchaseOrder.Items {
		expensesData[i] = map[string]interface{}{
			"DIỄN GIẢI": item.Product.Name,
		}

		switch item.Product.ProductType {
		case "Cơm", "Ăn nhẹ":
			expensesData[i]["ĂN NHẸ,CƠM"] = item.TotalPrice
		default:
			expensesData[i]["NƯỚC"] = item.TotalPrice
		}
	}

	if err := s.excelService.AddExpenses(ctx, sheetName, expensesData); err != nil {
		s.logger.WithFields(logrus.Fields{
			"operation":         "handleRevenueExpenseAsync",
			"purchase_order_id": purchaseOrder.ID,
			"file_path":         filePath,
			"sheet_name":        sheetName,
			"error":             err,
		}).Error("Failed to add expense to revenue expense excel file")
		return
	}

	s.logger.WithFields(logrus.Fields{
		"operation":         "handleRevenueExpenseAsync",
		"purchase_order_id": purchaseOrder.ID,
	}).Info("Successfully added expense to revenue expense excel file")
}
