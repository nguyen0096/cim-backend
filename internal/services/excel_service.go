package services

import (
	"cim-backend/internal/config"
	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/internal/repository/excel"
	"cim-backend/internal/repository/googlesheets"
	"cim-backend/pkg"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
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
	FinalizeRevenueExpense(ctx context.Context, date time.Time) error
	// Revenue/Expense Google Sheets operations
	InitializeRevenueExpenseGoogleSheets(ctx context.Context, spreadsheetID string) error
	AddExpensesToGoogleSheets(ctx context.Context, sheetName string, expensesData []map[string]interface{}, cellColors []string) error
	VerifyGoogleSheetAndSheet(ctx context.Context, spreadsheetID string, sheetName string) error
}

type excelService struct {
	productRepo                    repository.ProductRepository
	inventoryRepo                  repository.InventoryRepository
	revenueExpenseExcelRepo        excel.RevenueExpenseExcelRepository
	revenueExpenseGoogleSheetsRepo googlesheets.RevenueExpenseGoogleSheetsRepository
	settingsService                SettingsService
	googleServiceAccount           string
}

func NewExcelService(productRepo repository.ProductRepository, inventoryRepo repository.InventoryRepository, settingsService SettingsService) ExcelService {
	return &excelService{
		productRepo:                    productRepo,
		inventoryRepo:                  inventoryRepo,
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

// FinalizeRevenueExpense adds a new date row to the revenue expense file/sheet
func (s *excelService) FinalizeRevenueExpense(ctx context.Context, date time.Time) error {
	// Get settings to determine file type and sheet name
	settings, err := s.settingsService.GetSetting(ctx, config.RevenueExpenseExcelSettingsKey)
	if err != nil {
		return fmt.Errorf("failed to get revenue expense settings: %w", err)
	}

	if settings == nil {
		return fmt.Errorf("revenue expense settings not configured")
	}

	var settingsValue map[string]interface{}
	if err := json.Unmarshal([]byte(settings.Value), &settingsValue); err != nil {
		return fmt.Errorf("failed to parse revenue expense settings: %w", err)
	}

	filePath, ok := settingsValue["filePath"].(string)
	if !ok || filePath == "" {
		return fmt.Errorf("filePath not found in revenue expense settings")
	}

	sheetName, ok := settingsValue["sheetName"].(string)
	if !ok || sheetName == "" {
		return fmt.Errorf("sheetName not found in revenue expense settings")
	}

	// Calculate next day
	nextDay := date.AddDate(0, 0, 1)

	// Detect if filePath is a Google Sheets URL or local file path
	isGoogleSheets := strings.Contains(filePath, "docs.google.com/spreadsheets")

	if isGoogleSheets {
		// Handle Google Sheets
		spreadsheetID, err := pkg.ExtractSpreadsheetID(filePath)
		if err != nil {
			return fmt.Errorf("invalid Google Sheets URL: %w", err)
		}

		// Initialize repository
		if s.googleServiceAccount == "" {
			return fmt.Errorf("service account file path not configured")
		}
		if err := s.revenueExpenseGoogleSheetsRepo.InitializeWithSpreadsheet(ctx, s.googleServiceAccount, spreadsheetID, sheetName); err != nil {
			return fmt.Errorf("failed to initialize Google Sheets repository: %w", err)
		}

		// Add new date row
		if err := s.revenueExpenseGoogleSheetsRepo.AddNewDateRow(ctx, sheetName, nextDay); err != nil {
			return fmt.Errorf("failed to add new date row to Google Sheets: %w", err)
		}
	} else {
		// Handle local file
		// Initialize repository
		if err := s.revenueExpenseExcelRepo.InitializeWithFile(ctx, filePath, sheetName); err != nil {
			return fmt.Errorf("failed to initialize Excel repository: %w", err)
		}

		// Add new date row
		if err := s.revenueExpenseExcelRepo.AddNewDateRow(ctx, sheetName, nextDay); err != nil {
			return fmt.Errorf("failed to add new date row to Excel: %w", err)
		}
	}

	return nil
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
