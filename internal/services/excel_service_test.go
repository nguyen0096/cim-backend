package services_test

import (
	"cim-backend/internal/config"
	"cim-backend/internal/mocks/repositorymocks"
	"cim-backend/internal/mocks/servicemocks"
	"cim-backend/internal/models"
	"cim-backend/internal/services"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestExcelService(t *testing.T) {
	ctx := context.Background()

	setupMocks := func() (
		*repositorymocks.ProductRepository,
		*repositorymocks.InventoryRepository,
		*repositorymocks.PaymentReceiptFormRepository,
		*repositorymocks.RevenueExpenseExcelRepository,
		*repositorymocks.RevenueExpenseGoogleSheetsRepository,
		*servicemocks.SettingsService,
		services.ExcelService,
	) {
		productRepo := new(repositorymocks.ProductRepository)
		inventoryRepo := new(repositorymocks.InventoryRepository)
		paymentReceiptFormRepo := new(repositorymocks.PaymentReceiptFormRepository)
		excelRepo := new(repositorymocks.RevenueExpenseExcelRepository)
		googleRepo := new(repositorymocks.RevenueExpenseGoogleSheetsRepository)
		settingsService := new(servicemocks.SettingsService)

		os.Setenv("GOOGLE_SERVICE_ACCOUNT", "fake-account.json")
		service := services.NewExcelService(
			productRepo,
			inventoryRepo,
			paymentReceiptFormRepo,
			excelRepo,
			googleRepo,
			settingsService,
		)

		return productRepo, inventoryRepo, paymentReceiptFormRepo, excelRepo, googleRepo, settingsService, service
	}

	t.Run("InitializeRevenueExpenseFile", func(t *testing.T) {
		_, _, _, excelRepo, _, _, service := setupMocks()
		filePath := "test.xlsx"
		excelRepo.On("InitializeWithFile", ctx, filePath).Return(nil).Once()

		err := service.InitializeRevenueExpenseFile(ctx, filePath)
		assert.NoError(t, err)
		excelRepo.AssertExpectations(t)
	})

	t.Run("AddExpenses", func(t *testing.T) {
		_, _, _, excelRepo, _, _, service := setupMocks()
		sheetName := "Sheet1"
		expensesData := []map[string]interface{}{{"col1": "val1"}}
		cellColors := []string{"FFFFFF"}
		excelRepo.On("AddExpenses", ctx, sheetName, expensesData, cellColors).Return(nil).Once()

		err := service.AddExpenses(ctx, sheetName, expensesData, cellColors)
		assert.NoError(t, err)
		excelRepo.AssertExpectations(t)
	})

	t.Run("GetRevenueExpenseSchema", func(t *testing.T) {
		_, _, _, excelRepo, _, _, service := setupMocks()
		expectedSchema := &models.FileMetadata{}
		excelRepo.On("GetSchema", ctx).Return(expectedSchema).Once()

		schema := service.GetRevenueExpenseSchema(ctx)
		assert.Equal(t, expectedSchema, schema)
		excelRepo.AssertExpectations(t)
	})

	t.Run("VerifyFileAndSheet", func(t *testing.T) {
		_, _, _, excelRepo, _, _, service := setupMocks()
		filePath := "test.xlsx"
		sheetName := "Sheet1"
		excelRepo.On("VerifyFileAndSheet", ctx, filePath, sheetName).Return(nil).Once()

		err := service.VerifyFileAndSheet(ctx, filePath, sheetName)
		assert.NoError(t, err)
		excelRepo.AssertExpectations(t)
	})

	t.Run("GetHeaderAndColorFromProductType", func(t *testing.T) {
		_, _, _, _, _, _, service := setupMocks()

		header, color := service.GetHeaderAndColorFromProductType("nước")
		assert.Equal(t, pkg.RevenueExpenseColumnWater, header)
		assert.Equal(t, pkg.RevenueExpenseColumnWaterColor, color)

		header, color = service.GetHeaderAndColorFromProductType("snack")
		assert.Equal(t, pkg.RevenueExpenseColumnSnackAndRice, header)
		assert.Equal(t, pkg.RevenueExpenseColumnSnackAndRiceColor, color)
	})

	t.Run("FinalizeRevenueExpense - Local Excel", func(t *testing.T) {
		_, _, paymentReceiptFormRepo, excelRepo, _, settingsService, service := setupMocks()

		prefixDate := time.Now()
		dateInExcel := time.Now()

		settings := &models.Settings{
			Value: []byte(`{"filePath": "/path/to/test.xlsx", "sheetName": "Sheet1"}`),
		}
		settingsService.On("GetSetting", ctx, config.RevenueExpenseExcelSettingsKey).Return(settings, nil).Once()

		paymentReceiptFormRepo.On("List", ctx,
			mock.MatchedBy(func(req *dto.PaymentReceiptFormListRequest) bool {
				return req.FinalizedDate == prefixDate.Format("20060102")
			}),
			"PurchaseOrder.Items", "PurchaseOrder.Items.Supplier", "PurchaseOrder.Items.Product").
			Return([]models.PaymentReceiptForm{}, int64(0), nil).Once()

		excelRepo.On("InitializeWithFile", ctx, "/path/to/test.xlsx", "Sheet1").Return(nil).Once()
		excelRepo.On("Close").Return(nil).Once()

		err := service.FinalizeRevenueExpense(ctx, prefixDate, dateInExcel)
		assert.NoError(t, err)

		settingsService.AssertExpectations(t)
		paymentReceiptFormRepo.AssertExpectations(t)
		excelRepo.AssertExpectations(t)
	})

	t.Run("FinalizeRevenueExpense - Google Sheets", func(t *testing.T) {
		_, _, paymentReceiptFormRepo, _, googleRepo, settingsService, service := setupMocks()

		prefixDate := pkg.GetTodayDate()
		dateInExcel := pkg.GetTodayDate()

		settings := &models.Settings{
			Value: []byte(`{"filePath": "https://docs.google.com/spreadsheets/d/abc123/edit", "sheetName": "Sheet1"}`),
		}
		settingsService.On("GetSetting", ctx, config.RevenueExpenseExcelSettingsKey).Return(settings, nil).Once()

		paymentReceiptFormRepo.On("List", ctx,
			mock.MatchedBy(func(req *dto.PaymentReceiptFormListRequest) bool {
				return req.FinalizedDate == prefixDate.Format("20060102")
			}),
			"PurchaseOrder.Items", "PurchaseOrder.Items.Supplier", "PurchaseOrder.Items.Product").
			Return([]models.PaymentReceiptForm{}, int64(0), nil).Once()

		googleRepo.On("InitializeWithSpreadsheet", ctx, "fake-account.json", "abc123", "Sheet1").Return(nil).Once()
		googleRepo.On("AddNewDateRow", ctx, "Sheet1", dateInExcel).Return(nil).Once()

		err := service.FinalizeRevenueExpense(ctx, prefixDate, dateInExcel)
		assert.NoError(t, err)

		settingsService.AssertExpectations(t)
		paymentReceiptFormRepo.AssertExpectations(t)
		googleRepo.AssertExpectations(t)
	})

	t.Run("FinalizeRevenueExpense - With Data", func(t *testing.T) {
		_, _, paymentReceiptFormRepo, excelRepo, _, settingsService, service := setupMocks()

		prefixDate := time.Now()
		dateInExcel := time.Now()

		settings := &models.Settings{
			Value: []byte(`{"filePath": "/path/to/test.xlsx", "sheetName": "Sheet1"}`),
		}
		settingsService.On("GetSetting", ctx, config.RevenueExpenseExcelSettingsKey).Return(settings, nil).Once()

		formNumber := "RC-2026-001"
		forms := []models.PaymentReceiptForm{
			{
				Base:        models.Base{ID: 1},
				FormNumber:  &formNumber,
				TotalAmount: 1000,
				PurchaseOrder: &models.PurchaseOrder{
					Items: []*models.PurchaseOrderItem{
						{
							Product: &models.Product{
								ProductType: "snack",
							},
							Supplier: &models.Supplier{
								Name: "Supplier A",
							},
						},
					},
				},
			},
		}
		paymentReceiptFormRepo.On("List", ctx, mock.Anything, "PurchaseOrder.Items", "PurchaseOrder.Items.Supplier", "PurchaseOrder.Items.Product").
			Return(forms, int64(1), nil).Once()

		excelRepo.On("InitializeWithFile", ctx, "/path/to/test.xlsx", "Sheet1").Return(nil).Once()
		excelRepo.On("AddNewDateRow", ctx, "Sheet1", dateInExcel).Return(nil).Once()

		expectedExpensesData := []map[string]interface{}{
			{
				pkg.RevenueExpenseColumnName:          "Supplier A",
				pkg.RevenueExpenseColumnSnackAndRice:  1000.0,
				pkg.RevenueExpenseColumnOrdinalNumber: 1,
			},
		}
		expectedColors := []string{pkg.RevenueExpenseColumnSnackAndRiceColor}

		excelRepo.On("AddExpenses", ctx, "Sheet1", expectedExpensesData, expectedColors).Return(nil).Once()
		excelRepo.On("Close").Return(nil).Once()

		err := service.FinalizeRevenueExpense(ctx, prefixDate, dateInExcel)
		assert.NoError(t, err)

		settingsService.AssertExpectations(t)
		paymentReceiptFormRepo.AssertExpectations(t)
		excelRepo.AssertExpectations(t)
	})
}
