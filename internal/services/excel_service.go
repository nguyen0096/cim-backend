package services

import (
	"cim-backend/internal/config"
	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/internal/repository/excel"
	"cim-backend/internal/repository/googlesheets"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
	"cim-backend/pkg/log"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// FontConfig represents font styling configuration for Excel cells
type FontConfig struct {
	Family    string  // Font family (e.g., "Arial", "Calibri", "Times New Roman")
	Size      float64 // Font size
	Bold      bool    // Bold text
	Italic    bool    // Italic text
	Underline string  // Underline style ("single", "double", "none")
	Color     string  // Font color (hex format like "FF0000" for red)
}

// CellStyle represents complete cell styling including font
type CellStyle struct {
	Font      FontConfig
	Fill      string // Background color (hex format)
	Alignment string // Text alignment ("left", "center", "right")
	Border    bool   // Whether to add borders
}

// DefaultFontConfigs provides predefined font configurations
var (
	DefaultHeaderFont = FontConfig{
		Family:    "Times New Roman",
		Size:      12,
		Bold:      true,
		Italic:    false,
		Underline: "none",
		Color:     "000000", // Black
	}

	DefaultDataFont = FontConfig{
		Family:    "Times New Roman",
		Size:      11,
		Bold:      false,
		Italic:    false,
		Underline: "none",
		Color:     "000000", // Black
	}

	DefaultHeaderStyle = CellStyle{
		Font:      DefaultHeaderFont,
		Fill:      "E7E6E6", // Light gray background
		Alignment: "center",
		Border:    true,
	}

	DefaultDataStyle = CellStyle{
		Font:      DefaultDataFont,
		Fill:      "FFFFFF", // White background
		Alignment: "left",
		Border:    true,
	}
)

// Predefined style configurations for common use cases
var (
	// Professional style with blue headers
	ProfessionalHeaderStyle = CellStyle{
		Font: FontConfig{
			Family:    "Times New Roman",
			Size:      12,
			Bold:      true,
			Italic:    false,
			Underline: "none",
			Color:     "FFFFFF", // White text
		},
		Fill:      "366092", // Dark blue background
		Alignment: "center",
		Border:    true,
	}

	ProfessionalDataStyle = CellStyle{
		Font:      DefaultDataFont,
		Fill:      "FFFFFF", // White background
		Alignment: "left",
		Border:    true,
	}

	// Corporate style with green headers
	CorporateHeaderStyle = CellStyle{
		Font: FontConfig{
			Family:    "Arial",
			Size:      11,
			Bold:      true,
			Italic:    false,
			Underline: "none",
			Color:     "FFFFFF", // White text
		},
		Fill:      "70AD47", // Green background
		Alignment: "center",
		Border:    true,
	}

	CorporateDataStyle = CellStyle{
		Font: FontConfig{
			Family:    "Arial",
			Size:      10,
			Bold:      false,
			Italic:    false,
			Underline: "none",
			Color:     "000000", // Black text
		},
		Fill:      "FFFFFF", // White background
		Alignment: "left",
		Border:    true,
	}

	// Minimal style with no borders
	MinimalHeaderStyle = CellStyle{
		Font: FontConfig{
			Family:    "Times New Roman",
			Size:      11,
			Bold:      true,
			Italic:    false,
			Underline: "none",
			Color:     "000000", // Black text
		},
		Fill:      "F2F2F2", // Light gray background
		Alignment: "left",
		Border:    false,
	}

	MinimalDataStyle = CellStyle{
		Font:      DefaultDataFont,
		Fill:      "FFFFFF", // White background
		Alignment: "left",
		Border:    false,
	}
)

//go:generate mockery --name=ExcelService --structname=ExcelService --output=../mocks/servicemocks --outpkg=servicemocks
type ExcelService interface {
	// Revenue/Expense Excel operations
	InitializeRevenueExpenseFile(ctx context.Context, filePath string) error
	AddExpenses(ctx context.Context, sheetName string, expensesData []map[string]interface{}, cellColors []string) error
	GetRevenueExpenseSchema(ctx context.Context) *models.FileMetadata
	VerifyFileAndSheet(ctx context.Context, filePath string, sheetName string) error
	FinalizeRevenueExpense(ctx context.Context, date time.Time, today time.Time) (*time.Time, error)
	// Revenue/Expense Google Sheets operations
	InitializeRevenueExpenseGoogleSheets(ctx context.Context, spreadsheetID string) error
	AddExpensesToGoogleSheets(ctx context.Context, sheetName string, expensesData []map[string]interface{}, cellColors []string) error
	VerifyGoogleSheetAndSheet(ctx context.Context, spreadsheetID string, sheetName string) error
	GetHeaderAndColorFromProductType(productType string) (header string, color string)
}

type excelService struct {
	productRepo                    repository.ProductRepository
	inventoryRepo                  repository.InventoryRepository
	paymentReceiptFormRepo         repository.PaymentReceiptFormRepository
	revenueExpenseExcelRepo        excel.RevenueExpenseExcelRepository
	revenueExpenseGoogleSheetsRepo googlesheets.RevenueExpenseGoogleSheetsRepository
	settingsService                SettingsService
	googleServiceAccount           string
}

func NewExcelService(productRepo repository.ProductRepository, inventoryRepo repository.InventoryRepository, paymentReceiptFormRepo repository.PaymentReceiptFormRepository, settingsService SettingsService) ExcelService {
	return &excelService{
		productRepo:                    productRepo,
		inventoryRepo:                  inventoryRepo,
		paymentReceiptFormRepo:         paymentReceiptFormRepo,
		revenueExpenseExcelRepo:        excel.NewRevenueExpenseExcelRepository(),
		revenueExpenseGoogleSheetsRepo: googlesheets.NewRevenueExpenseGoogleSheetsRepository(),
		settingsService:                settingsService,
		googleServiceAccount:           os.Getenv("GOOGLE_SERVICE_ACCOUNT"),
	}
}

// InitializeRevenueExpenseFile initializes the revenue/expense Excel file
func (s *excelService) InitializeRevenueExpenseFile(ctx context.Context, filePath string) error {
	return s.revenueExpenseExcelRepo.InitializeWithFile(ctx, filePath)
}

// AddExpenses adds multiple expense entries to the Excel file
func (s *excelService) AddExpenses(ctx context.Context, sheetName string, expensesData []map[string]interface{}, cellColors []string) error {
	return s.revenueExpenseExcelRepo.AddExpenses(ctx, sheetName, expensesData, cellColors)
}

// GetRevenueExpenseSchema returns the Excel file schema
func (s *excelService) GetRevenueExpenseSchema(ctx context.Context) *models.FileMetadata {
	return s.revenueExpenseExcelRepo.GetSchema(ctx)
}

// VerifyFileAndSheet verifies that the filepath and sheetname exist
func (s *excelService) VerifyFileAndSheet(ctx context.Context, filePath string, sheetName string) error {
	return s.revenueExpenseExcelRepo.VerifyFileAndSheet(ctx, filePath, sheetName)
}

// GetHeaderAndColorFromProductType maps product type to expense category and color
func (s *excelService) GetHeaderAndColorFromProductType(productType string) (header string, color string) {
	switch strings.ToLower(productType) {
	case "nước":
		header = pkg.RevenueExpenseColumnWater
		color = pkg.RevenueExpenseColumnWaterColor
	default:
		header = pkg.RevenueExpenseColumnSnackAndRice
		color = pkg.RevenueExpenseColumnSnackAndRiceColor
	}
	return
}

// createExpenseDataFromForms creates expense data and cell colors from payment receipt forms
func (s *excelService) createExpenseDataFromForms(paymentReceiptForms []models.PaymentReceiptForm) ([]map[string]interface{}, []string) {
	if len(paymentReceiptForms) == 0 {
		log.WithFields(logrus.Fields{
			"operation": "createExpenseDataFromForms",
		}).Warn("No payment receipt forms provided")
		return nil, nil
	}

	expensesData := make([]map[string]interface{}, 0, len(paymentReceiptForms))
	cellColors := make([]string, 0, len(paymentReceiptForms))

	for _, paymentReceiptForm := range paymentReceiptForms {
		// Validate required fields
		if paymentReceiptForm.PurchaseOrder == nil {
			log.WithFields(logrus.Fields{
				"operation":               "createExpenseDataFromForms",
				"payment_receipt_form_id": paymentReceiptForm.ID,
			}).Warn("Skipping payment receipt form: purchase order is nil")
			continue
		}

		if len(paymentReceiptForm.PurchaseOrder.Items) == 0 {
			log.WithFields(logrus.Fields{
				"operation":               "createExpenseDataFromForms",
				"payment_receipt_form_id": paymentReceiptForm.ID,
			}).Warn("Skipping payment receipt form: no purchase order items")
			continue
		}

		if paymentReceiptForm.PurchaseOrder.Items[0].Product == nil {
			log.WithFields(logrus.Fields{
				"operation":               "createExpenseDataFromForms",
				"payment_receipt_form_id": paymentReceiptForm.ID,
			}).Warn("Skipping payment receipt form: product is nil")
			continue
		}

		if paymentReceiptForm.FormNumber == nil {
			log.WithFields(logrus.Fields{
				"operation":               "createExpenseDataFromForms",
				"payment_receipt_form_id": paymentReceiptForm.ID,
			}).Warn("Skipping payment receipt form: form number is nil")
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
		header, color := s.GetHeaderAndColorFromProductType(productType)
		expenseData[header] = paymentReceiptForm.TotalAmount

		formNumberParts := strings.Split(*paymentReceiptForm.FormNumber, "-")
		if len(formNumberParts) < 3 {
			log.WithFields(logrus.Fields{
				"operation":               "createExpenseDataFromForms",
				"payment_receipt_form_id": paymentReceiptForm.ID,
				"form_number":             paymentReceiptForm.FormNumber,
			}).Warn("Skipping payment receipt form: invalid form number format")
			continue
		}

		ordinalNumber, err := strconv.Atoi(formNumberParts[2])
		if err != nil {
			log.WithFields(logrus.Fields{
				"operation":               "createExpenseDataFromForms",
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

// FinalizeRevenueExpense adds a new date row to the revenue expense file/sheet and writes payment receipt forms
func (s *excelService) FinalizeRevenueExpense(ctx context.Context, date time.Time, today time.Time) (*time.Time, error) {
	// Get settings to determine file type and sheet name
	settings, err := s.settingsService.GetSetting(ctx, config.RevenueExpenseExcelSettingsKey)
	if err != nil {
		return nil, pkg.ErrFailedToGetRevenueExpenseSettings(ctx, err)
	}

	if settings == nil {
		return nil, pkg.ErrRevenueExpenseSettingsNotConfigured(ctx)
	}

	var settingsValue map[string]interface{}
	if err := json.Unmarshal([]byte(settings.Value), &settingsValue); err != nil {
		return nil, pkg.ErrFailedToParseRevenueExpenseSettings(ctx, err)
	}

	filePath, ok := settingsValue["filePath"].(string)
	if !ok || filePath == "" {
		return nil, pkg.ErrFilePathNotFoundInSettings(ctx)
	}

	sheetName, ok := settingsValue["sheetName"].(string)
	if !ok || sheetName == "" {
		return nil, pkg.ErrSheetNameNotFoundInSettings(ctx)
	}

	// Query payment receipt forms from lastFinalizedDate to today
	// Only get approved forms for finalization
	lastFinalizedDate := date.Truncate(24 * time.Hour)
	req := &dto.PaymentReceiptFormListRequest{
		ListParams: models.ListParams{
			Page:  1,
			Limit: 1000, // Large limit to get all forms for the day
			Sort:  "form_number",
			Order: "asc",
		},
		FinalizedDate: lastFinalizedDate,
		Statuses:      []models.PaymentReceiptFormStatus{models.PaymentReceiptFormStatusApproved},
	}
	req.ValidateAndSetDefaults()

	forms, _, err := s.paymentReceiptFormRepo.List(ctx, req, "PurchaseOrder.Items", "PurchaseOrder.Items.Supplier", "PurchaseOrder.Items.Product")
	if err != nil {
		return nil, pkg.ErrFailedToQueryPaymentReceiptForms(ctx, lastFinalizedDate.Format("2006-01-02"), err)
	}

	// Detect if filePath is a Google Sheets URL or local file path
	isGoogleSheets := strings.Contains(filePath, "docs.google.com/spreadsheets")

	if isGoogleSheets {
		// Handle Google Sheets
		spreadsheetID, err := pkg.ExtractSpreadsheetID(filePath)
		if err != nil {
			return nil, pkg.ErrInvalidGoogleSheetsURL(ctx, err)
		}

		// Initialize repository
		if s.googleServiceAccount == "" {
			return nil, pkg.ErrServiceAccountNotConfigured(ctx)
		}
		if err := s.revenueExpenseGoogleSheetsRepo.InitializeWithSpreadsheet(ctx, s.googleServiceAccount, spreadsheetID, sheetName); err != nil {
			return nil, pkg.ErrFailedToInitializeGoogleSheetsRepo(ctx, err)
		}

		// Add new date row, the date when user click finalize button, not the last finalized date
		if err := s.revenueExpenseGoogleSheetsRepo.AddNewDateRow(ctx, sheetName, today); err != nil {
			return nil, pkg.ErrFailedToAddNewDateRowGoogleSheets(ctx, err)
		}

		expensesData, cellColors := s.createExpenseDataFromForms(forms)
		if len(expensesData) > 0 {
			if err := s.AddExpensesToGoogleSheets(ctx, sheetName, expensesData, cellColors); err != nil {
				return nil, pkg.ErrFailedToAddExpensesToGoogleSheets(ctx, err)
			}
		}
	} else {
		// Handle local file
		// Initialize repository
		if err := s.revenueExpenseExcelRepo.InitializeWithFile(ctx, filePath, sheetName); err != nil {
			return nil, pkg.ErrFailedToInitializeExcelRepo(ctx, err)
		}

		// Add new date row, the date when user click finalize button, not the last finalized date
		if err := s.revenueExpenseExcelRepo.AddNewDateRow(ctx, sheetName, today); err != nil {
			return nil, pkg.ErrFailedToAddNewDateRowExcel(ctx, err)
		}

		expensesData, cellColors := s.createExpenseDataFromForms(forms)
		if len(expensesData) > 0 {
			if err := s.AddExpenses(ctx, sheetName, expensesData, cellColors); err != nil {
				return nil, pkg.ErrFailedToAddExpensesToExcel(ctx, err)
			}
		}
	}

	nextDay := today.AddDate(0, 0, 1)

	return &nextDay, nil
}

// InitializeRevenueExpenseGoogleSheets initializes the Google Sheets repository for revenue/expense tracking
func (s *excelService) InitializeRevenueExpenseGoogleSheets(ctx context.Context, spreadsheetID string) error {
	if s.googleServiceAccount == "" {
		return fmt.Errorf("service account file path not configured")
	}
	return s.revenueExpenseGoogleSheetsRepo.InitializeWithSpreadsheet(ctx, s.googleServiceAccount, spreadsheetID)
}

// AddExpensesToGoogleSheets adds expense entries to the Google Sheets
func (s *excelService) AddExpensesToGoogleSheets(ctx context.Context, sheetName string, expensesData []map[string]interface{}, cellColors []string) error {
	return s.revenueExpenseGoogleSheetsRepo.AddExpenses(ctx, sheetName, expensesData, cellColors)
}

// VerifyGoogleSheetAndSheet verifies that the spreadsheet ID and sheet name exist
func (s *excelService) VerifyGoogleSheetAndSheet(ctx context.Context, spreadsheetID string, sheetName string) error {
	if s.googleServiceAccount == "" {
		return fmt.Errorf("service account file path not configured")
	}
	return s.revenueExpenseGoogleSheetsRepo.VerifySpreadsheetAndSheet(ctx, s.googleServiceAccount, spreadsheetID, sheetName)
}
