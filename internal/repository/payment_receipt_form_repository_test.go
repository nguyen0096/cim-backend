package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"cim-backend/internal/models"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: db,
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Error),
	})
	require.NoError(t, err)

	return gormDB, mock
}

func TestPaymentReceiptFormRepository_Create(t *testing.T) {
	gormDB, mock := setupTestDB(t)
	repo := NewPaymentReceiptFormRepository(gormDB)
	ctx := pkg.WithUserEmail(context.Background(), "test@cim.local")

	form := &models.PaymentReceiptForm{
		FullName:    "John Doe",
		Department:  "IT",
		TotalAmount: 100.0,
		Status:      models.PaymentReceiptFormStatusPending,
	}

	mock.ExpectBegin()
	// GORM inserts all fields. Use lenient regex.
	mock.ExpectQuery(`INSERT INTO .*payment_receipt_forms.*`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()

	err := repo.Create(ctx, form)
	assert.NoError(t, err)
	assert.Equal(t, uint(1), form.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentReceiptFormRepository_GetByID(t *testing.T) {
	gormDB, mock := setupTestDB(t)
	repo := NewPaymentReceiptFormRepository(gormDB)
	ctx := pkg.WithUserEmail(context.Background(), "test@cim.local")
	formID := uint(123)

	t.Run("success", func(t *testing.T) {
		// GORM First adds LIMIT 1 (argument $2)
		mock.ExpectQuery(`SELECT \* FROM .*payment_receipt_forms.* WHERE .*id.* = \$1.*LIMIT \$2`).
			WithArgs(formID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "full_name"}).AddRow(formID, "John Doe"))

		form, err := repo.GetByID(ctx, formID)
		assert.NoError(t, err)
		require.NotNil(t, form)
		assert.Equal(t, formID, form.ID)
		assert.Equal(t, "John Doe", form.FullName)
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectQuery(`SELECT \* FROM .*payment_receipt_forms.* WHERE .*id.* = \$1.*LIMIT \$2`).
			WithArgs(formID, 1).
			WillReturnError(gorm.ErrRecordNotFound)

		form, err := repo.GetByID(ctx, formID)
		assert.Error(t, err)
		assert.Nil(t, form)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestPaymentReceiptFormRepository_GetByIDFull(t *testing.T) {
	gormDB, mock := setupTestDB(t)
	repo := NewPaymentReceiptFormRepository(gormDB)
	ctx := pkg.WithUserEmail(context.Background(), "test@cim.local")
	formID := uint(123)
	poID := uint(456)
	inventoryID := uint(1)

	t.Run("success", func(t *testing.T) {
		// 1. Main query
		mock.ExpectQuery(`SELECT \* FROM .*payment_receipt_forms.* WHERE .*id.* = \$1.*LIMIT \$2`).
			WithArgs(formID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "purchase_order_id"}).AddRow(formID, poID))

		// 2. Preload PurchaseOrder
		mock.ExpectQuery(`SELECT \* FROM .*purchase_orders.* WHERE .*id.* (=|IN) .*`).
			WithArgs(poID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "inventory_id"}).AddRow(poID, inventoryID))

		// 3. Preload PurchaseOrder.Inventory
		mock.ExpectQuery(`SELECT \* FROM .*inventories.* WHERE .*id.* (=|IN) .*`).
			WithArgs(inventoryID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(inventoryID, "Main Inventory"))

		// 4. Preload PurchaseOrder.Items
		mock.ExpectQuery(`SELECT \* FROM .*purchase_order_items.* WHERE .*purchase_order_id.* (=|IN) .*`).
			WithArgs(poID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "purchase_order_id", "product_id", "supplier_id"}).
				AddRow(1, poID, 10, 20))

		// 5. Preload PurchaseOrder.Items.Product
		mock.ExpectQuery(`SELECT \* FROM .*products.* WHERE .*id.* (=|IN) .*`).
			WithArgs(10).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(10, "Product A"))

		// 6. Preload PurchaseOrder.Items.Supplier
		mock.ExpectQuery(`SELECT \* FROM .*suppliers.* WHERE .*id.* (=|IN) .*`).
			WithArgs(20).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(20, "Supplier X"))

		form, err := repo.GetByIDFull(ctx, formID)
		assert.NoError(t, err)
		require.NotNil(t, form)
		assert.Equal(t, formID, form.ID)
		require.NotNil(t, form.PurchaseOrder)
		assert.Equal(t, poID, form.PurchaseOrder.ID)
		require.NotNil(t, form.PurchaseOrder.Inventory)
		assert.Equal(t, "Main Inventory", form.PurchaseOrder.Inventory.Name)
		assert.Len(t, form.PurchaseOrder.Items, 1)
		assert.Equal(t, "Product A", form.PurchaseOrder.Items[0].Product.Name)
		assert.Equal(t, "Supplier X", form.PurchaseOrder.Items[0].Supplier.Name)
	})
}

func TestPaymentReceiptFormRepository_Update(t *testing.T) {
	gormDB, mock := setupTestDB(t)
	repo := NewPaymentReceiptFormRepository(gormDB)
	ctx := pkg.WithUserEmail(context.Background(), "test@cim.local")

	form := &models.PaymentReceiptForm{
		Base:     models.Base{ID: 1},
		FullName: "Updated Name",
	}

	mock.ExpectBegin()
	// GORM Update uses many arguments. Use lenient regex and AnyArg.
	mock.ExpectExec(`UPDATE .*payment_receipt_forms.* SET .* WHERE .*id.* = .*`).
		WithArgs(
			sqlmock.AnyArg(), // updated_at
			sqlmock.AnyArg(), // form_number
			sqlmock.AnyArg(), // date
			"Updated Name",   // full_name
			sqlmock.AnyArg(), // department
			sqlmock.AnyArg(), // details
			sqlmock.AnyArg(), // total_amount
			sqlmock.AnyArg(), // status
			1,                // id in WHERE
			1,                // id in second WHERE (GORM sometimes duplicates)
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := repo.Update(ctx, form)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentReceiptFormRepository_List(t *testing.T) {
	gormDB, mock := setupTestDB(t)
	repo := NewPaymentReceiptFormRepository(gormDB)
	ctx := pkg.WithUserEmail(context.Background(), "test@cim.local")

	t.Run("list with filters", func(t *testing.T) {
		req := &dto.PaymentReceiptFormListRequest{
			InventoryID: 1,
			Statuses:    []models.PaymentReceiptFormStatus{models.PaymentReceiptFormStatusPending},
			ListParams: models.ListParams{
				Limit:  10,
				Page:   1,
				Search: "John",
			},
		}

		// Mock Count query
		// Uses JOIN purchase_orders
		mock.ExpectQuery(`SELECT count\(\*\) FROM .*payment_receipt_forms.* JOIN .*purchase_orders.*`).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

		// Mock Find query
		mock.ExpectQuery(`SELECT .*payment_receipt_forms.* FROM .*payment_receipt_forms.* JOIN .*purchase_orders.*`).
			WillReturnRows(sqlmock.NewRows([]string{"id", "full_name"}).AddRow(1, "John Doe"))

		forms, total, err := repo.List(ctx, req)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), total)
		require.NotEmpty(t, forms)
		assert.Equal(t, "John Doe", forms[0].FullName)
	})
}

func TestPaymentReceiptFormRepository_GenerateNextFormNumber(t *testing.T) {
	gormDB, mock := setupTestDB(t)
	repo := NewPaymentReceiptFormRepository(gormDB)
	ctx := pkg.WithUserEmail(context.Background(), "test@cim.local")
	date := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	inventoryID := uint(1)
	dateStr := "20240115"
	pattern := fmt.Sprintf("%s-%d-%%", dateStr, inventoryID)

	t.Run("should generate first form number if none exist", func(t *testing.T) {
		// Mock MAX query
		mock.ExpectQuery(`SELECT COALESCE\(MAX\(CAST\(SPLIT_PART\(form_number, '-', 3\) AS INTEGER\)\), 0\) FROM payment_receipt_forms`).
			WithArgs(pattern).
			WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(0))

		// Mock existence check
		expectedFormNumber := fmt.Sprintf("%s-%d-1", dateStr, inventoryID)
		mock.ExpectQuery(`SELECT count\(\*\) FROM "payment_receipt_forms" WHERE form_number =`).
			WithArgs(expectedFormNumber).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

		formNumber, err := repo.GenerateNextFormNumber(ctx, date, inventoryID)
		assert.NoError(t, err)
		assert.Equal(t, expectedFormNumber, formNumber)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("should generate next increment", func(t *testing.T) {
		// Mock MAX query returning 5
		mock.ExpectQuery(`SELECT COALESCE\(MAX\(CAST\(SPLIT_PART\(form_number, '-', 3\) AS INTEGER\)\), 0\) FROM payment_receipt_forms`).
			WithArgs(pattern).
			WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(5))

		// Mock existence check
		expectedFormNumber := fmt.Sprintf("%s-%d-6", dateStr, inventoryID)
		mock.ExpectQuery(`SELECT count\(\*\) FROM "payment_receipt_forms" WHERE form_number =`).
			WithArgs(expectedFormNumber).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

		formNumber, err := repo.GenerateNextFormNumber(ctx, date, inventoryID)
		assert.NoError(t, err)
		assert.Equal(t, expectedFormNumber, formNumber)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("should retry on race condition (existence check)", func(t *testing.T) {
		// First attempt: MAX returns 5, but increment 6 already exists
		mock.ExpectQuery(`SELECT COALESCE\(MAX\(CAST\(SPLIT_PART\(form_number, '-', 3\) AS INTEGER\)\), 0\) FROM payment_receipt_forms`).
			WithArgs(pattern).
			WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(5))

		mock.ExpectQuery(`SELECT count\(\*\) FROM "payment_receipt_forms" WHERE form_number =`).
			WithArgs(fmt.Sprintf("%s-%d-6", dateStr, inventoryID)).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

		// Second attempt: MAX now returns 6, increment 7 is free
		mock.ExpectQuery(`SELECT COALESCE\(MAX\(CAST\(SPLIT_PART\(form_number, '-', 3\) AS INTEGER\)\), 0\) FROM payment_receipt_forms`).
			WithArgs(pattern).
			WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(6))

		mock.ExpectQuery(`SELECT count\(\*\) FROM "payment_receipt_forms" WHERE form_number =`).
			WithArgs(fmt.Sprintf("%s-%d-7", dateStr, inventoryID)).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

		formNumber, err := repo.GenerateNextFormNumber(ctx, date, inventoryID)
		assert.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("%s-%d-7", dateStr, inventoryID), formNumber)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("should fallback to COUNT if MAX query fails", func(t *testing.T) {
		// Mock MAX query failing
		mock.ExpectQuery(`SELECT COALESCE\(MAX\(CAST\(SPLIT_PART\(form_number, '-', 3\) AS INTEGER\)\), 0\) FROM payment_receipt_forms`).
			WithArgs(pattern).
			WillReturnError(fmt.Errorf("db error"))

		// Mock Fallback COUNT query
		// GORM generates: SELECT count(*) FROM "payment_receipt_forms" WHERE form_number LIKE $1
		mock.ExpectQuery(`SELECT count\(\*\) FROM "payment_receipt_forms" WHERE form_number (LIKE|like)`).
			WithArgs(pattern).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(10))

		// Mock existence check for increment 11
		expectedFormNumber := fmt.Sprintf("%s-%d-11", dateStr, inventoryID)
		mock.ExpectQuery(`SELECT count\(\*\) FROM "payment_receipt_forms" WHERE form_number =`).
			WithArgs(expectedFormNumber).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

		formNumber, err := repo.GenerateNextFormNumber(ctx, date, inventoryID)
		assert.NoError(t, err)
		assert.Equal(t, expectedFormNumber, formNumber)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
