package services

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"cim-backend/internal/mocks/repositorymocks"
	"cim-backend/internal/models"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
	"cim-backend/pkg/testutil/fixture"
)

func Test_generatePurchaseOrderNumber(t *testing.T) {
	// Setup
	mockRepo := repositorymocks.NewPurchaseOrderRepository(t)
	// Note: Service mocks are not used in this test as it's not needed for generatePurchaseOrderNumber
	service := NewPurchaseOrderService(mockRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).(*purchaseOrderService)

	t.Run("should generate valid purchase order number format", func(t *testing.T) {
		// Execute
		orderNumber, err := service.generatePurchaseOrderNumber()

		// Assertions
		require.NoError(t, err)
		assert.NotEmpty(t, orderNumber)

		// Verify format: PO-YYMMDD-HHMMSS-XXXX
		pattern := `^PO-\d{6}-\d{6}-[A-Z0-9]{4}$`
		matched, err := regexp.MatchString(pattern, orderNumber)
		require.NoError(t, err)
		assert.True(t, matched, "Order number %s should match pattern %s", orderNumber, pattern)

		// Verify length: PO- (3) + YYMMDD (6) + - (1) + HHMMSS (6) + - (1) + XXXX (4) = 21 characters
		expectedLength := len("PO-250925-143052-ABCD")
		assert.Equal(t, expectedLength, len(orderNumber))
	})

	t.Run("should generate unique order numbers on consecutive calls", func(t *testing.T) {
		// Execute multiple generations
		orderNumber1, err1 := service.generatePurchaseOrderNumber()
		orderNumber2, err2 := service.generatePurchaseOrderNumber()

		// Assertions
		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.NotEqual(t, orderNumber1, orderNumber2, "Consecutive calls should generate different order numbers")
	})
}

func Test_convertQuantityToBaseUnit(t *testing.T) {
	baseUnit := models.Unit{Base: models.Base{ID: 1}, UnitType: "general", ConversionFactor: 1, Level: 1}
	derivedID := uint(2)

	t.Run("converts price in decimal without float rounding", func(t *testing.T) {
		mockUnitRepo := repositorymocks.NewUnitRepository(t)
		service := NewPurchaseOrderService(nil, nil, mockUnitRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).(*purchaseOrderService)

		derived := baseUnit
		derived.Base = models.Base{ID: derivedID}
		derived.Level = 2
		derived.ConversionFactor = 3
		derived.BaseUnitID = &baseUnit.Base.ID

		mockUnitRepo.On("GetByID", mock.Anything, derivedID).Return(&derived, nil)
		mockUnitRepo.On("GetByID", mock.Anything, baseUnit.ID).Return(&baseUnit, nil)

		baseQty, basePrice, baseUnitID, err := service.convertQuantityToBaseUnit(
			context.Background(), decimal.NewFromInt(10), 0.3, derivedID)

		require.NoError(t, err)
		assert.Equal(t, baseUnit.ID, baseUnitID)
		assert.True(t, baseQty.Equal(decimal.NewFromInt(30)), "10 * 3 = 30, got %s", baseQty)
		// Float division 0.3/3 yields 0.09999999999999999; decimal yields exactly 0.1.
		assert.Equal(t, 0.1, basePrice)
	})

	t.Run("rejects a zero conversion factor", func(t *testing.T) {
		mockUnitRepo := repositorymocks.NewUnitRepository(t)
		service := NewPurchaseOrderService(nil, nil, mockUnitRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).(*purchaseOrderService)

		derived := baseUnit
		derived.Base = models.Base{ID: derivedID}
		derived.Level = 2
		derived.ConversionFactor = 0
		derived.BaseUnitID = &baseUnit.Base.ID

		mockUnitRepo.On("GetByID", mock.Anything, derivedID).Return(&derived, nil)

		_, _, _, err := service.convertQuantityToBaseUnit(
			context.Background(), decimal.NewFromInt(10), 100, derivedID)

		require.Error(t, err)
	})
}

func TestCreatePurchaseOrder(t *testing.T) {
	t.Run("should create purchase order with auto-generated order number when empty", func(t *testing.T) {
		// Setup
		mockRepo := repositorymocks.NewPurchaseOrderRepository(t)
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockUnitRepo := repositorymocks.NewUnitRepository(t)
		service := NewPurchaseOrderService(mockRepo, nil, mockUnitRepo, mockProductRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil)

		inventory := fixture.ValidInventory()
		unit := fixture.ValidBaseUnit()
		product := fixture.ValidProduct(unit.ID)
		supplier := fixture.ValidSupplier()
		po := fixture.ValidPurchaseOrder(inventory.ID, product.ID, supplier.ID, unit.ID)

		// Setup mock - expect Create to be called with purchase order that has generated order number
		mockProductRepo.On("GetByID", mock.Anything, product.ID).Return(&product, nil)
		mockUnitRepo.On("GetByID", mock.Anything, unit.ID).Return(&unit, nil)

		mockRepo.On("Create", mock.Anything, mock.MatchedBy(func(po *models.PurchaseOrder) bool {
			// Check that order number was generated and follows the expected pattern
			pattern := `^PO-\d{6}-\d{6}-[A-Z0-9]{4}$`
			matched, _ := regexp.MatchString(pattern, po.OrderNumber)
			return matched && po.OrderNumber != ""
		})).Return(nil)

		mockRepo.On("GetByID", mock.AnythingOfType("uint")).Return(&po, nil)

		// Execute
		err := service.CreatePurchaseOrder(context.Background(), &po)

		// Assertions
		require.NoError(t, err)
		assert.NotEmpty(t, po.OrderNumber, "Order number should be generated")

		// Verify the generated order number format
		pattern := `^PO-\d{6}-\d{6}-[A-Z0-9]{4}$`
		matched, err := regexp.MatchString(pattern, po.OrderNumber)
		require.NoError(t, err)
		assert.True(t, matched, "Generated order number should match expected format")

		mockRepo.AssertExpectations(t)
	})

	t.Run("should create purchase order without changing provided order number", func(t *testing.T) {
		// Setup
		mockRepo := repositorymocks.NewPurchaseOrderRepository(t)
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockUnitRepo := repositorymocks.NewUnitRepository(t)
		service := NewPurchaseOrderService(mockRepo, nil, mockUnitRepo, mockProductRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil)

		inventory := fixture.ValidInventory()
		unit := fixture.ValidBaseUnit()
		product := fixture.ValidProduct(unit.ID)
		supplier := fixture.ValidSupplier()
		po := fixture.ValidPurchaseOrder(inventory.ID, product.ID, supplier.ID, unit.ID)
		existingOrderNumber := "CUSTOM-PO-123"
		po.OrderNumber = existingOrderNumber

		// Setup mock
		mockProductRepo.On("GetByID", mock.Anything, product.ID).Return(&product, nil)
		mockUnitRepo.On("GetByID", mock.Anything, unit.ID).Return(&unit, nil)
		mockRepo.On("Create", mock.Anything, mock.MatchedBy(func(po *models.PurchaseOrder) bool {
			return po.OrderNumber == existingOrderNumber
		})).Return(nil).Once()
		mockRepo.On("GetByID", mock.AnythingOfType("uint")).Return(&po, nil).Once()

		// Execute
		err := service.CreatePurchaseOrder(context.Background(), &po)

		// Assertions
		require.NoError(t, err)
		assert.Equal(t, existingOrderNumber, po.OrderNumber, "Existing order number should not be changed")

		mockRepo.AssertExpectations(t)
	})

	t.Run("should return error when repository fails", func(t *testing.T) {
		unit := &models.Unit{
			Base:             models.Base{ID: 1},
			Name:             "unit",
			Symbol:           "unit",
			UnitType:         "general",
			ConversionFactor: 1,
		}

		product := &models.Product{
			Base:   models.Base{ID: 1},
			Name:   "product",
			UnitID: unit.ID,
		}

		// Setup
		mockRepo := repositorymocks.NewPurchaseOrderRepository(t)
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockUnitRepo := repositorymocks.NewUnitRepository(t)
		service := NewPurchaseOrderService(mockRepo, nil, mockUnitRepo, mockProductRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil)

		purchaseOrder := &models.PurchaseOrder{
			InventoryID: pkg.Ptr(uint(1)),
			Status:      models.PurchaseOrderStatusOrderPlaced,
			TotalAmount: decimal.NewFromFloat(1500.50),
			Notes:       "Test purchase order",
			Items: []*models.PurchaseOrderItem{
				{
					ProductID:  pkg.Ptr(product.ID),
					SupplierID: pkg.Ptr(uint(1)),
					UnitID:     pkg.Ptr(unit.ID),
					Quantity:   decimal.NewFromInt(1),
				},
			},
		}

		// Setup mock - repository returns error
		expectedError := errors.New("database connection failed")
		mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.PurchaseOrder")).Return(expectedError).Once()

		mockProductRepo.On("GetByID", mock.Anything, mock.AnythingOfType("uint")).Return(product, nil).Once()

		mockUnitRepo.On("GetByID", mock.Anything, mock.AnythingOfType("uint")).Return(unit, nil)

		// Execute
		err := service.CreatePurchaseOrder(context.Background(), purchaseOrder)

		// Assertions
		require.Error(t, err)
		assert.Equal(t, expectedError, err)
		assert.NotEmpty(t, purchaseOrder.OrderNumber, "Order number should still be generated even if repository fails")

		mockRepo.AssertExpectations(t)
	})

	t.Run("should regenerate order number and retry once on duplicate key, then run post-create steps once", func(t *testing.T) {
		// Reproduces the #84 concurrency collision deterministically: the first
		// Create fails with a 23505 duplicate-key on the auto-generated
		// order_number; the service must regenerate the number, reset the
		// in-memory PKs, retry the Create, and then run the post-create steps once.
		mockRepo := repositorymocks.NewPurchaseOrderRepository(t)
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockUnitRepo := repositorymocks.NewUnitRepository(t)
		service := NewPurchaseOrderService(mockRepo, nil, mockUnitRepo, mockProductRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil)

		inventory := fixture.ValidInventory()
		unit := fixture.ValidBaseUnit()
		product := fixture.ValidProduct(unit.ID)
		supplier := fixture.ValidSupplier()
		po := fixture.ValidPurchaseOrder(inventory.ID, product.ID, supplier.ID, unit.ID)

		mockProductRepo.On("GetByID", mock.Anything, product.ID).Return(&product, nil)
		mockUnitRepo.On("GetByID", mock.Anything, unit.ID).Return(&unit, nil)

		orderNumberPattern := `^PO-\d{6}-\d{6}-[A-Z0-9]{4}$`

		// Capture the state seen by Create on each attempt via a side-effect-free
		// Run callback (matchers must stay pure — testify re-evaluates them).
		type createSnapshot struct {
			orderNumber     string
			poID            uint
			itemPOIDsAllNil bool
		}
		var snapshots []createSnapshot
		recordCreate := func(args mock.Arguments) {
			poArg := args.Get(1).(*models.PurchaseOrder)
			allNil := true
			for _, item := range poArg.Items {
				if item != nil && item.PurchaseOrderID != nil {
					allNil = false
				}
			}
			snapshots = append(snapshots, createSnapshot{
				orderNumber:     poArg.OrderNumber,
				poID:            poArg.ID,
				itemPOIDsAllNil: allNil,
			})
		}

		// Attempt 1: the repository detected the order_number unique violation and
		// translated it into the typed pkg.ErrDuplicateOrderNumber domain error — the
		// only signal the service uses to regenerate-and-retry. (The raw DB 23505 /
		// constraint-name string never reaches the service.)
		dupErr := pkg.ErrDuplicateOrderNumber(
			errors.New("ERROR: duplicate key value violates unique constraint \"purchase_orders_order_number_key\" (SQLSTATE 23505)"))
		mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.PurchaseOrder")).
			Run(recordCreate).Return(dupErr).Once()
		// Attempt 2: succeeds.
		mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.PurchaseOrder")).
			Run(recordCreate).Return(nil).Once()

		// Post-create steps (selling-price creation + reload) must run exactly once.
		mockRepo.On("GetByID", mock.AnythingOfType("uint")).Return(&po, nil).Once()

		err := service.CreatePurchaseOrder(context.Background(), &po)

		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
		mockRepo.AssertNumberOfCalls(t, "Create", 2)
		mockRepo.AssertNumberOfCalls(t, "GetByID", 1)

		require.Len(t, snapshots, 2, "Create should be attempted exactly twice")
		// Both attempts use a valid auto-generated order number.
		for i, s := range snapshots {
			matched, _ := regexp.MatchString(orderNumberPattern, s.orderNumber)
			assert.True(t, matched, "attempt %d order number %q should match %s", i+1, s.orderNumber, orderNumberPattern)
		}
		// The retry regenerated the order number rather than re-inserting the same one.
		assert.NotEqual(t, snapshots[0].orderNumber, snapshots[1].orderNumber, "order number should be regenerated on retry")
		// PK reset fired before the retry: parent ID zeroed and item FK cleared.
		assert.Equal(t, uint(0), snapshots[1].poID, "purchase order ID should be reset before retry")
		assert.True(t, snapshots[1].itemPOIDsAllNil, "item PurchaseOrderID should be reset to nil before retry")
	})

	t.Run("should not regenerate or retry a caller-provided order number on duplicate key", func(t *testing.T) {
		// A caller-supplied order number is never renumbered: a duplicate-key on it
		// surfaces as an error after a single Create, with no regeneration.
		mockRepo := repositorymocks.NewPurchaseOrderRepository(t)
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockUnitRepo := repositorymocks.NewUnitRepository(t)
		service := NewPurchaseOrderService(mockRepo, nil, mockUnitRepo, mockProductRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil)

		inventory := fixture.ValidInventory()
		unit := fixture.ValidBaseUnit()
		product := fixture.ValidProduct(unit.ID)
		supplier := fixture.ValidSupplier()
		po := fixture.ValidPurchaseOrder(inventory.ID, product.ID, supplier.ID, unit.ID)
		providedOrderNumber := "CUSTOM-PO-123"
		po.OrderNumber = providedOrderNumber

		mockProductRepo.On("GetByID", mock.Anything, product.ID).Return(&product, nil)
		mockUnitRepo.On("GetByID", mock.Anything, unit.ID).Return(&unit, nil)

		// The repository translates an order_number violation to the typed
		// pkg.ErrDuplicateOrderNumber regardless of who supplied the number. The
		// regenerate-and-retry gate is keyed on the number being auto-generated, so a
		// caller-supplied number must surface this error unwrapped after a single
		// Create — never renumbered or retried.
		dupErr := pkg.ErrDuplicateOrderNumber(
			errors.New("ERROR: duplicate key value violates unique constraint \"purchase_orders_order_number_key\" (SQLSTATE 23505)"))
		mockRepo.On("Create", mock.Anything, mock.MatchedBy(func(po *models.PurchaseOrder) bool {
			return po.OrderNumber == providedOrderNumber
		})).Return(dupErr).Once()

		err := service.CreatePurchaseOrder(context.Background(), &po)

		require.Error(t, err)
		assert.Equal(t, dupErr, err, "duplicate key on a caller-provided number should surface unwrapped, not retried")
		assert.Equal(t, providedOrderNumber, po.OrderNumber, "caller-provided order number must not be regenerated")
		mockRepo.AssertExpectations(t)
		mockRepo.AssertNumberOfCalls(t, "Create", 1)
	})

	t.Run("should not regenerate or retry when a non-order-number 23505 (item constraint) is returned", func(t *testing.T) {
		// Regression for the Codex P2 finding: repo.Create also inserts
		// PurchaseOrderItems guarded by idx_product_supplier_po. The repository only
		// translates the order_number violation into pkg.ErrDuplicateOrderNumber; an
		// item-constraint violation passes through untranslated (here the raw item
		// 23505). The service keys its regenerate-and-retry solely on the typed
		// pkg.ErrDuplicateOrderNumber, so it must NOT treat this as an order-number
		// collision: no regeneration, a single Create attempt, and the real
		// item-constraint error surfaced unwrapped (not the "...after N retries due to
		// duplicate key conflicts" wrap).
		mockRepo := repositorymocks.NewPurchaseOrderRepository(t)
		mockProductRepo := repositorymocks.NewProductRepository(t)
		mockUnitRepo := repositorymocks.NewUnitRepository(t)
		service := NewPurchaseOrderService(mockRepo, nil, mockUnitRepo, mockProductRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil)

		inventory := fixture.ValidInventory()
		unit := fixture.ValidBaseUnit()
		product := fixture.ValidProduct(unit.ID)
		supplier := fixture.ValidSupplier()
		po := fixture.ValidPurchaseOrder(inventory.ID, product.ID, supplier.ID, unit.ID)
		// Auto-numbered request (no caller-supplied order number).
		po.OrderNumber = ""

		mockProductRepo.On("GetByID", mock.Anything, product.ID).Return(&product, nil)
		mockUnitRepo.On("GetByID", mock.Anything, unit.ID).Return(&unit, nil)

		// 23505 from the ITEM unique index, not the order_number constraint.
		itemDupErr := errors.New("ERROR: duplicate key value violates unique constraint \"idx_product_supplier_po\" (SQLSTATE 23505)")

		var orderNumbers []string
		mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.PurchaseOrder")).
			Run(func(args mock.Arguments) {
				orderNumbers = append(orderNumbers, args.Get(1).(*models.PurchaseOrder).OrderNumber)
			}).
			Return(itemDupErr).Once()

		err := service.CreatePurchaseOrder(context.Background(), &po)

		require.Error(t, err)
		// The real item-constraint error must surface unwrapped — NOT mis-wrapped as
		// an order-number collision after exhausting retries.
		assert.Equal(t, itemDupErr, err, "an item-constraint 23505 must surface unwrapped, not be retried or wrapped as an order-number collision")
		assert.NotContains(t, err.Error(), "retries due to duplicate key conflicts", "must not wrap as an order-number-collision exhaustion error")
		mockRepo.AssertExpectations(t)
		// Single attempt: no regeneration loop.
		mockRepo.AssertNumberOfCalls(t, "Create", 1)
		require.Len(t, orderNumbers, 1, "Create must be attempted exactly once (no regenerate-and-retry)")
	})
}

func TestUpdatePurchaseOrderStatus_WithApprovalCheck(t *testing.T) {
	t.Run("should return error when trying to complete purchase order without approved payment receipt form", func(t *testing.T) {
		// Setup
		mockRepo := repositorymocks.NewPurchaseOrderRepository(t)
		mockPaymentRepo := repositorymocks.NewPaymentReceiptFormRepository(t)
		service := NewPurchaseOrderService(mockRepo, mockPaymentRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

		// Mock payment receipt form repository to return empty slice (no approved form found)
		mockPaymentRepo.On("GetLatestPaymentReceiptForms", mock.Anything, uint(1), models.PaymentReceiptFormStatusApproved, 0).Return([]*models.PaymentReceiptForm{}, nil)

		// Execute
		err := service.UpdatePurchaseOrderStatus(context.Background(), 1, models.PurchaseOrderStatusCompleted)

		// Assert
		require.Error(t, err)
		var appErr *pkg.AppError
		assert.ErrorAs(t, err, &appErr)
		assert.Equal(t, pkg.ErrorCodePurchaseOrderNoApprovedPaymentReceipt, appErr.Code)
		// Note: UpdateStatus should NOT be called since validation fails before reaching that point
	})

	t.Run("should complete purchase order when approved payment receipt form exists", func(t *testing.T) {
		// Setup
		mockRepo := repositorymocks.NewPurchaseOrderRepository(t)
		mockPaymentRepo := repositorymocks.NewPaymentReceiptFormRepository(t)
		service := NewPurchaseOrderService(mockRepo, mockPaymentRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

		// Mock payment receipt form repository to return an approved form
		approvedForm := &models.PaymentReceiptForm{
			Base:            models.Base{ID: 1},
			PurchaseOrderID: 1,
			Status:          models.PaymentReceiptFormStatusApproved,
		}
		mockPaymentRepo.On("GetLatestPaymentReceiptForms", mock.Anything, uint(1), models.PaymentReceiptFormStatusApproved, 0).Return([]*models.PaymentReceiptForm{approvedForm}, nil)

		// Mock the repository to return success for status update
		mockRepo.On("UpdateStatus", mock.Anything, uint(1), models.PurchaseOrderStatusCompleted).Return(nil)

		// Execute
		err := service.UpdatePurchaseOrderStatus(context.Background(), 1, models.PurchaseOrderStatusCompleted)

		// Assert
		assert.NoError(t, err)
	})
}

func Test_createPOItemSellingPrices(t *testing.T) {
	t.Run("creates pisp rows for all items when none exist", func(t *testing.T) {
		ctx := context.Background()
		spRepo := repositorymocks.NewSellingPriceRepository(t)
		service := &purchaseOrderService{sellingPriceRepo: spRepo}

		invID := uint(10)
		po := &models.PurchaseOrder{
			Base:        models.Base{ID: 1},
			InventoryID: &invID,
			Items: []*models.PurchaseOrderItem{
				{Base: models.Base{ID: 100}, ProductID: pkg.Ptr(uint(1))},
				{Base: models.Base{ID: 101}, ProductID: pkg.Ptr(uint(2))},
			},
		}

		spRepo.On("GetPOItemSellingPricesByPOItemIDs", ctx, []uint{100, 101}).Return(nil, nil).Once()
		spRepo.On("GetLatestForProducts", ctx, []uint{1, 2}, &invID, mock.Anything).Return(map[uint]*models.SellingPrice{
			1: {Base: models.Base{ID: 50}, ProductID: 1, Price: decimal.NewFromInt(20)},
			2: {Base: models.Base{ID: 51}, ProductID: 2, Price: decimal.NewFromInt(30)},
		}, nil).Once()
		spRepo.On("CreatePOItemSellingPrice", ctx, mock.MatchedBy(func(p *models.POItemSellingPrice) bool {
			return p.PurchaseOrderItemID == 100 && p.SellingPriceID != nil && *p.SellingPriceID == 50 && p.SellingPrice == nil
		})).Return(nil).Once()
		spRepo.On("CreatePOItemSellingPrice", ctx, mock.MatchedBy(func(p *models.POItemSellingPrice) bool {
			return p.PurchaseOrderItemID == 101 && p.SellingPriceID != nil && *p.SellingPriceID == 51 && p.SellingPrice == nil
		})).Return(nil).Once()

		err := service.createPOItemSellingPrices(ctx, po)
		require.NoError(t, err)
	})

	t.Run("idempotent: skips items that already have a pisp row", func(t *testing.T) {
		ctx := context.Background()
		spRepo := repositorymocks.NewSellingPriceRepository(t)
		service := &purchaseOrderService{sellingPriceRepo: spRepo}

		invID := uint(10)
		po := &models.PurchaseOrder{
			Base:        models.Base{ID: 1},
			InventoryID: &invID,
			Items: []*models.PurchaseOrderItem{
				{Base: models.Base{ID: 100}, ProductID: pkg.Ptr(uint(1))},
				{Base: models.Base{ID: 101}, ProductID: pkg.Ptr(uint(2))},
			},
		}

		spRepo.On("GetPOItemSellingPricesByPOItemIDs", ctx, []uint{100, 101}).Return([]*models.POItemSellingPrice{
			{PurchaseOrderItemID: 100},
		}, nil).Once()
		spRepo.On("GetLatestForProducts", ctx, []uint{1, 2}, &invID, mock.Anything).Return(map[uint]*models.SellingPrice{
			2: {Base: models.Base{ID: 51}, ProductID: 2, Price: decimal.NewFromInt(30)},
		}, nil).Once()
		// Only item 101 should get inserted; item 100 already has a pisp row.
		spRepo.On("CreatePOItemSellingPrice", ctx, mock.MatchedBy(func(p *models.POItemSellingPrice) bool {
			return p.PurchaseOrderItemID == 101
		})).Return(nil).Once()

		err := service.createPOItemSellingPrices(ctx, po)
		require.NoError(t, err)
	})

	t.Run("inserts pisp with nil selling_price_id when no ledger price exists", func(t *testing.T) {
		ctx := context.Background()
		spRepo := repositorymocks.NewSellingPriceRepository(t)
		service := &purchaseOrderService{sellingPriceRepo: spRepo}

		invID := uint(10)
		po := &models.PurchaseOrder{
			Base:        models.Base{ID: 1},
			InventoryID: &invID,
			Items: []*models.PurchaseOrderItem{
				{Base: models.Base{ID: 100}, ProductID: pkg.Ptr(uint(1))},
			},
		}

		spRepo.On("GetPOItemSellingPricesByPOItemIDs", ctx, []uint{100}).Return(nil, nil).Once()
		spRepo.On("GetLatestForProducts", ctx, []uint{1}, &invID, mock.Anything).Return(map[uint]*models.SellingPrice{}, nil).Once()
		spRepo.On("CreatePOItemSellingPrice", ctx, mock.MatchedBy(func(p *models.POItemSellingPrice) bool {
			return p.PurchaseOrderItemID == 100 && p.SellingPriceID == nil && p.SellingPrice == nil
		})).Return(nil).Once()

		err := service.createPOItemSellingPrices(ctx, po)
		require.NoError(t, err)
	})

	t.Run("no inserts when every item already has a pisp row", func(t *testing.T) {
		ctx := context.Background()
		spRepo := repositorymocks.NewSellingPriceRepository(t)
		service := &purchaseOrderService{sellingPriceRepo: spRepo}

		invID := uint(10)
		po := &models.PurchaseOrder{
			Base:        models.Base{ID: 1},
			InventoryID: &invID,
			Items: []*models.PurchaseOrderItem{
				{Base: models.Base{ID: 100}, ProductID: pkg.Ptr(uint(1))},
			},
		}

		spRepo.On("GetPOItemSellingPricesByPOItemIDs", ctx, []uint{100}).Return([]*models.POItemSellingPrice{
			{PurchaseOrderItemID: 100},
		}, nil).Once()
		spRepo.On("GetLatestForProducts", ctx, []uint{1}, &invID, mock.Anything).Return(map[uint]*models.SellingPrice{}, nil).Once()
		// No CreatePOItemSellingPrice expected — all items already have pisp.

		err := service.createPOItemSellingPrices(ctx, po)
		require.NoError(t, err)
	})

	t.Run("returns error when sellingPriceRepo is nil", func(t *testing.T) {
		service := &purchaseOrderService{sellingPriceRepo: nil}
		po := &models.PurchaseOrder{
			Items: []*models.PurchaseOrderItem{
				{Base: models.Base{ID: 100}, ProductID: pkg.Ptr(uint(1))},
			},
		}
		err := service.createPOItemSellingPrices(context.Background(), po)
		require.Error(t, err)
	})

	t.Run("returns error when PO has no items", func(t *testing.T) {
		spRepo := repositorymocks.NewSellingPriceRepository(t)
		service := &purchaseOrderService{sellingPriceRepo: spRepo}
		err := service.createPOItemSellingPrices(context.Background(), &models.PurchaseOrder{})
		require.Error(t, err)
	})
}

func TestUpdatePurchaseOrder_EnsuresPOItemSellingPrices(t *testing.T) {
	t.Run("calls createPOItemSellingPrices after successful repo update", func(t *testing.T) {
		ctx := context.Background()
		mockPORepo := repositorymocks.NewPurchaseOrderRepository(t)
		spRepo := repositorymocks.NewSellingPriceRepository(t)
		service := NewPurchaseOrderService(mockPORepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, spRepo)

		invID := uint(10)
		// Items have UnitID nil → service skips the unit-conversion path entirely.
		req := dto.UpdatePurchaseOrderRequest{
			InventoryID: &invID,
			Items: []dto.UpdatePurchaseOrderItemRequest{
				{ProductID: pkg.Ptr(uint(1)), SupplierID: pkg.Ptr(uint(2)), Quantity: decimal.NewFromInt(5)},
			},
		}
		// Repo returns a PO with one new item whose pisp doesn't yet exist.
		updatedPO := &models.PurchaseOrder{
			Base:        models.Base{ID: 1},
			InventoryID: &invID,
			Items: []*models.PurchaseOrderItem{
				{Base: models.Base{ID: 100}, ProductID: pkg.Ptr(uint(1))},
			},
		}
		mockPORepo.On("UpdatePurchaseOrder", ctx, uint(1), req).Return(updatedPO, nil).Once()

		spRepo.On("GetPOItemSellingPricesByPOItemIDs", ctx, []uint{100}).Return(nil, nil).Once()
		spRepo.On("GetLatestForProducts", ctx, []uint{1}, &invID, mock.Anything).Return(map[uint]*models.SellingPrice{
			1: {Base: models.Base{ID: 50}, ProductID: 1, Price: decimal.NewFromInt(20)},
		}, nil).Once()
		spRepo.On("CreatePOItemSellingPrice", ctx, mock.MatchedBy(func(p *models.POItemSellingPrice) bool {
			return p.PurchaseOrderItemID == 100 && p.SellingPriceID != nil && *p.SellingPriceID == 50
		})).Return(nil).Once()

		po, err := service.UpdatePurchaseOrder(ctx, 1, req)
		require.NoError(t, err)
		require.NotNil(t, po)
	})

	t.Run("update succeeds even if createPOItemSellingPrices fails (logged, not propagated)", func(t *testing.T) {
		ctx := context.Background()
		mockPORepo := repositorymocks.NewPurchaseOrderRepository(t)
		spRepo := repositorymocks.NewSellingPriceRepository(t)
		service := NewPurchaseOrderService(mockPORepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, spRepo)

		invID := uint(10)
		req := dto.UpdatePurchaseOrderRequest{
			InventoryID: &invID,
			Items: []dto.UpdatePurchaseOrderItemRequest{
				{ProductID: pkg.Ptr(uint(1)), SupplierID: pkg.Ptr(uint(2)), Quantity: decimal.NewFromInt(5)},
			},
		}
		updatedPO := &models.PurchaseOrder{
			Base:        models.Base{ID: 1},
			InventoryID: &invID,
			Items: []*models.PurchaseOrderItem{
				{Base: models.Base{ID: 100}, ProductID: pkg.Ptr(uint(1))},
			},
		}
		mockPORepo.On("UpdatePurchaseOrder", ctx, uint(1), req).Return(updatedPO, nil).Once()
		// Existing-pisp lookup fails — UpdatePurchaseOrder must still succeed.
		spRepo.On("GetPOItemSellingPricesByPOItemIDs", ctx, []uint{100}).Return(nil, errors.New("db down")).Once()

		po, err := service.UpdatePurchaseOrder(ctx, 1, req)
		require.NoError(t, err)
		require.NotNil(t, po)
	})
}
