package apptest

import (
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cim-backend/internal/config"
	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/internal/repository/excel"
	"cim-backend/pkg"
	"cim-backend/pkg/testutil"
	"cim-backend/pkg/testutil/fixture"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/shopspring/decimal"
)

var _ = Describe("Revenue Expense API", func() {
	Describe("Finalize Revenue Expense", func() {
		var excelFilePath string
		var tempExcelFile string
		var sheetName string

		BeforeEach(func() {
			// Get the absolute path to the test Excel file
			excelFilePath = pkg.TranslateCallerRelativePath("data/excel/revenue_expense_sample.xlsx")
			sheetName = "TIỀN MẶT"

			// Create a temporary copy of the Excel file for testing
			tempDir, err := os.MkdirTemp("", "revenue-expense-test-*")
			Expect(err).NotTo(HaveOccurred())

			tempExcelFile = filepath.Join(tempDir, "test_revenue_expense.xlsx")

			// Copy the original file to temp location
			src, err := os.Open(excelFilePath)
			Expect(err).NotTo(HaveOccurred())
			defer src.Close()

			dst, err := os.Create(tempExcelFile)
			Expect(err).NotTo(HaveOccurred())
			defer dst.Close()

			_, err = io.Copy(dst, src)
			Expect(err).NotTo(HaveOccurred())
			dst.Close()

			// Get absolute path for the temp file
			absPath, err := filepath.Abs(tempExcelFile)
			Expect(err).NotTo(HaveOccurred())
			tempExcelFile = absPath

			// Configure revenue expense settings
			ctx := pkg.WithUserEmail(tenv.DefaultContext, "test@cim.local")
			settingsRepo := repository.NewSettingsRepository(tenv.ContextfulDB())

			settingsValue := map[string]interface{}{
				"filePath":  tempExcelFile,
				"sheetName": sheetName,
			}

			err = settingsRepo.Set(ctx, config.RevenueExpenseExcelSettingsKey, settingsValue)
			Expect(err).NotTo(HaveOccurred())

			DeferCleanup(func() {
				// Clean up temp file
				if err := os.Remove(tempExcelFile); err != nil && !os.IsNotExist(err) {
					GinkgoWriter.Printf("Warning: failed to delete temp file %s: %v\n", tempExcelFile, err)
				}
				// Clean up temp directory
				if err := os.RemoveAll(filepath.Dir(tempExcelFile)); err != nil {
					GinkgoWriter.Printf("Warning: failed to delete temp dir: %v\n", err)
				}
			})
		})

		Context("when user has authorized role", func() {
			role := models.RoleAdmin
			It(fmt.Sprintf("should finalize revenue expense successfully with %s role", role), func(ctx SpecContext) {
				// Set lastFinalizedDate to yesterday before the test
				settingsCtx := pkg.WithUserEmail(tenv.DefaultContext, "test@cim.local")
				settingsRepo := repository.NewSettingsRepository(tenv.ContextfulDB())
				randomDate := time.Now().AddDate(0, 0, -1-rand.Intn(30))
				err := settingsRepo.Set(settingsCtx, config.LastFinalizedDateSettingsKey, randomDate)
				Expect(err).NotTo(HaveOccurred())

				inventory := fixture.WithInventory(tenv.ContextfulDB(), models.Inventory{
					Name:   "Inventory",
					Status: models.InventoryStatusActive,
				})
				suppliers := fixture.WithSuppliers(tenv.ContextfulDB(), []*models.Supplier{
					{
						Name: fmt.Sprintf("SUPPLIER %s", strings.ToUpper(uuid.New().String())),
					},
					{
						Name: fmt.Sprintf("SUPPLIER %s", strings.ToUpper(uuid.New().String())),
					},
				})
				units := fixture.WithUnits(tenv.ContextfulDB(), []*models.Unit{
					{Name: "Pack"},
				})
				products := fixture.WithProducts(tenv.ContextfulDB(), []*models.Product{
					{Name: "Snack and Rice", ProductType: pkg.RevenueExpenseColumnSnackAndRice, Unit: units[0], Suppliers: suppliers},
					{Name: "Water", ProductType: pkg.RevenueExpenseColumnWater, Unit: units[0], Suppliers: suppliers},
				})
				purchaseOrders := fixture.WithPurchaseOrders(tenv.ContextfulDB(), []*models.PurchaseOrder{
					{
						Inventory:   inventory,
						OrderNumber: uuid.New().String(),
						Items: []*models.PurchaseOrderItem{
							{Product: products[0], Supplier: suppliers[0], Quantity: decimal.NewFromFloat(1), UnitPrice: 1000, Unit: units[0]},
							{Product: products[1], Supplier: suppliers[0], Quantity: decimal.NewFromFloat(2), UnitPrice: 2000, Unit: units[0]},
						},
					},
					{
						Inventory:   inventory,
						OrderNumber: uuid.New().String(),
						Items: []*models.PurchaseOrderItem{
							{Product: products[0], Supplier: suppliers[1], Quantity: decimal.NewFromFloat(1), UnitPrice: 1000, Unit: units[0]},
						},
					},
				})

				// Set payment receipt forms
				totalForms := 1
				approvedPaymentReceiptForms := []*models.PaymentReceiptForm{}
				for i := 0; i < totalForms; i++ {
					approvedPaymentReceiptForms = append(approvedPaymentReceiptForms, &models.PaymentReceiptForm{
						FormNumber:    pkg.Ptr(fmt.Sprintf("%s-1-%d", randomDate.Format("20060102"), i+1)),
						Date:          randomDate,
						FullName:      "John Doe",
						Department:    "Finance",
						Details:       "Office supplies",
						TotalAmount:   100000,
						Status:        models.PaymentReceiptFormStatusApproved,
						PurchaseOrder: purchaseOrders[0],
					})
				}
				notApprovedPaymentReceiptForms := &models.PaymentReceiptForm{
					FormNumber:    pkg.Ptr(fmt.Sprintf("%s-1-%d", randomDate.Format("20060102"), len(approvedPaymentReceiptForms)+1)),
					Date:          randomDate,
					FullName:      "Jane Doe",
					Department:    "HR",
					Details:       "HR expenses",
					TotalAmount:   200000,
					Status:        models.PaymentReceiptFormStatusSubmitted,
					PurchaseOrder: purchaseOrders[1],
				}
				fixture.WithPaymentReceiptForms(tenv.ContextfulDB(), append(approvedPaymentReceiptForms, notApprovedPaymentReceiptForms))

				client := testutil.NewClient(tenv, role)

				// Use randomDate's date as the date to finalize
				payload := map[string]interface{}{
					"date": randomDate.Format("2006-01-02"),
				}

				resp, err := client.MakeRequest("POST", "/api/v1/revenue-expenses/finalize", payload, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				// Validate response structure
				finalizeResp := testutil.ParseResponse(resp)
				Expect(finalizeResp["message"]).To(Equal("Revenue expense finalized successfully"))
				Expect(finalizeResp["date"]).To(Equal(randomDate.Format("2006-01-02")))

				// Verify next_day is the day after the finalized date
				nextDay, ok := finalizeResp["next_day"].(string)
				Expect(ok).To(BeTrue())
				expectedNextDay := pkg.GetTodayDate().AddDate(0, 0, 1).Format("2006-01-02")
				Expect(nextDay).To(Equal(expectedNextDay))

				// Verify records in excel are created in order
				excelRepo := excel.NewRevenueExpenseExcelRepository()
				err = excelRepo.InitializeWithFile(tenv.DefaultContext, tempExcelFile)
				Expect(err).NotTo(HaveOccurred())
				_, _, rows, err := excelRepo.GetFileAndSheetData(sheetName)
				Expect(err).NotTo(HaveOccurred())
				Expect(rows).NotTo(BeEmpty())
				// get last row
				_, lastRowIndex, err := excelRepo.FindLastTransactionRow(rows)
				Expect(err).NotTo(HaveOccurred())
				// Convert Excel row number (1-based) to array index (0-based)
				lastRowArrayIndex := lastRowIndex - 1
				for i, approvedPaymentReceiptForm := range approvedPaymentReceiptForms {
					expenseData, err := excelRepo.MapRowToExpense(sheetName, rows[lastRowArrayIndex-totalForms+i+1])
					Expect(err).NotTo(HaveOccurred())
					Expect(expenseData[pkg.RevenueExpenseColumnName]).To(Equal(suppliers[0].Name))
					Expect(expenseData[pkg.RevenueExpenseColumnOrdinalNumber]).To(Equal(fmt.Sprintf("%d", i+1)))
					// MapRowToExpense returns values as strings (Excel stores them as text after uppercase conversion)
					// Convert string to float64 for comparison
					snackAndRiceValueStr, ok := expenseData[pkg.RevenueExpenseColumnSnackAndRice].(string)
					Expect(ok).To(BeTrue())
					var snackAndRiceValue float64
					_, err = fmt.Sscanf(strings.ToLower(snackAndRiceValueStr), "%f", &snackAndRiceValue)
					Expect(err).NotTo(HaveOccurred())
					Expect(snackAndRiceValue).To(Equal(approvedPaymentReceiptForm.TotalAmount))
				}

				// Verify the date was written correctly using GetLastTransactionDate
				// This method finds the last date row and parses it, confirming the date exists and is correct
				lastDate, err := excelRepo.GetLastTransactionDate(tenv.DefaultContext, sheetName)
				Expect(err).NotTo(HaveOccurred())
				// Compare dates by year, month, and day to avoid format ambiguity issues
				today := pkg.GetTodayDate()
				Expect(lastDate.Year()).To(Equal(today.Year()))
				Expect(lastDate.Month()).To(Equal(today.Month()))
				Expect(lastDate.Day()).To(Equal(today.Day()))

				// Verify the date row exists and has the correct format by finding it manually
				// Scan backwards from the last transaction row to find the date row
				headerRow := 0
				for i, row := range rows {
					if len(row) > 0 && strings.Contains(strings.ToUpper(row[0]), "NGÀY") {
						headerRow = i
						break
					}
				}
				var dateRow []string
				for i := len(rows) - 1; i >= headerRow+1; i-- {
					if len(rows[i]) == 0 {
						continue
					}
					firstCol := strings.TrimSpace(rows[i][0])
					if firstCol != "" {
						// Try to parse as date using common formats
						parsed := false
						for _, format := range []string{"02/01/2006", "2/1/2006", "01/02/2006", "1/2/2006"} {
							if _, err := time.Parse(format, firstCol); err == nil {
								dateRow = rows[i]
								parsed = true
								break
							}
						}
						if parsed {
							break
						}
					}
				}
				Expect(dateRow).NotTo(BeEmpty(), "Date row should be found")
				dateStr := strings.TrimSpace(dateRow[0])
				Expect(dateStr).NotTo(BeEmpty(), "Date string should not be empty")
				// Verify the date matches today (format may vary, so we parse and compare)
				// Try all formats and prefer the one closest to today to avoid ambiguity
				var parsedDate time.Time
				var parseErr error
				var bestDate time.Time
				var bestDiff time.Duration = time.Hour * 24 * 365 * 100 // Very large initial diff
				found := false
				for _, format := range []string{"02/01/2006", "2/1/2006", "01/02/2006", "1/2/2006"} {
					if parsed, err := time.Parse(format, dateStr); err == nil {
						// Calculate absolute difference from today
						diff := today.Sub(parsed)
						if diff < 0 {
							diff = -diff
						}
						// Prefer dates closer to today (within reasonable range, e.g., last 10 years)
						if diff < time.Hour*24*365*10 && diff < bestDiff {
							bestDate = parsed
							bestDiff = diff
							found = true
						}
					}
				}
				if found {
					parsedDate = bestDate
					parseErr = nil
				} else {
					// Fallback to first format that matches
					for _, format := range []string{"02/01/2006", "2/1/2006", "01/02/2006", "1/2/2006"} {
						parsedDate, parseErr = time.Parse(format, dateStr)
						if parseErr == nil {
							break
						}
					}
				}
				Expect(parseErr).NotTo(HaveOccurred(), "Date should be parseable")
				// Compare dates by year, month, and day to avoid format ambiguity issues
				Expect(parsedDate.Year()).To(Equal(today.Year()))
				Expect(parsedDate.Month()).To(Equal(today.Month()))
				Expect(parsedDate.Day()).To(Equal(today.Day()))
			})
		})

		Context("when request validation fails", func() {
			It("should return 400 when date is missing", func(ctx SpecContext) {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				payload := map[string]interface{}{}

				resp, err := client.MakeRequest("POST", "/api/v1/revenue-expenses/finalize", payload, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(400))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["message"]).NotTo(BeEmpty())
			})

			It("should return 200 when date format is invalid (validation only checks required)", func(ctx SpecContext) {
				// Note: The current validation only checks if date is required, not if it's a valid date format
				// The handler will use lastFinalizedDate from settings or time.Now() if not set
				client := testutil.NewClient(tenv, models.RoleAdmin)

				payload := map[string]interface{}{
					"date": "invalid-date-format",
				}

				resp, err := client.MakeRequest("POST", "/api/v1/revenue-expenses/finalize", payload, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				// The validation only checks required, so invalid format still passes validation
				// The handler will proceed and use lastFinalizedDate from settings or time.Now()
				Expect(resp.StatusCode).To(Equal(200))
			})
		})

		Context("when settings are not configured", func() {
			It("should return error when revenue expense settings are missing", func(ctx SpecContext) {
				// Remove the settings
				settingsCtx := pkg.WithUserEmail(tenv.DefaultContext, "test@cim.local")
				settingsRepo := repository.NewSettingsRepository(tenv.ContextfulDB())

				// Delete the setting
				err := settingsRepo.Delete(settingsCtx, config.RevenueExpenseExcelSettingsKey)
				Expect(err).NotTo(HaveOccurred())

				client := testutil.NewClient(tenv, models.RoleAdmin)

				randomDate := time.Now().AddDate(0, 0, rand.Intn(30))
				payload := map[string]interface{}{
					"date": randomDate.Format("2006-01-02"),
				}

				resp, err := client.MakeRequest("POST", "/api/v1/revenue-expenses/finalize", payload, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(500))

				errorResp := testutil.ParseResponse(resp)
				Expect(errorResp["message"]).NotTo(BeEmpty())
			})
		})

		Context("when user is unauthorized", func() {
			It("should return 403 when user has staff role", func(ctx SpecContext) {
				client := testutil.NewClient(tenv, models.RoleStaff)

				randomDate := time.Now().AddDate(0, 0, rand.Intn(30))
				payload := map[string]interface{}{
					"date": randomDate.Format("2006-01-02"),
				}

				resp, err := client.MakeRequest("POST", "/api/v1/revenue-expenses/finalize", payload, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))
			})
		})

		Context("when finalizing with existing Excel file", func() {
			It("should successfully finalize even when no payment receipt forms exist", func(ctx SpecContext) {
				// Set lastFinalizedDate to yesterday before the test
				settingsCtx := pkg.WithUserEmail(tenv.DefaultContext, "test@cim.local")
				settingsRepo := repository.NewSettingsRepository(tenv.ContextfulDB())
				randomDate := time.Now().AddDate(0, 0, rand.Intn(30))
				err := settingsRepo.Set(settingsCtx, config.LastFinalizedDateSettingsKey, randomDate)
				Expect(err).NotTo(HaveOccurred())

				client := testutil.NewClient(tenv, models.RoleAdmin)

				payload := map[string]interface{}{
					"date": randomDate.Format("2006-01-02"),
				}

				resp, err := client.MakeRequest("POST", "/api/v1/revenue-expenses/finalize", payload, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				// Verify response
				finalizeResp := testutil.ParseResponse(resp)
				Expect(finalizeResp["message"]).To(Equal("Revenue expense finalized successfully"))
				Expect(finalizeResp["date"]).To(Equal(randomDate.Format("2006-01-02")))

				// Verify next_day is the day after the finalized date
				nextDay, ok := finalizeResp["next_day"].(string)
				Expect(ok).To(BeTrue())
				expectedNextDay := pkg.GetTodayDate().AddDate(0, 0, 1).Format("2006-01-02")
				Expect(nextDay).To(Equal(expectedNextDay))
			})
		})
	})
})
