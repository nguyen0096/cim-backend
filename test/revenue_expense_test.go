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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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

			tenv.ContextfulDB().Exec("TRUNCATE TABLE revenue_expense_finalizations")

			DeferCleanup(func() {
				// Clean up temp file
				if err := os.Remove(tempExcelFile); err != nil && !os.IsNotExist(err) {
					GinkgoWriter.Printf("Warning: failed to delete temp file %s: %v\n", tempExcelFile, err)
				}
				// Clean up temp directory
				if err := os.RemoveAll(filepath.Dir(tempExcelFile)); err != nil {
					GinkgoWriter.Printf("Warning: failed to delete temp dir: %v\n", err)
				}
				// Clean up finalization table
				tenv.ContextfulDB().Exec("TRUNCATE TABLE revenue_expense_finalizations")
			})
		})

		Context("when user has authorized role", func() {
			role := models.RoleAdmin
			It("should finalize revenue expense successfully with role admin", func(ctx SpecContext) {
				preparation := fixture.WithRevenueExpenseFinalizationsPreparation(tenv.ContextfulDB(), pkg.GetTodayDate())
				DeferCleanup(func() {
					fixture.CleanupRevenueExpenseFinalizationsPreparation(tenv.ContextfulDB(), preparation)
				})
				client := testutil.NewClient(tenv, role)

				// Use randomDate's date as the date to finalize
				payload := map[string]interface{}{}

				resp, err := client.MakeRequest("POST", "/api/v1/revenue-expenses/finalize", payload, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				// Validate response structure
				finalizeResp := testutil.ParseResponse(resp)
				Expect(finalizeResp["message"]).To(Equal("Revenue expense finalized successfully"))
				Expect(finalizeResp["date"]).To(Equal(pkg.GetTodayDate().Format("2006-01-02")))

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
				for i, approvedPaymentReceiptForm := range preparation.ApprovedPaymentReceiptForms {
					expenseData, err := excelRepo.MapRowToExpense(sheetName, rows[lastRowArrayIndex-preparation.TotalForms+i+1])
					Expect(err).NotTo(HaveOccurred())
					Expect(expenseData[pkg.RevenueExpenseColumnName]).To(Equal(preparation.Suppliers[0].Name))
					Expect(expenseData[pkg.RevenueExpenseColumnOrdinalNumber]).To(Equal(fmt.Sprintf("%d", i+1)))
					// MapRowToExpense returns values as strings (Excel stores them as text after uppercase conversion)
					// Convert string to float64 for comparison
					snackAndRiceValueStr, ok := expenseData[pkg.RevenueExpenseColumnSnackAndRice].(string)
					Expect(ok).To(BeTrue())
					var snackAndRiceValue float64
					normalizedValue := strings.ReplaceAll(strings.ToLower(snackAndRiceValueStr), ",", "")
					_, err = fmt.Sscanf(normalizedValue, "%f", &snackAndRiceValue)
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
			It("should return 200 when date format is invalid (validation only checks required)", func(ctx SpecContext) {
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

		Context("when finalization fails", func() {
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

			It("should create finalization record with failed status and reason", func(ctx SpecContext) {
				// Set lastFinalizedDate
				settingsCtx := pkg.WithUserEmail(tenv.DefaultContext, "test@cim.local")
				settingsRepo := repository.NewSettingsRepository(tenv.ContextfulDB())

				// Configure revenue expense settings with invalid file path to cause failure
				invalidFilePath := "/nonexistent/path/to/file.xlsx"
				settingsValue := map[string]interface{}{
					"filePath":  invalidFilePath,
					"sheetName": "TIỀN MẶT",
				}
				err := settingsRepo.Set(settingsCtx, config.RevenueExpenseExcelSettingsKey, settingsValue)
				Expect(err).NotTo(HaveOccurred())

				client := testutil.NewClient(tenv, models.RoleAdmin)

				payload := map[string]interface{}{
					"date": pkg.GetTodayDate().Format("2006-01-02"),
				}

				// Make request that will fail
				resp, err := client.MakeRequest("POST", "/api/v1/revenue-expenses/finalize", payload, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(500))

				// Query finalization repository to verify record was created
				finalizationRepo := repository.NewRevenueExpenseFinalizationRepository(tenv.ContextfulDB())
				finalizations, _, err := finalizationRepo.List(tenv.DefaultContext, 10, 0)
				Expect(err).NotTo(HaveOccurred())
				Expect(finalizations).NotTo(BeEmpty())

				// Get the most recent finalization record (should be the one we just created)
				latestFinalization := finalizations[0]
				Expect(latestFinalization.Status).NotTo(BeNil())
				Expect(*latestFinalization.Status).To(Equal(models.RevenueExpenseFinalizationStatusFailed))
				Expect(latestFinalization.Reason).NotTo(BeNil())
				Expect(*latestFinalization.Reason).NotTo(BeEmpty())
				// Check that reason contains error information (case-insensitive)
				reasonLower := strings.ToLower(*latestFinalization.Reason)
				Expect(reasonLower).To(
					ContainSubstring("failed to initialize excel repository: failed to open file"),
				)
			})
		})

		Context("when finalization is successful", Ordered, func() {
			It("should successfully finalize even when no payment receipt forms exist", func(ctx SpecContext) {
				client := testutil.NewClient(tenv, models.RoleAdmin)

				payload := map[string]interface{}{
					"date": pkg.GetTodayDate().Format("2006-01-02"),
				}

				resp, err := client.MakeRequest("POST", "/api/v1/revenue-expenses/finalize", payload, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				// Verify response
				finalizeResp := testutil.ParseResponse(resp)
				Expect(finalizeResp["message"]).To(Equal("Revenue expense finalized successfully"))
				Expect(finalizeResp["date"]).To(Equal(pkg.GetTodayDate().Format("2006-01-02")))

				// Verify next_day is the day after the finalized date
				nextDay, ok := finalizeResp["next_day"].(string)
				Expect(ok).To(BeTrue())
				expectedNextDay := pkg.GetTodayDate().AddDate(0, 0, 1).Format("2006-01-02")
				Expect(nextDay).To(Equal(expectedNextDay))
			})

			It("should create finalization record with success status and today's date when finalization is successful and no record found in finalization table", func(ctx SpecContext) {
				// Set lastFinalizedDate
				client := testutil.NewClient(tenv, models.RoleAdmin)

				payload := map[string]interface{}{
					"date": pkg.GetTodayDate().Format("2006-01-02"),
				}

				resp, err := client.MakeRequest("POST", "/api/v1/revenue-expenses/finalize", payload, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				// Verify response
				finalizeResp := testutil.ParseResponse(resp)
				Expect(finalizeResp["message"]).To(Equal("Revenue expense finalized successfully"))
				Expect(finalizeResp["date"]).To(Equal(pkg.GetTodayDate().Format("2006-01-02")))

				// Verify next_day is the day after the finalized date
				nextDay, ok := finalizeResp["next_day"].(string)
				Expect(ok).To(BeTrue())
				expectedNextDay := pkg.GetTodayDate().AddDate(0, 0, 1).Format("2006-01-02")
				Expect(nextDay).To(Equal(expectedNextDay))

				// Verify finalization record was created with success status
				finalizationRepo := repository.NewRevenueExpenseFinalizationRepository(tenv.ContextfulDB())
				finalizations, _, err := finalizationRepo.List(tenv.DefaultContext, 10, 0)
				Expect(err).NotTo(HaveOccurred())
				Expect(finalizations).NotTo(BeEmpty())
				latestFinalization := finalizations[0]
				Expect(latestFinalization.Status).NotTo(BeNil())
				Expect(*latestFinalization.Status).To(Equal(models.RevenueExpenseFinalizationStatusSuccess))
				Expect(latestFinalization.FinalizedDate).NotTo(BeNil())
				Expect(latestFinalization.FinalizedDate.Year()).To(Equal(pkg.GetTodayDate().Year()))
				Expect(latestFinalization.FinalizedDate.Month()).To(Equal(pkg.GetTodayDate().Month()))
				Expect(latestFinalization.FinalizedDate.Day()).To(Equal(pkg.GetTodayDate().Day()))
				DeferCleanup(func() {
					tenv.ContextfulDB().Exec("DELETE FROM revenue_expense_finalizations WHERE id = ?", latestFinalization.ID)
				})
			})

			It("should create finalization record with success status when finalization is successful and last finalized date is not today's date", func(ctx SpecContext) {
				// Set lastFinalizedDate using fixture
				randomDate := pkg.GetTodayDate().AddDate(0, 0, -5)
				fixture.WithRevenueExpenseFinalizations(tenv.ContextfulDB(), []*models.RevenueExpenseFinalization{
					{
						FinalizedDate: randomDate,
						Status:        pkg.Ptr(models.RevenueExpenseFinalizationStatusSuccess),
					},
				})

				client := testutil.NewClient(tenv, models.RoleAdmin)

				payload := map[string]interface{}{
					"prefix_date":   randomDate.Format("2006-01-02"),
					"date_in_excel": randomDate.Add(time.Hour * 24).Format("2006-01-02"),
				}

				resp, err := client.MakeRequest("POST", "/api/v1/revenue-expenses/finalize", payload, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				// Verify response
				finalizeResp := testutil.ParseResponse(resp)
				Expect(finalizeResp["message"]).To(Equal("Revenue expense finalized successfully"))
				Expect(finalizeResp["date"]).To(Equal(randomDate.Format("2006-01-02")))
				Expect(finalizeResp["next_day"]).To(Equal(pkg.GetTodayDate().AddDate(0, 0, 1).Format("2006-01-02")))

				// Verify finalization record was created with success status
				finalizationRepo := repository.NewRevenueExpenseFinalizationRepository(tenv.ContextfulDB())
				lastSuccessfulFinalization, err := finalizationRepo.GetLastest(tenv.DefaultContext)
				Expect(err).NotTo(HaveOccurred())
				Expect(lastSuccessfulFinalization).NotTo(BeNil())
				Expect(*lastSuccessfulFinalization.Status).To(Equal(models.RevenueExpenseFinalizationStatusSuccess))
				Expect(lastSuccessfulFinalization.FinalizedDate).NotTo(BeNil())
				Expect(lastSuccessfulFinalization.FinalizedDate.Year()).To(Equal(randomDate.Year()))
				Expect(lastSuccessfulFinalization.FinalizedDate.Month()).To(Equal(randomDate.Month()))
				Expect(lastSuccessfulFinalization.FinalizedDate.Day()).To(Equal(randomDate.Day()))
				Expect(lastSuccessfulFinalization.Reason).To(BeNil())
			})
		})
	})
})
