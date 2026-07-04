package services

import (
	"cim-backend/internal/config"
	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
	"cim-backend/pkg/log"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	mrand "math/rand"
	"mime/multipart"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
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

	// v1 - AGENTS MUST CONFIRM BEFORE MODIFYING SECTION BELOW THIS LINE

	UpdatePurchaseOrder(ctx context.Context, id uint, req dto.UpdatePurchaseOrderRequest) (*models.PurchaseOrder, error)
	ReceiveInventory(ctx context.Context, req dto.UpdatePurchaseOrderDeliveryStatusRequest) (*models.PurchaseOrder, error)
	RetryQueueRevenueExpenseRequest(ctx context.Context, paymentReceiptFormIDs []uint) error
	UploadAndValidatePurchaseOrderFile(ctx context.Context, file multipart.File, filename string, fileStorageService FileStorageService) (*dto.UploadPurchaseOrderFileResponse, error)
	ProcessImportPurchaseOrder(ctx context.Context, fileUID string, extension string, sheetName string, fileStorageService FileStorageService) (*models.PurchaseOrder, error)
}

type purchaseOrderService struct {
	purchaseOrderRepo          repository.PurchaseOrderRepository
	paymentReceiptFormRepo     repository.PaymentReceiptFormRepository
	unitRepo                   repository.UnitRepository
	productRepo                repository.ProductRepository
	inventoryService           InventoryService
	excelService               ExcelService
	unitService                UnitService
	productService             ProductService
	settingsService            SettingsService
	db                         *gorm.DB
	revenueExpenseRequestQueue chan revenueExpenseRequest
	supplierRepo               repository.SupplierRepository
	inventoryRepo              repository.InventoryRepository
	sellingPriceRepo           repository.SellingPriceRepository
}

// revenueExpenseRequest represents a queued request to process revenue expense
type revenueExpenseRequest struct {
	paymentReceiptFormIDs []uint
}

func NewPurchaseOrderService(
	purchaseOrderRepo repository.PurchaseOrderRepository,
	paymentReceiptFormRepo repository.PaymentReceiptFormRepository,
	unitRepo repository.UnitRepository,
	productRepo repository.ProductRepository,
	inventoryService InventoryService,
	excelService ExcelService,
	settingsService SettingsService,
	db *gorm.DB,
	supplierRepo repository.SupplierRepository,
	inventoryRepo repository.InventoryRepository,
	unitService UnitService,
	productService ProductService,
	sellingPriceRepo repository.SellingPriceRepository,
) PurchaseOrderService {
	// Create a buffered channel to queue revenue expense requests
	// Buffer size of 100 allows queuing up to 100 requests without blocking
	requestQueue := make(chan revenueExpenseRequest, 100)

	service := &purchaseOrderService{
		db:                         db,
		purchaseOrderRepo:          purchaseOrderRepo,
		paymentReceiptFormRepo:     paymentReceiptFormRepo,
		unitRepo:                   unitRepo,
		productRepo:                productRepo,
		supplierRepo:               supplierRepo,
		inventoryRepo:              inventoryRepo,
		excelService:               excelService,
		settingsService:            settingsService,
		unitService:                unitService,
		inventoryService:           inventoryService,
		productService:             productService,
		sellingPriceRepo:           sellingPriceRepo,
		revenueExpenseRequestQueue: requestQueue,
	}

	// Start the worker goroutine to process revenue expense requests serially
	go service.revenueExpenseWorker()

	log.Info("Purchase order service initialized with revenue expense worker")

	return service
}

// getBaseUnitID returns the base unit ID for a given unit
// If the unit is a base unit (BaseUnitID is nil), returns the unit ID itself
// If the unit is a derived unit, recursively drills down to the ultimate base unit
func (s *purchaseOrderService) getBaseUnitID(ctx context.Context, unitID uint) (uint, error) {
	unit, err := s.unitRepo.GetByID(ctx, unitID)
	if err != nil {
		return 0, fmt.Errorf("failed to get unit %d: %w", unitID, err)
	}

	// If unit is a base unit (no BaseUnitID), return the unit ID itself
	if unit.BaseUnitID == nil {
		return unitID, nil
	}

	// If unit is a derived unit, recursively drill down to the ultimate base unit
	return s.getBaseUnitID(ctx, *unit.BaseUnitID)
}

// convertQuantityToBaseUnit converts a quantity from the given unit to its base unit
// Returns the converted quantity, converted unit price, and the base unit ID
// If the unit is a base unit (BaseUnitID is nil), returns the quantity and price as-is and the unit ID itself
// If the unit is a derived unit, recursively drills down to the ultimate base unit, applying conversion factors along the way
func (s *purchaseOrderService) convertQuantityToBaseUnit(
	ctx context.Context,
	quantity decimal.Decimal,
	unitPrice float64,
	unitID uint,
) (decimal.Decimal, float64, uint, error) {
	unit, err := s.unitRepo.GetByID(ctx, unitID)
	if err != nil {
		return decimal.Zero, 0, 0, fmt.Errorf("failed to get unit %d: %w", unitID, err)
	}

	// If unit is a base unit (no BaseUnitID), return quantity and price as-is and the unit ID itself
	if unit.BaseUnitID == nil {
		return quantity, unitPrice, unitID, nil
	}

	// Reject corrupt data that would divide by zero.
	if unit.ConversionFactor == 0 {
		return decimal.Zero, 0, 0, fmt.Errorf("unit %d has a zero conversion factor", unitID)
	}

	// Convert to the base unit entirely in decimal to avoid float rounding.
	// Base quantity/price persist at 2dp, so a non-terminating factor (e.g. 1/3)
	// leaves a bounded, non-compounding sub-cent residual.
	factor := decimal.NewFromFloat(unit.ConversionFactor)
	baseQuantity := quantity.Mul(factor)
	baseUnitPrice := decimal.NewFromFloat(unitPrice).Div(factor).InexactFloat64()
	return s.convertQuantityToBaseUnit(ctx, baseQuantity, baseUnitPrice, *unit.BaseUnitID)
}

// generatePurchaseOrderNumber generates a unique purchase order number
func (s *purchaseOrderService) generatePurchaseOrderNumber() (string, error) {
	now := time.Now()

	// Generate 4-character random alphanumeric suffix.
	// Defense-in-depth only: collision-safety is guaranteed by the
	// generate-and-retry-on-23505 loop in CreatePurchaseOrder, not by suffix width.
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	suffix := make([]byte, 4)
	if _, err := rand.Read(suffix); err != nil {
		return "", fmt.Errorf("failed to generate random suffix: %w", err)
	}

	for i := range suffix {
		suffix[i] = charset[suffix[i]%byte(len(charset))]
	}

	// Format: PO-YYMMDD-HHMMSS-XXXX
	return fmt.Sprintf("PO-%s-%s",
		now.Format("060102-150405"),
		string(suffix)), nil
}

// CreatePurchaseOrder creates a new purchase order with auto-generated order number
func (s *purchaseOrderService) CreatePurchaseOrder(ctx context.Context, purchaseOrder *models.PurchaseOrder) error {
	log.WithFields(logrus.Fields{
		"operation":    "CreatePurchaseOrder",
		"order_number": purchaseOrder.OrderNumber,
	}).Info("Creating new purchase order")

	// Validate purchase order struct
	if err := pkg.Validator.Struct(purchaseOrder); err != nil {
		log.WithFields(logrus.Fields{
			"operation": "CreatePurchaseOrder",
			"error":     err,
		}).Error("Purchase order validation failed")
		return pkg.ErrValidation("invalid purchase order", err)
	}

	// Whether the order number is auto-generated by us. When a caller supplies an
	// order number we never regenerate or retry it (preserves the existing
	// "without changing provided order number" contract) — a duplicate-key on a
	// caller-supplied number surfaces as an error, as today.
	autoGenerateOrderNumber := purchaseOrder.OrderNumber == ""

	// Set status to order_placed for new purchase orders
	purchaseOrder.Status = models.PurchaseOrderStatusOrderPlaced

	// Validate and convert quantities to base unit for each item
	for _, item := range purchaseOrder.Items {
		if item != nil && item.Quantity.LessThanOrEqual(decimal.Zero) {
			return pkg.ErrValidation("quantity must be greater than 0", nil)
		}
		if item != nil && item.UnitID != nil && item.ProductID != nil {
			// Get product to check its unit
			product, err := s.productRepo.GetByID(ctx, *item.ProductID)
			if err != nil {
				log.WithFields(logrus.Fields{
					"operation":  "CreatePurchaseOrder",
					"product_id": *item.ProductID,
					"error":      err,
				}).Error("Failed to get product")
				return fmt.Errorf("failed to get product %d: %w", *item.ProductID, err)
			}

			// Get base unit ID for product's unit
			productBaseUnitID, err := s.getBaseUnitID(ctx, product.UnitID)
			if err != nil {
				log.WithFields(logrus.Fields{
					"operation":  "CreatePurchaseOrder",
					"product_id": *item.ProductID,
					"error":      err,
				}).Error("Failed to get product base unit")
				return fmt.Errorf("failed to get product base unit: %w", err)
			}

			// Get base unit ID for item's unit
			itemBaseUnitID, err := s.getBaseUnitID(ctx, *item.UnitID)
			if err != nil {
				log.WithFields(logrus.Fields{
					"operation": "CreatePurchaseOrder",
					"unit_id":   *item.UnitID,
					"error":     err,
				}).Error("Failed to get item base unit")
				return fmt.Errorf("failed to get item base unit: %w", err)
			}

			// Validate that item's unit base unit matches product's unit base unit
			if itemBaseUnitID != productBaseUnitID {
				log.WithFields(logrus.Fields{
					"operation":         "CreatePurchaseOrder",
					"product_id":        *item.ProductID,
					"product_base_unit": productBaseUnitID,
					"item_unit_id":      *item.UnitID,
					"item_base_unit":    itemBaseUnitID,
				}).Error("Item unit base unit does not match product unit base unit")
				return pkg.NewAppError(pkg.ErrorCodeValidation,
					fmt.Sprintf("unit %d (base unit %d) is not compatible with product %d (base unit %d)",
						*item.UnitID, itemBaseUnitID, *item.ProductID, productBaseUnitID), nil)
			}

			// Convert quantities to base unit
			baseQuantity, baseUnitPrice, baseUnitID, err := s.convertQuantityToBaseUnit(ctx, item.Quantity, item.UnitPrice, *item.UnitID)
			if err != nil {
				log.WithFields(logrus.Fields{
					"operation": "CreatePurchaseOrder",
					"unit_id":   *item.UnitID,
					"error":     err,
				}).Error("Failed to convert quantity to base unit")
				return fmt.Errorf("failed to convert quantity to base unit for unit %d: %w", *item.UnitID, err)
			}
			item.Quantity = baseQuantity
			item.UnitPrice = baseUnitPrice
			item.UnitID = &baseUnitID
		}
	}

	// Create the purchase order, retrying on a unique-violation of the
	// auto-generated order_number. Under concurrency two POs created in the same
	// second can generate the same PO-YYMMDD-HHMMSS-XXXX value; on a 23505 we
	// regenerate the number and retry the INSERT. Only the Create is retried:
	// validation, unit conversion and the per-item lookups above ran once, and the
	// post-create steps below run once after a successful Create.
	const maxRetries = 5
	var err error
	for attempt := 0; attempt < maxRetries; attempt++ {
		// Regenerate the order number on every attempt when auto-generating, so a
		// collision retry uses a fresh value rather than re-inserting the same one.
		if autoGenerateOrderNumber {
			orderNumber, genErr := s.generatePurchaseOrderNumber()
			if genErr != nil {
				log.WithFields(logrus.Fields{
					"operation": "CreatePurchaseOrder",
					"error":     genErr,
				}).Error("Failed to generate purchase order number")
				return fmt.Errorf("failed to generate purchase order number: %w", genErr)
			}
			purchaseOrder.OrderNumber = orderNumber
			log.WithFields(logrus.Fields{
				"operation":    "CreatePurchaseOrder",
				"order_number": orderNumber,
				"attempt":      attempt + 1,
			}).Info("Generated purchase order number")
		}

		// Reset auto-increment PKs before each attempt so GORM always performs a
		// fresh INSERT. A failed Create may have assigned in-memory PKs to the
		// parent and its items; leaving them set could flip GORM to an upsert or
		// collide on idx_product_supplier_po on the next attempt.
		purchaseOrder.ID = 0
		for _, item := range purchaseOrder.Items {
			if item != nil {
				item.ID = 0
				item.PurchaseOrderID = nil
			}
		}

		err = s.purchaseOrderRepo.Create(ctx, purchaseOrder)
		if err == nil {
			break
		}

		// Only an auto-generated order_number is retried, and only when the repository
		// translated the failure into the typed pkg.ErrDuplicateOrderNumber domain
		// error (its order_number unique-constraint violation — the #84 race). The
		// service does not inspect SQLSTATE codes or constraint names; that DB detail
		// lives in the repository. Any other error from Create — including the items'
		// idx_product_supplier_po unique violation (duplicate product/supplier in the
		// request), which the repository does NOT translate — is surfaced now:
		// regenerating the number and retrying would just resubmit the same invalid
		// items. Caller-supplied numbers are likewise never retried.
		if autoGenerateOrderNumber && pkg.IsErrorCode(err, pkg.ErrorCodeDuplicateOrderNumber) {
			if attempt == maxRetries-1 {
				log.WithFields(logrus.Fields{
					"operation":    "CreatePurchaseOrder",
					"order_number": purchaseOrder.OrderNumber,
					"attempts":     maxRetries,
					"error":        err,
				}).Error("Failed to create purchase order after retries due to duplicate key conflicts")
				return fmt.Errorf("failed to create purchase order after %d retries due to duplicate key conflicts: %w", maxRetries, err)
			}
			// Exponential backoff with jitter to reduce thundering herd.
			baseDelay := time.Millisecond * time.Duration(50*(1<<uint(attempt))) // 50ms, 100ms, 200ms, 400ms...
			jitter := time.Millisecond * time.Duration(mrand.Intn(50))           // random jitter up to 50ms
			log.WithFields(logrus.Fields{
				"operation":    "CreatePurchaseOrder",
				"order_number": purchaseOrder.OrderNumber,
				"attempt":      attempt + 1,
				"next_attempt": attempt + 2,
				"delay_ms":     (baseDelay + jitter).Milliseconds(),
			}).Warn("Duplicate order number, regenerating and retrying purchase order create")
			time.Sleep(baseDelay + jitter)
			continue
		}

		log.WithFields(logrus.Fields{
			"operation":    "CreatePurchaseOrder",
			"order_number": purchaseOrder.OrderNumber,
			"error":        err,
		}).Error("Failed to create purchase order")
		return err
	}

	// Auto-create POItemSellingPrice records from the selling price ledger
	if err := s.createPOItemSellingPrices(ctx, purchaseOrder); err != nil {
		log.WithFields(logrus.Fields{
			"operation":         "CreatePurchaseOrder",
			"purchase_order_id": purchaseOrder.ID,
			"error":             err.Error(),
		}).Error("Failed to create PO item selling prices")
	} else {
		log.WithFields(logrus.Fields{
			"operation":         "CreatePurchaseOrder",
			"purchase_order_id": purchaseOrder.ID,
			"items_count":       len(purchaseOrder.Items),
		}).Info("Successfully created PO item selling prices")
	}

	// Reload purchase order with relationships
	reloadedPO, err := s.purchaseOrderRepo.GetByID(purchaseOrder.ID)
	if err != nil {
		log.WithFields(logrus.Fields{
			"operation":    "CreatePurchaseOrder",
			"order_number": purchaseOrder.OrderNumber,
			"error":        err,
		}).Error("Failed to reload purchase order")
		return fmt.Errorf("failed to reload purchase order: %w", err)
	}
	*purchaseOrder = *reloadedPO
	purchaseOrder.TotalAmount = purchaseOrder.CalculateTotalAmount()

	log.WithFields(logrus.Fields{
		"operation":         "CreatePurchaseOrder",
		"order_number":      purchaseOrder.OrderNumber,
		"purchase_order_id": purchaseOrder.ID,
	}).Info("Successfully created purchase order")

	return nil
}

// createPOItemSellingPrices ensures every PO item has a POItemSellingPrice row.
// Idempotent: items that already have a row are skipped, so this is safe to call
// from both CreatePurchaseOrder (all items new) and UpdatePurchaseOrder (some items
// pre-existing). selling_price_id defaults from the selling price ledger
// (inventory-specific → global fallback); selling_price (override) is left NULL.
func (s *purchaseOrderService) createPOItemSellingPrices(ctx context.Context, po *models.PurchaseOrder) error {
	if s.sellingPriceRepo == nil {
		return fmt.Errorf("sellingPriceRepo is nil")
	}
	if len(po.Items) == 0 {
		return fmt.Errorf("PO has no items")
	}

	// Collect candidate item + product IDs
	itemIDs := make([]uint, 0, len(po.Items))
	productIDs := make([]uint, 0, len(po.Items))
	for _, item := range po.Items {
		if item == nil || item.ProductID == nil {
			continue
		}
		itemIDs = append(itemIDs, item.ID)
		productIDs = append(productIDs, *item.ProductID)
	}
	if len(productIDs) == 0 {
		return fmt.Errorf("no product IDs found in PO items (items: %d)", len(po.Items))
	}

	// Skip items that already have a pisp row (idempotency)
	existing, err := s.sellingPriceRepo.GetPOItemSellingPricesByPOItemIDs(ctx, itemIDs)
	if err != nil {
		return fmt.Errorf("failed to check existing PO item selling prices: %w", err)
	}
	existingByItemID := make(map[uint]bool, len(existing))
	for _, e := range existing {
		existingByItemID[e.PurchaseOrderItemID] = true
	}

	log.WithFields(logrus.Fields{
		"operation":      "createPOItemSellingPrices",
		"po_id":          po.ID,
		"product_ids":    productIDs,
		"existing_count": len(existing),
	}).Info("Looking up selling prices for products")

	// Look up latest selling prices for the products that still need a pisp row
	sellingPriceMap, err := s.sellingPriceRepo.GetLatestForProducts(ctx, productIDs, po.InventoryID, time.Now())
	if err != nil {
		return fmt.Errorf("failed to get latest selling prices: %w", err)
	}

	log.WithFields(logrus.Fields{
		"operation":    "createPOItemSellingPrices",
		"prices_found": len(sellingPriceMap),
	}).Info("Selling prices lookup result")

	for _, item := range po.Items {
		if item == nil || item.ProductID == nil {
			continue
		}
		if existingByItemID[item.ID] {
			continue
		}
		poItemSP := &models.POItemSellingPrice{
			PurchaseOrderItemID: item.ID,
		}
		if sp, ok := sellingPriceMap[*item.ProductID]; ok {
			poItemSP.SellingPriceID = &sp.ID
		}
		if err := s.sellingPriceRepo.CreatePOItemSellingPrice(ctx, poItemSP); err != nil {
			return fmt.Errorf("failed to create PO item selling price for item %d: %w", item.ID, err)
		}
	}

	return nil
}

func (s *purchaseOrderService) GetPurchaseOrderByID(id uint) (*models.PurchaseOrder, error) {
	log.WithFields(logrus.Fields{
		"operation":         "GetPurchaseOrderByID",
		"purchase_order_id": id,
	}).Info("Retrieving purchase order by ID")

	purchaseOrder, err := s.purchaseOrderRepo.GetByID(id)
	if err != nil {
		log.WithFields(logrus.Fields{
			"operation":         "GetPurchaseOrderByID",
			"purchase_order_id": id,
			"error":             err,
		}).Error("Failed to retrieve purchase order")
		return nil, err
	}

	// Enrich each item's unit with conversion_factor_to_current for derived units
	ctx := context.Background()
	for _, item := range purchaseOrder.Items {
		if item.Unit != nil && item.UnitID != nil {
			// Get the full unit information with conversion_factor_to_current calculated
			enrichedUnit, err := s.unitRepo.GetByID(ctx, *item.UnitID)
			if err == nil && enrichedUnit != nil {
				// Remove base_unit from derived units (no need to nest base_unit inside derived units)
				for _, derivedUnit := range enrichedUnit.DerivedUnits {
					if derivedUnit != nil {
						derivedUnit.BaseUnit = nil
					}
				}
				// Replace the unit with the enriched version that has conversion_factor_to_current
				item.Unit = enrichedUnit
			}
		}
	}

	// Calculate total amount based on items
	purchaseOrder.TotalAmount = purchaseOrder.CalculateTotalAmount()

	log.WithFields(logrus.Fields{
		"operation":         "GetPurchaseOrderByID",
		"purchase_order_id": id,
		"order_number":      purchaseOrder.OrderNumber,
		"total_amount":      purchaseOrder.TotalAmount,
	}).Info("Successfully retrieved purchase order")

	return purchaseOrder, nil
}

func (s *purchaseOrderService) UpdatePurchaseOrderStatus(ctx context.Context, id uint, status models.PurchaseOrderStatus) error {
	log.WithFields(logrus.Fields{
		"operation":         "UpdatePurchaseOrderStatus",
		"purchase_order_id": id,
		"status":            status,
	}).Info("Updating purchase order status")

	if status == models.PurchaseOrderStatusCompleted {
		// Get all approved payment receipt forms for this purchase order
		forms, err := s.paymentReceiptFormRepo.GetLatestPaymentReceiptForms(ctx, id, models.PaymentReceiptFormStatusApproved, 0)
		if err != nil {
			log.WithFields(logrus.Fields{
				"operation":         "UpdatePurchaseOrderStatus",
				"purchase_order_id": id,
				"error":             err,
			}).Error("Failed to get approved payment receipt forms")
			return pkg.ErrFailedToGetApprovedPaymentReceiptForms(ctx, err)
		}

		if len(forms) == 0 {
			log.WithFields(logrus.Fields{
				"operation":         "UpdatePurchaseOrderStatus",
				"purchase_order_id": id,
			}).Warn("Cannot complete purchase order: no approved payment receipt form found")
			return pkg.ErrNoApprovedPaymentReceiptForm(ctx)
		}
	}

	err := s.purchaseOrderRepo.UpdateStatus(ctx, id, status)
	if err != nil {
		log.WithFields(logrus.Fields{
			"operation":         "UpdatePurchaseOrderStatus",
			"purchase_order_id": id,
			"status":            status,
			"error":             err,
		}).Error("Failed to update purchase order status")
		return pkg.ErrFailedToUpdatePurchaseOrderStatus(ctx, err)
	}

	log.WithFields(logrus.Fields{
		"operation":         "UpdatePurchaseOrderStatus",
		"purchase_order_id": id,
		"status":            status,
	}).Info("Successfully updated purchase order status")

	return nil
}

func (s *purchaseOrderService) DeletePurchaseOrder(id uint) error {
	log.WithFields(logrus.Fields{
		"operation":         "DeletePurchaseOrder",
		"purchase_order_id": id,
	}).Info("Deleting purchase order")

	err := s.purchaseOrderRepo.Delete(id)
	if err != nil {
		log.WithFields(logrus.Fields{
			"operation":         "DeletePurchaseOrder",
			"purchase_order_id": id,
			"error":             err,
		}).Error("Failed to delete purchase order")
		return err
	}

	log.WithFields(logrus.Fields{
		"operation":         "DeletePurchaseOrder",
		"purchase_order_id": id,
	}).Info("Successfully deleted purchase order")

	return nil
}

// ListPurchaseOrders retrieves purchase orders with search and pagination
func (s *purchaseOrderService) ListPurchaseOrders(ctx context.Context, params models.ListParams) (*models.PaginationResult[models.PurchaseOrder], error) {
	log.WithFields(logrus.Fields{
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
		log.WithFields(logrus.Fields{
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

	log.WithFields(logrus.Fields{
		"operation":      "ListPurchaseOrders",
		"total_count":    total,
		"returned_count": len(purchaseOrders),
		"page":           params.Page,
		"limit":          params.Limit,
	}).Info("Successfully listed purchase orders")

	return result, nil
}

func (s *purchaseOrderService) GetPurchaseOrdersByStatus(status string) ([]models.PurchaseOrder, error) {
	log.WithFields(logrus.Fields{
		"operation": "GetPurchaseOrdersByStatus",
		"status":    status,
	}).Info("Retrieving purchase orders by status")

	purchaseOrders, err := s.purchaseOrderRepo.GetByStatus(status)
	if err != nil {
		log.WithFields(logrus.Fields{
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

	log.WithFields(logrus.Fields{
		"operation": "GetPurchaseOrdersByStatus",
		"status":    status,
		"count":     len(purchaseOrders),
	}).Info("Successfully retrieved purchase orders by status")

	return purchaseOrders, nil
}

func (s *purchaseOrderService) ReceivePurchaseOrder(ctx context.Context, id uint) error {
	log.WithFields(logrus.Fields{
		"operation":         "ReceivePurchaseOrder",
		"purchase_order_id": id,
	}).Info("Receiving purchase order")

	purchaseOrder, err := s.purchaseOrderRepo.GetByID(id)
	if err != nil {
		log.WithFields(logrus.Fields{
			"operation":         "ReceivePurchaseOrder",
			"purchase_order_id": id,
			"error":             err,
		}).Error("Failed to get purchase order for receiving")
		return err
	}

	// Update status to received
	purchaseOrder.Status = models.PurchaseOrderStatusCompleted
	if err := s.purchaseOrderRepo.Update(ctx, purchaseOrder); err != nil {
		log.WithFields(logrus.Fields{
			"operation":         "ReceivePurchaseOrder",
			"purchase_order_id": id,
			"error":             err,
		}).Error("Failed to update purchase order status to completed")
		return fmt.Errorf("failed to update purchase order: %w", err)
	}

	log.WithFields(logrus.Fields{
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
			log.WithFields(logrus.Fields{
				"operation":         "ReceivePurchaseOrder",
				"purchase_order_id": id,
				"product_id":        *item.ProductID,
				"quantity":          item.Quantity,
				"error":             err,
			}).Error("Failed to add inventory for received item")
			return err
		}
	}

	log.WithFields(logrus.Fields{
		"operation":         "ReceivePurchaseOrder",
		"purchase_order_id": id,
		"order_number":      purchaseOrder.OrderNumber,
		"items_count":       len(purchaseOrder.Items),
	}).Info("Successfully received purchase order and added inventory")

	return nil
}

// UpdatePurchaseOrderItemStatus updates the status of a purchase order item
func (s *purchaseOrderService) UpdatePurchaseOrderItemStatus(ctx context.Context, purchaseOrderID, itemID uint, status models.PurchaseOrderItemStatus) (*dto.UpdatePurchaseOrderItemStatusResponse, error) {
	log.WithFields(logrus.Fields{
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
			log.WithFields(logrus.Fields{
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
			log.WithFields(logrus.Fields{
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
		log.WithFields(logrus.Fields{
			"operation":         "UpdatePurchaseOrderItemStatus",
			"purchase_order_id": purchaseOrderID,
			"item_id":           itemID,
			"error":             err,
		}).Error("Transaction failed for purchase order item status update")
		return nil, err
	}

	log.WithFields(logrus.Fields{
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
	log.Info("Revenue expense worker started")

	for req := range s.revenueExpenseRequestQueue {
		log.WithFields(logrus.Fields{
			"operation":                "revenueExpenseWorker",
			"payment_receipt_form_ids": req.paymentReceiptFormIDs,
			"forms_count":              len(req.paymentReceiptFormIDs),
			"queue_length":             len(s.revenueExpenseRequestQueue),
		}).Info("Processing revenue expense request from queue")

		// Process the request with retry mechanism
		s.handleRevenueExpenseAsyncBySettingsWithRetry(req.paymentReceiptFormIDs)
	}

	log.Warn("Revenue expense worker stopped - channel closed")
}

// queueRevenueExpenseRequest queues a revenue expense request for serial processing
func (s *purchaseOrderService) queueRevenueExpenseRequest(paymentReceiptFormIDs []uint) {
	if len(paymentReceiptFormIDs) == 0 {
		log.WithFields(logrus.Fields{
			"operation": "queueRevenueExpenseRequest",
		}).Warn("No payment receipt form IDs provided, skipping queue")
		return
	}

	req := revenueExpenseRequest{
		paymentReceiptFormIDs: paymentReceiptFormIDs,
	}

	s.revenueExpenseRequestQueue <- req
}

func (s *purchaseOrderService) GetLatestPaymentReceiptForms(ctx context.Context, purchaseOrderID uint) ([]*models.PaymentReceiptForm, error) {
	return s.paymentReceiptFormRepo.GetLatestPaymentReceiptForms(ctx, purchaseOrderID, models.PaymentReceiptFormStatusApproved, 0)
}

// RetryQueueRevenueExpenseRequest retries queueing a revenue expense request for payment receipt forms
func (s *purchaseOrderService) RetryQueueRevenueExpenseRequest(ctx context.Context, paymentReceiptFormIDs []uint) error {
	s.queueRevenueExpenseRequest(paymentReceiptFormIDs)

	log.WithFields(logrus.Fields{
		"operation":                "RetryQueueRevenueExpenseRequest",
		"payment_receipt_form_ids": paymentReceiptFormIDs,
		"forms_count":              len(paymentReceiptFormIDs),
	}).Info("Successfully queued revenue expense request")

	return nil
}

// handleRevenueExpenseAsyncBySettingsWithRetry wraps the async handler with retry mechanism
func (s *purchaseOrderService) handleRevenueExpenseAsyncBySettingsWithRetry(paymentReceiptFormIDs []uint) {
	if len(paymentReceiptFormIDs) == 0 {
		log.WithFields(logrus.Fields{
			"operation": "handleRevenueExpenseAsyncBySettingsWithRetry",
		}).Warn("No payment receipt form IDs provided")
		return
	}

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
					log.WithFields(logrus.Fields{
						"operation":                "handleRevenueExpenseAsyncBySettingsWithRetry",
						"payment_receipt_form_ids": paymentReceiptFormIDs,
						"attempt":                  attempt,
						"max_retries":              maxRetries,
						"panic":                    r,
						"stack_trace":              string(debug.Stack()),
					}).Error("Panic occurred in revenue expense handler")
				}
			}()

			log.WithFields(logrus.Fields{
				"operation":                "handleRevenueExpenseAsyncBySettingsWithRetry",
				"payment_receipt_form_ids": paymentReceiptFormIDs,
				"forms_count":              len(paymentReceiptFormIDs),
				"attempt":                  attempt,
				"max_retries":              maxRetries,
			}).Info("Attempting to process revenue expense")

			s.handleRevenueExpenseAsyncBySettings(context.Background(), paymentReceiptFormIDs)
			lastErr = nil // Success - clear any previous error
		}()

		// If successful, exit
		if lastErr == nil {
			log.WithFields(logrus.Fields{
				"operation":                "handleRevenueExpenseAsyncBySettingsWithRetry",
				"payment_receipt_form_ids": paymentReceiptFormIDs,
				"forms_count":              len(paymentReceiptFormIDs),
				"attempt":                  attempt,
			}).Info("Successfully processed revenue expense")
			return
		}

		// If this was the last attempt, log final failure
		if attempt == maxRetries {
			log.WithFields(logrus.Fields{
				"operation":                "handleRevenueExpenseAsyncBySettingsWithRetry",
				"payment_receipt_form_ids": paymentReceiptFormIDs,
				"forms_count":              len(paymentReceiptFormIDs),
				"attempts":                 attempt,
				"error":                    lastErr,
			}).Error("Failed to process revenue expense after all retry attempts")
			return
		}

		// Log retry attempt with delay
		log.WithFields(logrus.Fields{
			"operation":                "handleRevenueExpenseAsyncBySettingsWithRetry",
			"payment_receipt_form_ids": paymentReceiptFormIDs,
			"forms_count":              len(paymentReceiptFormIDs),
			"attempt":                  attempt,
			"next_attempt":             attempt + 1,
			"delay_seconds":            delay.Seconds(),
			"error":                    lastErr,
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
func (s *purchaseOrderService) handleRevenueExpenseAsyncBySettings(ctx context.Context, paymentReceiptFormIDs []uint) {
	if len(paymentReceiptFormIDs) == 0 {
		log.WithFields(logrus.Fields{
			"operation": "handleRevenueExpenseAsyncBySettings",
		}).Warn("No payment receipt form IDs provided")
		return
	}

	// Get settings to determine file type
	settings, err := s.settingsService.GetSetting(ctx, config.RevenueExpenseExcelSettingsKey)
	if err != nil {
		log.WithFields(logrus.Fields{
			"operation":                "handleRevenueExpenseAsyncBySettings",
			"payment_receipt_form_ids": paymentReceiptFormIDs,
			"error":                    err,
		}).Error("Failed to get revenue expense settings")
		return
	}

	if settings == nil {
		log.WithFields(logrus.Fields{
			"operation":                "handleRevenueExpenseAsyncBySettings",
			"payment_receipt_form_ids": paymentReceiptFormIDs,
		}).Error("Revenue expense settings not configured")
		return
	}

	var settingsValue map[string]interface{}
	if err := json.Unmarshal([]byte(settings.Value), &settingsValue); err != nil {
		log.WithFields(logrus.Fields{
			"operation":                "handleRevenueExpenseAsyncBySettings",
			"payment_receipt_form_ids": paymentReceiptFormIDs,
			"error":                    err,
		}).Error("Failed to parse revenue expense settings")
		return
	}

	filePath, ok := settingsValue["filePath"].(string)
	if !ok || filePath == "" {
		log.WithFields(logrus.Fields{
			"operation":                "handleRevenueExpenseAsyncBySettings",
			"payment_receipt_form_ids": paymentReceiptFormIDs,
		}).Error("filePath not found in revenue expense settings")
		return
	}

	// Detect if filePath is a Google Sheets URL or local file path
	isGoogleSheets := strings.Contains(filePath, "docs.google.com/spreadsheets")

	if isGoogleSheets {
		log.WithFields(logrus.Fields{
			"operation":                "handleRevenueExpenseAsyncBySettings",
			"payment_receipt_form_ids": paymentReceiptFormIDs,
			"file_type":                "google_sheets",
		}).Info("Detected Google Sheets, calling handleRevenueExpenseGoogleSheetsAsync")
		s.handleRevenueExpenseGoogleSheetsAsync(ctx, paymentReceiptFormIDs, settingsValue)
	} else {
		log.WithFields(logrus.Fields{
			"operation":                "handleRevenueExpenseAsyncBySettings",
			"payment_receipt_form_ids": paymentReceiptFormIDs,
			"file_type":                "local_file",
		}).Info("Detected local file, calling handleRevenueExpenseAsync")
		s.handleRevenueExpenseAsync(ctx, paymentReceiptFormIDs, settingsValue)
	}
}

// handleRevenueExpenseAsync handles revenue expense excel file operations asynchronously
func (s *purchaseOrderService) handleRevenueExpenseAsync(ctx context.Context, paymentReceiptFormIDs []uint, settingsValue map[string]interface{}) {
	startTime := time.Now()

	if len(paymentReceiptFormIDs) == 0 {
		log.WithFields(logrus.Fields{
			"operation": "handleRevenueExpenseAsync",
		}).Warn("No payment receipt form IDs provided")
		return
	}

	// Fetch all payment receipt forms
	paymentReceiptForms, err := s.paymentReceiptFormRepo.GetByIDsFull(ctx, paymentReceiptFormIDs)
	if err != nil {
		log.WithFields(logrus.Fields{
			"operation": "handleRevenueExpenseAsync",
			"error":     err,
		}).Error("Failed to get payment receipt forms")
		return
	}

	if len(paymentReceiptForms) == 0 {
		log.WithFields(logrus.Fields{
			"operation":                "handleRevenueExpenseAsync",
			"payment_receipt_form_ids": paymentReceiptFormIDs,
		}).Error("Failed to fetch any payment receipt forms")
		return
	}

	logger := log.WithFields(logrus.Fields{
		"operation":                "handleRevenueExpenseAsync",
		"payment_receipt_form_ids": paymentReceiptFormIDs,
		"forms_count":              len(paymentReceiptForms),
	})
	logger.Info("Starting async revenue expense processing")

	// Get and validate settings
	filePath, ok := settingsValue["filePath"].(string)
	if !ok || filePath == "" {
		log.WithFields(logrus.Fields{
			"operation":                "handleRevenueExpenseAsync",
			"payment_receipt_form_ids": paymentReceiptFormIDs,
		}).Error("filePath not found in revenue expense settings")
		return
	}
	sheetName, ok := settingsValue["sheetName"].(string)
	if !ok || sheetName == "" {
		log.WithFields(logrus.Fields{
			"operation":                "handleRevenueExpenseAsync",
			"payment_receipt_form_ids": paymentReceiptFormIDs,
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
	logger.WithFields(logrus.Fields{
		"file_path":                filePath,
		"sheet_name":               sheetName,
		"payment_receipt_form_ids": paymentReceiptFormIDs,
		"forms_count":              len(paymentReceiptForms),
	}).Info("Creating expense data from payment receipt forms")

	expensesData, cellColors := s.createExpenseData(paymentReceiptForms)
	if len(expensesData) == 0 {
		logger.Warn("No expense data created from payment receipt forms")
		return
	}

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
		"duration_ms":    duration.Milliseconds(),
		"expenses_count": len(expensesData),
	}).Info("Successfully added expense to revenue expense excel file")
}

// handleRevenueExpenseGoogleSheetsAsync handles revenue expense Google Sheets operations asynchronously
func (s *purchaseOrderService) handleRevenueExpenseGoogleSheetsAsync(ctx context.Context, paymentReceiptFormIDs []uint, settingsValue map[string]interface{}) {
	startTime := time.Now()

	if len(paymentReceiptFormIDs) == 0 {
		log.WithFields(logrus.Fields{
			"operation": "handleRevenueExpenseGoogleSheetsAsync",
		}).Warn("No payment receipt form IDs provided")
		return
	}

	// Fetch all payment receipt forms
	paymentReceiptForms, err := s.paymentReceiptFormRepo.GetByIDsFull(ctx, paymentReceiptFormIDs)
	if err != nil {
		log.WithFields(logrus.Fields{
			"operation": "handleRevenueExpenseGoogleSheetsAsync",
			"error":     err,
		}).Error("Failed to get payment receipt forms")
		return
	}

	if len(paymentReceiptForms) == 0 {
		log.WithFields(logrus.Fields{
			"operation":                "handleRevenueExpenseGoogleSheetsAsync",
			"payment_receipt_form_ids": paymentReceiptFormIDs,
		}).Error("Failed to fetch any payment receipt forms")
		return
	}

	logger := log.WithFields(logrus.Fields{
		"operation":                "handleRevenueExpenseGoogleSheetsAsync",
		"payment_receipt_form_ids": paymentReceiptFormIDs,
		"forms_count":              len(paymentReceiptForms),
	})
	logger.Info("Starting async Google Sheets revenue expense processing")

	filePath, ok := settingsValue["filePath"].(string)
	if !ok || filePath == "" {
		log.WithFields(logrus.Fields{
			"operation":                "handleRevenueExpenseGoogleSheetsAsync",
			"payment_receipt_form_ids": paymentReceiptFormIDs,
		}).Error("filePath not found in revenue expense settings")
		return
	}

	spreadsheetID := extractSpreadsheetID(filePath)
	if spreadsheetID == "" {
		log.WithFields(logrus.Fields{
			"operation":                "handleRevenueExpenseGoogleSheetsAsync",
			"payment_receipt_form_ids": paymentReceiptFormIDs,
		}).Error("spreadsheetID not found in revenue expense settings")
		return
	}

	sheetName, ok := settingsValue["sheetName"].(string)
	if !ok || sheetName == "" {
		log.WithFields(logrus.Fields{
			"operation":                "handleRevenueExpenseGoogleSheetsAsync",
			"payment_receipt_form_ids": paymentReceiptFormIDs,
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
		"spreadsheet_id":           spreadsheetID,
		"sheet_name":               sheetName,
		"payment_receipt_form_ids": paymentReceiptFormIDs,
		"forms_count":              len(paymentReceiptForms),
	}).Info("Creating expense data from payment receipt forms")

	expensesData, cellColors := s.createExpenseData(paymentReceiptForms)
	if len(expensesData) == 0 {
		logger.Warn("No expense data created from payment receipt forms")
		return
	}

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
		"expenses_count": len(expensesData),
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
		input = strings.TrimPrefix(input, "https://")
		input = strings.TrimPrefix(input, "docs.google.com/spreadsheets/d/")
		parts := strings.Split(input, "/")
		if len(parts) >= 1 && parts[0] != "" {
			return parts[0]
		}
	}

	return ""
}

// createExpenseData creates expense data and cell colors from payment receipt forms
func (s *purchaseOrderService) createExpenseData(paymentReceiptForms []*models.PaymentReceiptForm) ([]map[string]interface{}, []string) {
	if len(paymentReceiptForms) == 0 {
		log.WithFields(logrus.Fields{
			"operation": "createExpenseData",
		}).Warn("No payment receipt forms provided")
		return nil, nil
	}

	expensesData := make([]map[string]interface{}, 0, len(paymentReceiptForms))
	cellColors := make([]string, 0, len(paymentReceiptForms))

	for _, paymentReceiptForm := range paymentReceiptForms {
		// Add nil checks to prevent panics
		if paymentReceiptForm == nil {
			log.WithFields(logrus.Fields{
				"operation": "createExpenseData",
			}).Warn("Skipping nil payment receipt form")
			continue
		}

		// Validate required fields
		if paymentReceiptForm.PurchaseOrder == nil {
			log.WithFields(logrus.Fields{
				"operation":               "createExpenseData",
				"payment_receipt_form_id": paymentReceiptForm.ID,
			}).Warn("Skipping payment receipt form: purchase order is nil")
			continue
		}

		if len(paymentReceiptForm.PurchaseOrder.Items) == 0 {
			log.WithFields(logrus.Fields{
				"operation":               "createExpenseData",
				"payment_receipt_form_id": paymentReceiptForm.ID,
			}).Warn("Skipping payment receipt form: no purchase order items")
			continue
		}

		if paymentReceiptForm.PurchaseOrder.Items[0].Product == nil {
			log.WithFields(logrus.Fields{
				"operation":               "createExpenseData",
				"payment_receipt_form_id": paymentReceiptForm.ID,
			}).Warn("Skipping payment receipt form: product is nil")
			continue
		}

		supplierName := ""
		if paymentReceiptForm.PurchaseOrder.Items[0].Supplier != nil {
			supplierName = paymentReceiptForm.PurchaseOrder.Items[0].Supplier.Name
		}

		expenseData := map[string]interface{}{
			pkg.RevenueExpenseColumnName: supplierName,
		}

		productType := paymentReceiptForm.PurchaseOrder.Items[0].Product.ProductType
		header, color := s.excelService.GetHeaderAndColorFromProductType(productType)
		expenseData[header] = paymentReceiptForm.TotalAmount

		ordinalNumber, err := strconv.Atoi(strings.Split(*paymentReceiptForm.FormNumber, "-")[2])
		if err != nil {
			log.WithFields(logrus.Fields{
				"operation":               "createExpenseData",
				"payment_receipt_form_id": paymentReceiptForm.ID,
				"form_number":             paymentReceiptForm.FormNumber,
				"error":                   err,
			}).Error("Failed to convert form number to ordinal number to int")
			continue
		}

		expenseData[pkg.RevenueExpenseColumnOrdinalNumber] = ordinalNumber

		expensesData = append(expensesData, expenseData)
		cellColors = append(cellColors, color)
	}

	return expensesData, cellColors
}

// UpdatePurchaseOrder updates a purchase order while preserving ReceivedQuantity
func (s *purchaseOrderService) UpdatePurchaseOrder(ctx context.Context, id uint, req dto.UpdatePurchaseOrderRequest) (*models.PurchaseOrder, error) {
	log.WithFields(logrus.Fields{
		"operation":         "UpdatePurchaseOrder",
		"purchase_order_id": id,
	}).Info("Updating purchase order")

	// Validate and convert quantities to base unit for each item
	for i := range req.Items {
		if req.Items[i].UnitID != nil && req.Items[i].ProductID != nil {
			// Get product to check its unit
			product, err := s.productRepo.GetByID(ctx, *req.Items[i].ProductID)
			if err != nil {
				log.WithFields(logrus.Fields{
					"operation":  "UpdatePurchaseOrder",
					"product_id": *req.Items[i].ProductID,
					"error":      err,
				}).Error("Failed to get product")
				// Check if error is already an AppError, return it directly
				var appErr *pkg.AppError
				if errors.As(err, &appErr) {
					return nil, err
				}
				return nil, fmt.Errorf("failed to get product %d: %w", *req.Items[i].ProductID, err)
			}

			// Get base unit ID for product's unit
			productBaseUnitID, err := s.getBaseUnitID(ctx, product.UnitID)
			if err != nil {
				log.WithFields(logrus.Fields{
					"operation":  "UpdatePurchaseOrder",
					"product_id": *req.Items[i].ProductID,
					"error":      err,
				}).Error("Failed to get product base unit")
				return nil, pkg.ErrFailedToGetBaseUnit(ctx, err)
			}

			// Get base unit ID for item's unit
			itemBaseUnitID, err := s.getBaseUnitID(ctx, *req.Items[i].UnitID)
			if err != nil {
				log.WithFields(logrus.Fields{
					"operation": "UpdatePurchaseOrder",
					"unit_id":   *req.Items[i].UnitID,
					"error":     err,
				}).Error("Failed to get item base unit")
				return nil, pkg.ErrFailedToGetBaseUnit(ctx, err)
			}

			// Validate that item's unit base unit matches product's unit base unit
			if itemBaseUnitID != productBaseUnitID {
				log.WithFields(logrus.Fields{
					"operation":         "UpdatePurchaseOrder",
					"product_id":        *req.Items[i].ProductID,
					"product_base_unit": productBaseUnitID,
					"item_unit_id":      *req.Items[i].UnitID,
					"item_base_unit":    itemBaseUnitID,
				}).Error("Item unit base unit does not match product unit base unit")
				return nil, pkg.ErrUnitIncompatibleWithProduct(ctx, *req.Items[i].UnitID, itemBaseUnitID, *req.Items[i].ProductID, productBaseUnitID)
			}

			// Convert quantities to base unit
			baseQuantity, baseUnitPrice, baseUnitID, err := s.convertQuantityToBaseUnit(ctx, req.Items[i].Quantity, req.Items[i].UnitPrice, *req.Items[i].UnitID)
			if err != nil {
				log.WithFields(logrus.Fields{
					"operation": "UpdatePurchaseOrder",
					"unit_id":   *req.Items[i].UnitID,
					"error":     err,
				}).Error("Failed to convert quantity to base unit")
				return nil, pkg.ErrFailedToConvertQuantityToBaseUnit(ctx, *req.Items[i].UnitID, err)
			}
			req.Items[i].Quantity = baseQuantity
			req.Items[i].UnitPrice = baseUnitPrice
			req.Items[i].UnitID = &baseUnitID
		}
	}

	po, err := s.purchaseOrderRepo.UpdatePurchaseOrder(ctx, id, req)
	if err != nil {
		log.WithFields(logrus.Fields{
			"operation":         "UpdatePurchaseOrder",
			"purchase_order_id": id,
			"error":             err,
		}).Error("Failed to update purchase order")
		// Check if error is already an AppError, return it directly
		var appErr *pkg.AppError
		if errors.As(err, &appErr) {
			return nil, err
		}
		return nil, pkg.ErrFailedToUpdatePurchaseOrder(ctx, err)
	}

	// Ensure every PO item has a POItemSellingPrice row. Idempotent — existing items
	// are skipped, so this only inserts rows for items added by this update.
	if err := s.createPOItemSellingPrices(ctx, po); err != nil {
		log.WithFields(logrus.Fields{
			"operation":         "UpdatePurchaseOrder",
			"purchase_order_id": po.ID,
			"error":             err.Error(),
		}).Error("Failed to ensure PO item selling prices")
	}

	// Calculate total amount based on items
	po.TotalAmount = po.CalculateTotalAmount()

	log.WithFields(logrus.Fields{
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
	// @todo: fetch POI's product unit and use v2 unit service method to convert the quantity to same unit as product unit.
	// Context: currently, we're converting the quantity of POI at PO creation step.
	// That why's we might not this step yet. In the future, we should keep the POI unit the same as user input.
	// Quantity unit conversion in case POI unit is different from product unit.

	po, err := s.purchaseOrderRepo.ReceiveInventory(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to persist delivery update: %w", err)
	}

	po.TotalAmount = po.CalculateTotalAmount()
	return po, nil
}

// convertQuantityFromBaseUnit converts a quantity from base unit to the given target unit
// importPurchaseOrderItem represents a single item row in the import file
type importPurchaseOrderItem struct {
	rowNumber   int
	productName string
	unitName    string
	quantity    decimal.Decimal
	unitPrice   float64
	// Validated entities (populated after validation)
	product *models.Product
	unit    *models.Unit
}

// importPurchaseOrderData represents the parsed and validated data from the import file
type importPurchaseOrderData struct {
	inventoryName string
	orderDate     time.Time
	supplierName  string
	items         []importPurchaseOrderItem
	// Validated entities (populated after validation)
	inventory *models.Inventory
	supplier  *models.Supplier
}

// getRowCell safely retrieves a cell value from a row, returning empty string if index is out of bounds
func getRowCell(row []string, index int) string {
	if index >= 0 && index < len(row) {
		return strings.TrimSpace(row[index])
	}
	return ""
}

// parseSheetData parses a specific sheet from an XLSX file at the given path
// This function only validates file format (no database lookups)
func (s *purchaseOrderService) parseSheetData(ctx context.Context, filePath string, sheetName string) (*importPurchaseOrderData, error) {
	log.WithFields(logrus.Fields{
		"operation":  "parseSheetData",
		"file_path":  filePath,
		"sheet_name": sheetName,
	}).Debug("Parsing sheet data")

	// Open Excel file
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, pkg.ErrValidation("failed to open Excel file", err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.WithFields(logrus.Fields{
				"operation": "parseSheetData",
				"error":     err,
			}).Warn("Failed to close Excel file")
		}
	}()

	// Verify sheet exists
	sheets := f.GetSheetList()
	sheetExists := false
	for _, s := range sheets {
		if s == sheetName {
			sheetExists = true
			break
		}
	}
	if !sheetExists {
		return nil, pkg.ErrValidation(fmt.Sprintf("sheet '%s' not found in file", sheetName), nil)
	}

	// Read inventory name
	inventoryName, err := f.GetCellValue(sheetName, POImportInventoryNameCell)
	if err != nil {
		return nil, pkg.ErrValidation(fmt.Sprintf("failed to read inventory name from cell %s", POImportInventoryNameCell), err)
	}
	inventoryName = strings.TrimSpace(inventoryName)
	if inventoryName == "" {
		return nil, pkg.ErrValidation("inventory name is empty in cell A1", nil)
	}

	// Read and parse date
	dateStr, err := f.GetCellValue(sheetName, POImportDateCell)
	if err != nil {
		return nil, pkg.ErrValidation(fmt.Sprintf("failed to read date from cell %s", POImportDateCell), err)
	}
	dateStr = strings.TrimSpace(dateStr)
	dateStr = strings.TrimPrefix(dateStr, POImportDatePrefix)
	dateStr = strings.TrimSpace(dateStr)
	orderDate, err := time.Parse(POImportDateFormat, dateStr)
	if err != nil {
		return nil, pkg.ErrValidation(fmt.Sprintf("invalid date format in cell %s (expected DD/MM/YYYY): %s", POImportDateCell, dateStr), err)
	}

	// Read supplier name
	supplierName, err := f.GetCellValue(sheetName, POImportSupplierNameCell)
	if err != nil {
		return nil, pkg.ErrValidation(fmt.Sprintf("failed to read supplier name from cell %s", POImportSupplierNameCell), err)
	}
	supplierName = strings.TrimSpace(supplierName)
	if supplierName == "" {
		return nil, pkg.ErrValidation("supplier name is empty in cell A3", nil)
	}

	// Read all rows from the sheet
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, pkg.ErrValidation("failed to read Excel rows", err)
	}

	// Parse data rows
	batchError := pkg.ErrValidationBatchError()
	var items []importPurchaseOrderItem
	for i := POImportDataStartRow - 1; i < len(rows); i++ {
		var foundRowError bool
		row := rows[i]
		rowNumber := i + 1

		// Check if this is the last row by checking first cell for end marker
		firstCell := getRowCell(row, POImportColNumber)
		if strings.Contains(firstCell, POImportEndMarker) {
			log.WithFields(logrus.Fields{
				"operation":  "parseSheetData",
				"sheet_name": sheetName,
				"row_number": rowNumber,
			}).Debug("Reached end marker, stopping data parsing")
			break
		}

		// Parse values from exact cell positions
		productName := getRowCell(row, POImportColProductName)
		unitName := getRowCell(row, POImportColUnit)
		quantityStr := getRowCell(row, POImportColQuantity)
		unitPriceStr := getRowCell(row, POImportColUnitPrice)

		// Skip completely empty rows (all required fields are empty)
		if productName == "" &&
			unitName == "" &&
			quantityStr == "" &&
			unitPriceStr == "" {
			continue
		}

		// Validate required fields
		if productName == "" {
			batchError.AddLocation(fmt.Sprintf("row %d", rowNumber), "Product name is required")
			foundRowError = true
		}
		if unitName == "" {
			batchError.AddLocation(fmt.Sprintf("row %d", rowNumber), "Unit is required")
			foundRowError = true
		}

		quantity, err := validateUploadPOQuantity(quantityStr)
		if err != nil {
			batchError.AddLocation(fmt.Sprintf("row %d", rowNumber), err.Error())
			foundRowError = true
		}

		unitPrice, err := validateUploadPOUnitPrice(unitPriceStr)
		if err != nil {
			batchError.AddLocation(fmt.Sprintf("row %d", rowNumber), err.Error())
			foundRowError = true
		}

		if foundRowError {
			continue
		}

		// Add valid item
		items = append(items, importPurchaseOrderItem{
			rowNumber:   rowNumber,
			productName: productName,
			unitName:    unitName,
			quantity:    quantity,
			unitPrice:   unitPrice,
		})
	}

	if batchError.HasErrors() {
		return nil, batchError
	}

	if len(items) == 0 {
		return nil, pkg.ErrEmptyDataFile(ctx)
	}

	log.WithFields(logrus.Fields{
		"operation":      "parseSheetData",
		"sheet_name":     sheetName,
		"inventory_name": inventoryName,
		"supplier_name":  supplierName,
		"order_date":     orderDate,
		"items_count":    len(items),
	}).Debug("Successfully parsed sheet data")

	return &importPurchaseOrderData{
		inventoryName: inventoryName,
		orderDate:     orderDate,
		supplierName:  supplierName,
		items:         items,
	}, nil
}

// validateUploadPOUnitPrice validates the unit price of a row in the upload purchase order file
func validateUploadPOUnitPrice(unitPriceStr string) (float64, error) {
	if unitPriceStr == "" {
		return 0, fmt.Errorf("Unit price is required")
	}

	unitPrice, err := strconv.ParseFloat(unitPriceStr, 64)
	if err != nil {
		return 0, fmt.Errorf("Invalid unit price format: %s", unitPriceStr)
	}

	if unitPrice < 0 {
		return 0, fmt.Errorf("Unit price cannot be negative")
	}

	return unitPrice, nil
}

func validateUploadPOQuantity(quantityStr string) (decimal.Decimal, error) {
	if quantityStr == "" {
		return decimal.Zero, fmt.Errorf("Quantity is required")
	}

	quantity, err := decimal.NewFromString(quantityStr)
	if err != nil {
		return decimal.Zero, fmt.Errorf("Invalid quantity format: %s", quantityStr)
	}

	if quantity.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, fmt.Errorf("Quantity must be greater than zero")
	}

	return quantity, nil
}

// validateAllSheets validates all sheets in an XLSX file (format validation only, no DB lookups)
func (s *purchaseOrderService) validateAllSheets(ctx context.Context, filePath string) ([]dto.SheetValidationResult, error) {
	log.WithFields(logrus.Fields{
		"operation": "validateAllSheets",
		"file_path": filePath,
	}).Info("Validating all sheets in file")

	// Open Excel file
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, pkg.ErrValidation("failed to open Excel file", err)
	}
	defer f.Close()

	// Get all sheet names
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, pkg.ErrValidation("Excel file has no sheets", nil)
	}

	results := make([]dto.SheetValidationResult, 0, len(sheets))

	// Validate each sheet
	for _, sheetName := range sheets {
		result := dto.SheetValidationResult{
			SheetName: sheetName,
		}

		// Try to parse the sheet
		data, err := s.parseSheetData(ctx, filePath, sheetName)
		if err != nil {
			// Format validation failed
			result.IsValid = false

			// Check if it's a BatchError with structured errors
			var batchErr *pkg.BatchError
			if errors.As(err, &batchErr) {
				// Use the BatchError directly
				result.Error = batchErr
			} else {
				// Single error - wrap in BatchError
				batchError := pkg.NewBatchError(pkg.ErrorCodeValidation, "Sheet validation failed", err)
				batchError.AddLocation("sheet", err.Error())
				result.Error = batchError
			}
		} else {
			// Format validation succeeded - extract preview
			result.IsValid = true
			result.Preview = &dto.SheetPreview{
				InventoryName: data.inventoryName,
				OrderDate:     data.orderDate.Format("2006-01-02"),
				SupplierName:  data.supplierName,
				ItemsCount:    len(data.items),
			}
		}

		results = append(results, result)
	}

	log.WithFields(logrus.Fields{
		"operation":    "validateAllSheets",
		"sheets_count": len(results),
	}).Info("Completed validation of all sheets")

	return results, nil
}

// parseAndValidateSheet parses and validates a specific sheet (both format and DB validation)
func (s *purchaseOrderService) parseAndValidateSheet(ctx context.Context, filePath string, sheetName string) (*importPurchaseOrderData, error) {
	log.WithFields(logrus.Fields{
		"operation":  "parseAndValidateSheet",
		"sheet_name": sheetName,
	}).Info("Parsing and validating sheet")

	// Parse sheet data (format validation)
	data, err := s.parseSheetData(ctx, filePath, sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to parse sheet: %w", err)
	}

	// Validate entities exist in database (populates validated fields in data)
	err = s.validateImportPurchaseOrderData(ctx, data)
	if err != nil {
		return nil, fmt.Errorf("failed to validate entities: %w", err)
	}

	log.WithFields(logrus.Fields{
		"operation":       "parseAndValidateSheet",
		"sheet_name":      sheetName,
		"validated_items": len(data.items),
	}).Info("Successfully parsed and validated sheet")

	return data, nil
}

// validateImportPurchaseOrderData validates that all entities in import data exist in the database.
func (s *purchaseOrderService) validateImportPurchaseOrderData(
	ctx context.Context,
	data *importPurchaseOrderData,
) error {
	log.WithFields(logrus.Fields{
		"operation": "validateImportDataEntities",
	}).Info("Validating import data entities")

	batchError := pkg.ErrValidationBatchError()

	// Validate inventory exists
	inventory, err := s.inventoryRepo.GetByName(ctx, data.inventoryName)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			batchError.AddLocation("inventory", fmt.Sprintf("Inventory not found: %s", data.inventoryName))
		} else {
			return fmt.Errorf("failed to lookup inventory: %w", err)
		}
	} else {
		data.inventory = inventory
	}

	// Validate supplier exists
	supplier, err := s.supplierRepo.GetByName(ctx, data.supplierName)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			batchError.AddLocation("supplier", fmt.Sprintf("Supplier not found: %s", data.supplierName))
		} else {
			return fmt.Errorf("failed to lookup supplier: %w", err)
		}
	} else {
		data.supplier = supplier
	}

	// Collect all unique product names and unit names
	productNamesSet := make(map[string]bool)
	unitNamesSet := make(map[string]bool)
	for _, item := range data.items {
		productNamesSet[item.productName] = true
		unitNamesSet[strings.ToUpper(item.unitName)] = true
	}

	// Convert sets to slices
	productNames := make([]string, 0, len(productNamesSet))
	for name := range productNamesSet {
		productNames = append(productNames, name)
	}
	unitNames := make([]string, 0, len(unitNamesSet))
	for name := range unitNamesSet {
		unitNames = append(unitNames, name)
	}

	// Bulk fetch all products by names (returns map[string]*models.Product)
	productMap, err := s.productService.GetMapByNames(ctx, productNames)
	if err != nil {
		return fmt.Errorf("failed to bulk fetch products: %w", err)
	}

	// Bulk fetch all units by names (returns map[string]*models.Unit)
	unitMap, err := s.unitService.GetMapByNames(ctx, unitNames)
	if err != nil {
		return fmt.Errorf("failed to bulk fetch units: %w", err)
	}

	// Validate each item and populate validated entities
	itemsToRemove := make(map[int]bool) // Track items that failed validation
	for i := range data.items {
		item := &data.items[i]

		// Validate product exists
		product, exists := productMap[strings.ToUpper(item.productName)]
		if !exists {
			batchError.AddLocation(fmt.Sprintf("row %d", item.rowNumber), fmt.Sprintf("Product not found: %s", item.productName))
			itemsToRemove[i] = true
			continue
		}

		// Validate unit exists (lookup by uppercased name for consistency)
		unit, exists := unitMap[strings.ToUpper(item.unitName)]
		if !exists {
			batchError.AddLocation(fmt.Sprintf("row %d", item.rowNumber), fmt.Sprintf("Unit not found: %s", item.unitName))
			itemsToRemove[i] = true
			continue
		}

		// Validate unit compatibility with product
		productBaseUnitID, err := s.getBaseUnitID(ctx, product.UnitID)
		if err != nil {
			return fmt.Errorf("row %d: failed to get product base unit: %w", item.rowNumber, err)
		}

		itemBaseUnitID, err := s.getBaseUnitID(ctx, unit.ID)
		if err != nil {
			return fmt.Errorf("row %d: failed to get item base unit: %w", item.rowNumber, err)
		}

		if itemBaseUnitID != productBaseUnitID {
			batchError.AddLocation(fmt.Sprintf("row %d", item.rowNumber),
				fmt.Sprintf("Unit %s is not compatible with product %s (different base units)",
					item.unitName, item.productName))
			itemsToRemove[i] = true
			continue
		}

		// Populate validated entities
		item.product = product
		item.unit = unit
	}

	// If there are validation errors, return them all
	if batchError.HasErrors() {
		log.WithFields(logrus.Fields{
			"operation":   "validateImportDataEntities",
			"error_count": len(batchError.Locations),
			"errors":      batchError.Locations,
		}).Error("Entity validation failed")
		return batchError
	}

	// Ensure we have at least one valid item after validation
	validItemCount := len(data.items) - len(itemsToRemove)
	if validItemCount == 0 {
		return pkg.ErrValidation("no valid items found after validation", nil)
	}

	log.WithFields(logrus.Fields{
		"operation":       "validateImportDataEntities",
		"validated_items": validItemCount,
	}).Info("Successfully validated import data entities")

	return nil
}

// UploadAndValidatePurchaseOrderFile handles file upload and validation
func (s *purchaseOrderService) UploadAndValidatePurchaseOrderFile(ctx context.Context, file multipart.File, filename string, fileStorageService FileStorageService) (*dto.UploadPurchaseOrderFileResponse, error) {
	log.WithFields(logrus.Fields{
		"operation": "UploadAndValidatePurchaseOrderFile",
		"filename":  filename,
	}).Info("Uploading and validating purchase order file")

	// Save file
	fileUID, extension, err := fileStorageService.SaveFile(ctx, file, filename, pkg.FileCategoryPurchaseOrder)
	if err != nil {
		return nil, fmt.Errorf("failed to save file: %w", err)
	}

	// Get file path for validation
	filePath, err := fileStorageService.GetFilePath(ctx, fileUID, extension, pkg.FileCategoryPurchaseOrder)
	if err != nil {
		return nil, fmt.Errorf("failed to get file path: %w", err)
	}

	// Validate all sheets
	sheetResults, err := s.validateAllSheets(ctx, filePath)
	if err != nil {
		// Clean up file if validation completely fails
		_ = fileStorageService.DeleteFile(ctx, fileUID, extension, pkg.FileCategoryPurchaseOrder)
		return nil, fmt.Errorf("failed to validate sheets: %w", err)
	}

	response := &dto.UploadPurchaseOrderFileResponse{
		FileUID:       fileUID,
		FileName:      filename,
		FileExtension: extension,
		UploadedAt:    time.Now(),
		Sheets:        sheetResults,
	}

	log.WithFields(logrus.Fields{
		"operation":    "UploadAndValidatePurchaseOrderFile",
		"file_uid":     fileUID,
		"sheets_count": len(sheetResults),
	}).Info("Successfully uploaded and validated file")

	return response, nil
}

// ProcessImportPurchaseOrder processes an uploaded file and creates a purchase order
func (s *purchaseOrderService) ProcessImportPurchaseOrder(ctx context.Context, fileUID string, extension string, sheetName string, fileStorageService FileStorageService) (*models.PurchaseOrder, error) {
	ctx, span := pkg.Tracer.Start(ctx, "purchaseOrderService.ProcessImportPurchaseOrder")
	defer span.End()

	log.WithFields(logrus.Fields{
		"operation":  "ProcessImportPurchaseOrder",
		"file_uid":   fileUID,
		"sheet_name": sheetName,
	}).Info("Processing purchase order import")

	// Get file path
	filePath, err := fileStorageService.GetFilePath(ctx, fileUID, extension, pkg.FileCategoryPurchaseOrder)
	if err != nil {
		return nil, err
	}

	// Parse and validate the specific sheet
	validated, err := s.parseAndValidateSheet(ctx, filePath, sheetName)
	if err != nil {
		log.WithFields(logrus.Fields{
			"operation":  "ProcessImportPurchaseOrder",
			"sheet_name": sheetName,
			"error":      err,
		}).Error("Failed to parse and validate sheet")
		return nil, err
	}

	// Map validated data to PurchaseOrder model
	purchaseOrder := &models.PurchaseOrder{
		InventoryID: &validated.inventory.ID,
		Items:       make([]*models.PurchaseOrderItem, 0, len(validated.items)),
	}

	// Add items
	for _, item := range validated.items {
		productID := item.product.ID
		supplierID := validated.supplier.ID
		unitID := item.unit.ID

		purchaseOrder.Items = append(purchaseOrder.Items, &models.PurchaseOrderItem{
			ProductID:  &productID,
			SupplierID: &supplierID,
			UnitID:     &unitID,
			Quantity:   item.quantity,
			UnitPrice:  item.unitPrice,
		})
	}

	// Create the purchase order using existing CreatePurchaseOrder method
	err = s.CreatePurchaseOrder(ctx, purchaseOrder)
	if err != nil {
		log.WithFields(logrus.Fields{
			"operation": "ProcessImportPurchaseOrder",
			"error":     err,
		}).Error("Failed to create purchase order from import")
		return nil, fmt.Errorf("failed to create purchase order: %w", err)
	}

	log.WithFields(logrus.Fields{
		"operation":         "ProcessImportPurchaseOrder",
		"purchase_order_id": purchaseOrder.ID,
		"order_number":      purchaseOrder.OrderNumber,
		"items_count":       len(purchaseOrder.Items),
		"total_amount":      purchaseOrder.TotalAmount,
	}).Info("Successfully processed purchase order import")

	return purchaseOrder, nil
}
