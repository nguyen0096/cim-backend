package apptest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"cim-backend/internal/models"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
	"cim-backend/pkg/testutil"
	"cim-backend/pkg/testutil/fixture"
)

// Column widths the length guards must respect, in characters. Declared from the
// schema rather than imported from the service constants, so a wrong constant fails
// here instead of agreeing with itself.
const (
	initialStockProductNameChars = 255 // products.name VARCHAR(255)
	initialStockUnitNameChars    = 100 // units.name VARCHAR(100)
	initialStockUnitSymbolChars  = 20  // units.symbol VARCHAR(20)
)

const (
	initialStockSheetsPath      = "/api/v1/tools/initial-stock/sheets"
	initialStockImportPath      = "/api/v1/tools/initial-stock/import"
	initialStockInventoriesPath = "/api/v1/tools/inventories"
)

// initialStockForm builds the import multipart body. dryRun is written verbatim so
// a spec can assert the strict-boolean parse.
func initialStockForm(data []byte, inventoryID uint, sheet, dryRun string) *testutil.MultipartFormData {
	return &testutil.MultipartFormData{
		Fields: map[string]string{
			"inventory_id": fmt.Sprintf("%d", inventoryID),
			"sheet_name":   sheet,
			"dry_run":      dryRun,
		},
		Files: initialStockFile(data, "initial_stock.xlsx"),
	}
}

func initialStockFile(data []byte, name string) map[string]struct {
	Filename string
	Content  io.Reader
} {
	return map[string]struct {
		Filename string
		Content  io.Reader
	}{
		"file": {Filename: name, Content: bytes.NewReader(data)},
	}
}

func mustMarshal(v interface{}) json.RawMessage {
	out, err := json.Marshal(v)
	Expect(err).NotTo(HaveOccurred())
	return out
}

// decodeImportResponse decodes a 200 body into the typed response.
func decodeImportResponse(resp *http.Response) *dto.InitialStockImportResponse {
	body, err := io.ReadAll(resp.Body)
	Expect(err).NotTo(HaveOccurred())
	var out dto.InitialStockImportResponse
	Expect(json.Unmarshal(body, &out)).To(Succeed(), "body: %s", string(body))
	// The frontend requires a real JSON boolean, so assert the raw encoding too.
	var raw map[string]json.RawMessage
	Expect(json.Unmarshal(body, &raw)).To(Succeed())
	Expect(string(raw["dry_run"])).To(Or(Equal("true"), Equal("false")),
		"dry_run must be echoed as a JSON boolean, got %s", string(raw["dry_run"]))
	// Pinned contract: every row carries warnings as an array, never null.
	var rawRows struct {
		Rows []map[string]json.RawMessage `json:"rows"`
	}
	Expect(json.Unmarshal(body, &rawRows)).To(Succeed())
	for i, row := range rawRows.Rows {
		Expect(string(row["warnings"])).To(HavePrefix("["),
			"row %d: warnings must be a JSON array, got %s", i, string(row["warnings"]))
	}
	return &out
}

var _ = Describe("Initial stock import tool", func() {
	var (
		developer *testutil.Client
		inventory *models.Inventory
		unit      *models.Unit
		suffix    string
	)

	BeforeEach(func() {
		suffix = uuid.NewString()[:8]
		db := tenv.ContextfulDB()
		developer = testutil.NewClient(tenv, models.RoleDeveloper)
		inventory = fixture.WithInventory(db, models.Inventory{
			Name:     fmt.Sprintf("isi-inv-%s", suffix),
			Location: fmt.Sprintf("isi-loc-%s", suffix),
			Status:   models.InventoryStatusActive,
		})
		unit = fixture.WithUnit(db, models.Unit{
			Name: fmt.Sprintf("ISIU%s", suffix), Symbol: fmt.Sprintf("ISIU%s", suffix),
			UnitType: "general", ConversionFactor: 1, Level: 1, DecimalPlaces: 2,
		})
	})

	singleSheet := func(rows []fixture.InitialStockRowSpec) []byte {
		return fixture.CreateInitialStockWorkbook([]fixture.InitialStockSheetSpec{{Name: "TON", Rows: rows}})
	}

	post := func(client *testutil.Client, path string, form *testutil.MultipartFormData, opts ...testutil.RequestOptions) *http.Response {
		options := append([]testutil.RequestOptions{testutil.WithAuth(), testutil.WithMultipartFormData(form)}, opts...)
		resp, err := client.MakeRequest(http.MethodPost, path, nil, options...)
		Expect(err).NotTo(HaveOccurred())
		return resp
	}

	runImport := func(data []byte, dryRun string, opts ...testutil.RequestOptions) *http.Response {
		return post(developer, initialStockImportPath, initialStockForm(data, inventory.ID, "TON", dryRun), opts...)
	}

	cleanupLoaded := func(names ...string) {
		DeferCleanup(func() {
			db := tenv.ContextfulDB()
			for _, name := range names {
				db.Exec(`DELETE FROM inventory_transactions WHERE inventory_item_id IN
					(SELECT ii.id FROM inventory_items ii JOIN products p ON p.id = ii.product_id
					 WHERE ii.inventory_id = ? AND UPPER(TRIM(p.name)) = UPPER(TRIM(?)))`, inventory.ID, name)
				db.Exec(`DELETE FROM inventory_items WHERE inventory_id = ? AND product_id IN
					(SELECT id FROM products WHERE UPPER(TRIM(name)) = UPPER(TRIM(?)))`, inventory.ID, name)
				db.Exec(`DELETE FROM products WHERE UPPER(TRIM(name)) = UPPER(TRIM(?))`, name)
			}
			db.Exec(`DELETE FROM initial_stock_imports WHERE inventory_id = ?`, inventory.ID)
		})
	}

	Describe("authorization", func() {
		It("allows developer and 403s every other role on all three endpoints", func() {
			data := singleSheet([]fixture.InitialStockRowSpec{
				{Name: fmt.Sprintf("RBAC %s", suffix), Unit: unit.Name, Quantity: "1", Category: "NƯỚC"},
			})

			for _, role := range []models.UserRole{
				models.RoleAdmin, models.RoleAccountant, models.RoleStaff,
				models.RoleBotForm, models.RoleChef, models.RoleWaiter, models.RoleCashier,
			} {
				other := testutil.NewClient(tenv, role)

				listResp, err := other.MakeRequest(http.MethodGet, initialStockInventoriesPath, nil, testutil.WithAuth())
				Expect(err).NotTo(HaveOccurred())
				Expect(listResp.StatusCode).To(Equal(http.StatusForbidden), "role %s must not list tool inventories", role)

				sheetsResp := post(other, initialStockSheetsPath, &testutil.MultipartFormData{
					Fields: map[string]string{}, Files: initialStockFile(data, "initial_stock.xlsx"),
				})
				Expect(sheetsResp.StatusCode).To(Equal(http.StatusForbidden), "role %s must not list sheets", role)

				importResp := post(other, initialStockImportPath, initialStockForm(data, inventory.ID, "TON", "true"))
				Expect(importResp.StatusCode).To(Equal(http.StatusForbidden), "role %s must not import", role)
			}

			listResp, err := developer.MakeRequest(http.MethodGet, initialStockInventoriesPath, nil, testutil.WithAuth())
			Expect(err).NotTo(HaveOccurred())
			Expect(listResp.StatusCode).To(Equal(http.StatusOK))
		})

		It("returns exactly the three developer permission strings, so the screen can become visible", func() {
			resp, err := developer.MakeRequest(http.MethodGet, "/api/v1/users/permissions", nil, testutil.WithAuth())
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			raw, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			var body struct {
				Permissions []string `json:"permissions"`
				Data        struct {
					Permissions []string `json:"permissions"`
				} `json:"data"`
			}
			Expect(json.Unmarshal(raw, &body)).To(Succeed())
			permissions := body.Permissions
			if len(permissions) == 0 {
				permissions = body.Data.Permissions
			}
			Expect(permissions).NotTo(BeEmpty(), "permissions payload: %s", string(raw))
			Expect(permissions).To(ConsistOf(
				"developer-tools:view",
				"initial-stock-import:import",
				"permissions:view",
			), "payload: %s", string(raw))
		})
	})

	Describe("sheet listing", func() {
		It("reports a header verdict and data row count per sheet", func() {
			data := fixture.CreateInitialStockWorkbook([]fixture.InitialStockSheetSpec{
				{Name: "TỒN 11-2025 KO CÓ CỘT ĐIỀN", Rows: fixture.InitialStockRows("SP", unit.Name, []string{"1", "2", "3"}, "NƯỚC")},
				{Name: "TỒN 05-07-2026", Rows: fixture.InitialStockRows("SP", unit.Name, []string{"4", "5"}, "NƯỚC")},
				// Report-style sheet: right columns, wrong row, so it must not be eligible.
				{Name: "Sheet1", HeaderRow: 6, Rows: fixture.InitialStockRows("R", unit.Name, []string{"9"}, "NƯỚC")},
			})

			resp := post(developer, initialStockSheetsPath, &testutil.MultipartFormData{
				Fields: map[string]string{}, Files: initialStockFile(data, "initial_stock.xlsx"),
			})
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			raw, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			var body dto.InitialStockSheetsResponse
			Expect(json.Unmarshal(raw, &body)).To(Succeed())
			Expect(body.Sheets).To(HaveLen(3))

			byName := map[string]dto.InitialStockSheetInfo{}
			for _, s := range body.Sheets {
				byName[s.Name] = s
			}
			Expect(byName["TỒN 11-2025 KO CÓ CỘT ĐIỀN"].HasExpectedHeader).To(BeTrue())
			Expect(byName["TỒN 11-2025 KO CÓ CỘT ĐIỀN"].DataRowCount).To(Equal(3))
			Expect(byName["TỒN 05-07-2026"].HasExpectedHeader).To(BeTrue())
			Expect(byName["TỒN 05-07-2026"].DataRowCount).To(Equal(2))
			Expect(byName["Sheet1"].HasExpectedHeader).To(BeFalse())
			Expect(byName["Sheet1"].Reason).To(Equal("header_not_found"))
		})
	})

	// A blank line in the middle of a real sheet used to end the read there, dropping
	// everything below it with no count and no error saying so.
	Describe("blank rows", func() {
		blankRowSheet := func() ([]byte, []string) {
			names := []string{
				fmt.Sprintf("BLANKA %s", suffix),
				fmt.Sprintf("BLANKB %s", suffix),
				fmt.Sprintf("BLANKC %s", suffix),
			}
			// Sheet rows: 4 A, 5 blank, 6 B, 7 blank, 8 blank, 9 C.
			return singleSheet([]fixture.InitialStockRowSpec{
				{Name: names[0], Unit: unit.Name, Quantity: "1", Category: "NƯỚC"},
				{Blank: true},
				{Name: names[1], Unit: unit.Name, Quantity: "2", Category: "NƯỚC"},
				{Blank: true},
				{Blank: true},
				{Name: names[2], Unit: unit.Name, Quantity: "3", Category: "NƯỚC"},
			}), names
		}

		It("reads every row below a blank one, and the sheet listing counts the same rows", func() {
			data, names := blankRowSheet()

			listing := post(developer, initialStockSheetsPath, &testutil.MultipartFormData{
				Fields: map[string]string{}, Files: initialStockFile(data, "initial_stock.xlsx"),
			})
			Expect(listing.StatusCode).To(Equal(http.StatusOK))
			rawListing, err := io.ReadAll(listing.Body)
			Expect(err).NotTo(HaveOccurred())
			var sheets dto.InitialStockSheetsResponse
			Expect(json.Unmarshal(rawListing, &sheets)).To(Succeed())
			Expect(sheets.Sheets).To(HaveLen(1))
			Expect(sheets.Sheets[0].DataRowCount).To(Equal(3),
				"the listing must count what the import will read: %s", string(rawListing))

			dry := decodeImportResponse(runImport(data, "true"))
			Expect(dry.Errors).To(BeEmpty())
			Expect(dry.RowsProcessed).To(Equal(3))
			Expect(dry.RowsOK).To(Equal(3))
			Expect(dry.Rows).To(HaveLen(3))
			for i, row := range dry.Rows {
				Expect(row.Name).To(Equal(names[i]))
			}
			Expect([]int{dry.Rows[0].Row, dry.Rows[1].Row, dry.Rows[2].Row}).To(Equal([]int{4, 6, 9}),
				"a skipped blank row must not renumber the rows below it")
		})

		It("loads every row below a blank one on apply", func() {
			data, names := blankRowSheet()
			cleanupLoaded(names...)

			applied := decodeImportResponse(runImport(data, "false"))
			Expect(applied.RowsProcessed).To(Equal(3))
			Expect(applied.TotalQuantity).To(Equal("6"))

			db := tenv.ContextfulDB()
			for _, name := range names {
				var loaded int64
				Expect(db.Model(&models.InventoryItem{}).
					Joins("JOIN products p ON p.id = inventory_items.product_id").
					Where("inventory_items.inventory_id = ? AND UPPER(TRIM(p.name)) = ?",
						inventory.ID, strings.ToUpper(name)).
					Count(&loaded).Error).NotTo(HaveOccurred())
				Expect(loaded).To(Equal(int64(1)), "%s must be loaded, not truncated away", name)
			}
		})
	})

	Describe("inventory picker", func() {
		It("returns the minimal shape under the data key", func() {
			resp, err := developer.MakeRequest(http.MethodGet, initialStockInventoriesPath, nil, testutil.WithAuth())
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			raw, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			var body dto.InitialStockInventoriesResponse
			Expect(json.Unmarshal(raw, &body)).To(Succeed())

			var found *dto.InitialStockInventoryOption
			for i := range body.Data {
				if body.Data[i].ID == inventory.ID {
					found = &body.Data[i]
				}
			}
			Expect(found).NotTo(BeNil(), "the active inventory must be offered: %s", string(raw))
			Expect(found.Name).To(Equal(inventory.Name))
			Expect(found.Status).To(Equal(string(models.InventoryStatusActive)))
		})
	})

	Describe("dry run", func() {
		It("plans every row, reports resulting quantities, and writes nothing", func() {
			db := tenv.ContextfulDB()
			existing := fixture.WithProduct(db, models.Product{
				Name: fmt.Sprintf("DRYMATCH %s", suffix), UnitID: unit.ID, Status: "active",
			})
			item := &models.InventoryItem{
				InventoryID: inventory.ID, ProductID: existing.ID, UnitID: unit.ID,
				Quantity: decimal.NewFromInt(4), Status: models.InventoryItemStatusActive,
			}
			Expect(db.Create(item).Error).NotTo(HaveOccurred())
			DeferCleanup(func() {
				db.Unscoped().Where("inventory_item_id = ?", item.ID).Delete(&models.InventoryTransaction{})
				db.Unscoped().Delete(item)
			})

			newName := fmt.Sprintf("DRYNEW %s", suffix)
			data := singleSheet([]fixture.InitialStockRowSpec{
				{Name: existing.Name, Unit: unit.Name, Quantity: "6", Category: "NƯỚC"},
				{Name: newName + " ", Unit: unit.Name, Quantity: "1.5", Category: "ĂN NHẸ"},
				{Name: fmt.Sprintf("DRYZERO %s", suffix), Unit: unit.Name, Quantity: "0", Category: "NƯỚC"},
			})

			resp := runImport(data, "true")
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			body := decodeImportResponse(resp)

			Expect(body.DryRun).To(BeTrue())
			Expect(body.InventoryID).To(Equal(inventory.ID))
			Expect(body.SheetName).To(Equal("TON"))
			Expect(body.Errors).To(BeEmpty())
			Expect(body.Blocking).To(BeEmpty())

			// Flat frontend counters and the summary must agree.
			Expect(body.RowsProcessed).To(Equal(3))
			Expect(body.ProductsCreated).To(Equal(2))
			Expect(body.ProductsMatched).To(Equal(1))
			Expect(body.ItemsCreated).To(Equal(2))
			Expect(body.TransactionsCreated).To(Equal(2))
			Expect(body.RowsSkipped).To(Equal(1))
			Expect(body.TotalQuantity).To(Equal("7.5"))
			Expect(body.RowsOnItemsWithExistingStock).To(Equal(1))

			Expect(body.Rows).To(HaveLen(3))
			Expect(body.Rows[0].Row).To(Equal(4), "sheet row numbers are 1-based and start below the header")
			Expect(body.Rows[0].Actions).To(Equal([]string{"match_product", "create_transaction"}))
			Expect(body.Rows[0].CurrentQuantity).To(Equal("4"))
			Expect(body.Rows[0].ResultingQuantity).To(Equal("10"))
			Expect(body.Rows[0].UnitDecimalPlaces).To(Equal(2))

			Expect(body.Rows[1].Name).To(Equal(newName), "a trailing space must be trimmed")
			Expect(body.Rows[1].Actions).To(Equal([]string{"create_product", "create_item", "create_transaction"}))
			Expect(body.Rows[1].Quantity).To(Equal("1.5"))
			Expect(body.Rows[1].ResultingQuantity).To(Equal("1.5"))

			Expect(body.Rows[2].Actions).To(Equal([]string{"create_product", "create_item", "skip_zero_quantity"}))

			// Nothing written.
			var reloaded models.InventoryItem
			Expect(db.First(&reloaded, item.ID).Error).NotTo(HaveOccurred())
			Expect(reloaded.Quantity.Equal(decimal.NewFromInt(4))).To(BeTrue(), "dry run must not move on-hand")

			var created int64
			Expect(db.Model(&models.Product{}).Unscoped().
				Where("UPPER(TRIM(name)) = ?", strings.ToUpper(newName)).Count(&created).Error).NotTo(HaveOccurred())
			Expect(created).To(BeZero(), "dry run must not create a product")

			var txns int64
			Expect(db.Model(&models.InventoryTransaction{}).
				Where("transaction_type = ?", models.InventoryTransactionTypeInitial).
				Count(&txns).Error).NotTo(HaveOccurred())
			Expect(txns).To(BeZero(), "dry run must not write an initial transaction")
		})

		It("fails closed on any dry_run value that is not exactly true or false", func() {
			data := singleSheet([]fixture.InitialStockRowSpec{
				{Name: fmt.Sprintf("CLOSED %s", suffix), Unit: unit.Name, Quantity: "3", Category: "NƯỚC"},
			})

			for _, value := range []string{"", "1", "TRUE", "yes", "False"} {
				resp := runImport(data, value)
				Expect(resp.StatusCode).To(Equal(http.StatusBadRequest), "dry_run=%q must be rejected", value)
				Expect(testutil.ParseResponse(resp)["key"]).To(Equal(pkg.ErrKeyInitialStockInvalidDryRun))
			}

			// Absent entirely.
			form := initialStockForm(data, inventory.ID, "TON", "true")
			delete(form.Fields, "dry_run")
			resp := post(developer, initialStockImportPath, form)
			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
			Expect(testutil.ParseResponse(resp)["key"]).To(Equal(pkg.ErrKeyInitialStockInvalidDryRun))

			var txns int64
			Expect(tenv.ContextfulDB().Model(&models.InventoryTransaction{}).
				Where("transaction_type = ?", models.InventoryTransactionTypeInitial).
				Count(&txns).Error).NotTo(HaveOccurred())
			Expect(txns).To(BeZero())
		})
	})

	Describe("matched product type", func() {
		It("keeps the product's type, previews it, and warns naming both values", func() {
			db := tenv.ContextfulDB()
			typed := fixture.WithProduct(db, models.Product{
				Name: fmt.Sprintf("PTTYPED %s", suffix), UnitID: unit.ID, Status: "active", ProductType: "CƠM",
			})
			untyped := fixture.WithProduct(db, models.Product{
				Name: fmt.Sprintf("PTNONE %s", suffix), UnitID: unit.ID, Status: "active",
			})
			agreeing := fixture.WithProduct(db, models.Product{
				Name: fmt.Sprintf("PTSAME %s", suffix), UnitID: unit.ID, Status: "active", ProductType: "NƯỚC",
			})
			fresh := fmt.Sprintf("PTNEW %s", suffix)

			data := singleSheet([]fixture.InitialStockRowSpec{
				{Name: typed.Name, Unit: unit.Name, Quantity: "1", Category: "NƯỚC"},
				{Name: untyped.Name, Unit: unit.Name, Quantity: "1", Category: "NƯỚC"},
				{Name: agreeing.Name, Unit: unit.Name, Quantity: "1", Category: "NƯỚC"},
				{Name: fresh, Unit: unit.Name, Quantity: "1", Category: "NƯỚC"},
			})

			dry := decodeImportResponse(runImport(data, "true"))
			Expect(dry.Errors).To(BeEmpty(), "a differing type warns, it never rejects: %v", dry.Errors)
			Expect(dry.Rows).To(HaveLen(4))

			Expect(dry.Rows[0].ProductType).To(Equal("CƠM"), "the preview must show the type that will be in effect")
			Expect(dry.Rows[0].Warnings).To(Equal([]string{
				pkg.RowMessage(pkg.WarnKeyInitialStockRowProductTypeIgnored, "NƯỚC", "CƠM")}))

			Expect(dry.Rows[1].ProductType).To(BeEmpty())
			Expect(dry.Rows[1].Warnings).To(Equal([]string{
				pkg.RowMessage(pkg.WarnKeyInitialStockRowProductTypeIgnoredNoType, "NƯỚC")}))

			Expect(dry.Rows[2].ProductType).To(Equal("NƯỚC"))
			Expect(dry.Rows[2].Warnings).To(BeEmpty(), "an agreeing type is not a warning")

			Expect(dry.Rows[3].ProductType).To(Equal("NƯỚC"), "a new product does take the sheet's type")
			Expect(dry.Rows[3].Warnings).To(BeEmpty())

			cleanupLoaded(typed.Name, untyped.Name, agreeing.Name, fresh)
			applied := decodeImportResponse(runImport(data, "false"))
			Expect(applied.Rows).To(HaveLen(4))
			Expect(applied.Rows[0].Warnings).To(HaveLen(1), "the warning survives the apply response")

			for _, expected := range []struct {
				id   uint
				kind string
			}{{typed.ID, "CƠM"}, {untyped.ID, ""}, {agreeing.ID, "NƯỚC"}} {
				var reloaded models.Product
				Expect(db.First(&reloaded, expected.id).Error).NotTo(HaveOccurred())
				Expect(reloaded.ProductType).To(Equal(expected.kind),
					"product %d must keep its own type", expected.id)
			}
		})
	})

	Describe("apply", func() {
		It("loads a 68-row sheet exactly, including fractional quantities and diacritic-distinct units", func() {
			names := make([]string, 0, 68)
			rows := make([]fixture.InitialStockRowSpec, 0, 68)
			// Mirrors the real sheet: 68 rows, 5 fractional, trailing-space names, and
			// the three diacritic variants that must stay distinct units.
			fractional := map[int]string{3: "0.5", 11: "0.7", 27: "0.9", 44: "1.8", 60: "0.5"}
			expectedTotal := decimal.Zero
			for i := 0; i < 68; i++ {
				name := fmt.Sprintf("BULK %s %02d", suffix, i)
				if i%8 == 0 {
					name += " " // trailing space, must be trimmed
				}
				qty := fmt.Sprintf("%d", 10+i)
				if f, ok := fractional[i]; ok {
					qty = f
				}
				unitLabel := unit.Name
				switch i {
				case 5:
					unitLabel = "CUỐN"
				case 6:
					unitLabel = "CUỘN"
				case 7:
					unitLabel = "CUÔN"
				}
				category := "NƯỚC"
				if i%6 == 0 {
					category = "ĂN NHẸ"
				}
				rows = append(rows, fixture.InitialStockRowSpec{
					Name: name, Unit: unitLabel, Quantity: qty, Category: category,
				})
				names = append(names, strings.TrimSpace(name))
				expectedTotal = expectedTotal.Add(decimal.RequireFromString(qty))
			}
			cleanupLoaded(names...)

			data := singleSheet(rows)
			resp := runImport(data, "false", testutil.WithHeader("Idempotency-Key", uuid.NewString()))
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			body := decodeImportResponse(resp)

			Expect(body.DryRun).To(BeFalse())
			Expect(body.RowsProcessed).To(Equal(68))
			Expect(body.RowsOK).To(Equal(68))
			Expect(body.RowsFailed).To(BeZero())
			Expect(body.Errors).To(BeEmpty())
			Expect(body.ProductsCreated).To(Equal(68))
			Expect(body.ItemsCreated).To(Equal(68))
			Expect(body.TransactionsCreated).To(Equal(68))
			Expect(body.RowsSkipped).To(BeZero())
			Expect(body.TotalQuantity).To(Equal(expectedTotal.String()))
			// CUỐN, CUỘN and CUÔN are three units; the shared unit already exists.
			Expect(body.UnitsCreated).To(Equal(3))

			db := tenv.ContextfulDB()
			var unitCount int64
			Expect(db.Model(&models.Unit{}).Where("unit_type = ? AND name IN ?", "general",
				[]string{"CUỐN", "CUỘN", "CUÔN"}).Count(&unitCount).Error).NotTo(HaveOccurred())
			Expect(unitCount).To(Equal(int64(3)), "diacritics must never be folded when resolving units")
			DeferCleanup(func() {
				db.Exec("DELETE FROM units WHERE unit_type = ? AND name IN ?", "general",
					[]string{"CUỐN", "CUỘN", "CUÔN"})
			})

			// Fractional quantities land exactly on both the item and its layer.
			for i, q := range fractional {
				name := strings.TrimSpace(names[i])
				var item models.InventoryItem
				Expect(db.Joins("JOIN products p ON p.id = inventory_items.product_id").
					Where("inventory_items.inventory_id = ? AND UPPER(TRIM(p.name)) = ?", inventory.ID, strings.ToUpper(name)).
					First(&item).Error).NotTo(HaveOccurred())
				Expect(item.Quantity.Equal(decimal.RequireFromString(q))).To(BeTrue(),
					"row %d on-hand must be %s, got %s", i, q, item.Quantity)

				var layer models.InventoryTransaction
				Expect(db.Where("inventory_item_id = ? AND transaction_type = ?",
					item.ID, models.InventoryTransactionTypeInitial).First(&layer).Error).NotTo(HaveOccurred())
				Expect(layer.Quantity.Equal(decimal.RequireFromString(q))).To(BeTrue())
				Expect(layer.Price).To(Equal(0.0))
			}

			// Selling prices are never touched by this tool.
			var sellingPrices int64
			Expect(db.Table("selling_prices").
				Joins("JOIN products p ON p.id = selling_prices.product_id").
				Where("UPPER(TRIM(p.name)) LIKE ?", strings.ToUpper(fmt.Sprintf("BULK %s%%", suffix))).
				Count(&sellingPrices).Error).NotTo(HaveOccurred())
			Expect(sellingPrices).To(BeZero(), "the loader must never write a selling price")
		})

		It("adds to an existing item's quantity without touching its other columns", func() {
			db := tenv.ContextfulDB()
			otherUnit := fixture.WithUnit(db, models.Unit{
				Name: fmt.Sprintf("ISIOTHER%s", suffix), Symbol: fmt.Sprintf("ISIOTHER%s", suffix),
				UnitType: "general", ConversionFactor: 1, Level: 1, DecimalPlaces: 2,
			})
			product := fixture.WithProduct(db, models.Product{
				Name: fmt.Sprintf("KEEPCOLS %s", suffix), UnitID: otherUnit.ID, Status: "active",
			})
			item := &models.InventoryItem{
				InventoryID: inventory.ID, ProductID: product.ID, UnitID: otherUnit.ID,
				Quantity: decimal.NewFromInt(3), Status: models.InventoryItemStatusActive,
				ConsumingTransactionID: 0,
			}
			Expect(db.Create(item).Error).NotTo(HaveOccurred())
			DeferCleanup(func() {
				db.Unscoped().Where("inventory_item_id = ?", item.ID).Delete(&models.InventoryTransaction{})
				db.Unscoped().Delete(item)
				db.Exec("DELETE FROM initial_stock_imports WHERE inventory_id = ?", inventory.ID)
			})

			data := singleSheet([]fixture.InitialStockRowSpec{
				{Name: product.Name, Unit: otherUnit.Name, Quantity: "2.25", Category: "NƯỚC"},
			})
			resp := runImport(data, "false", testutil.WithHeader("Idempotency-Key", uuid.NewString()))
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			var reloaded models.InventoryItem
			Expect(db.First(&reloaded, item.ID).Error).NotTo(HaveOccurred())
			Expect(reloaded.Quantity.Equal(decimal.RequireFromString("5.25"))).To(BeTrue(),
				"on-hand must be 3 + 2.25, got %s", reloaded.Quantity)
			Expect(reloaded.Status).To(Equal(models.InventoryItemStatusActive), "status must survive the batch upsert")
			Expect(reloaded.UnitID).To(Equal(otherUnit.ID), "unit_id must survive the batch upsert")
			Expect(reloaded.ConsumingTransactionID).To(Equal(item.ConsumingTransactionID),
				"consuming_transaction_id must survive the batch upsert")

			// The product itself is untouched.
			var reloadedProduct models.Product
			Expect(db.First(&reloadedProduct, product.ID).Error).NotTo(HaveOccurred())
			Expect(reloadedProduct.Name).To(Equal(product.Name))
			Expect(reloadedProduct.UnitID).To(Equal(otherUnit.ID))
		})
	})

	Describe("idempotency", func() {
		It("replays a committed key, refuses a fresh key, and refuses key reuse for another payload", func() {
			name := fmt.Sprintf("IDEM %s", suffix)
			cleanupLoaded(name)
			data := singleSheet([]fixture.InitialStockRowSpec{
				{Name: name, Unit: unit.Name, Quantity: "7", Category: "NƯỚC"},
			})
			key := uuid.NewString()

			first := runImport(data, "false", testutil.WithHeader("Idempotency-Key", key))
			Expect(first.StatusCode).To(Equal(http.StatusOK))
			firstBody := decodeImportResponse(first)

			// Same key, same payload: the stored result comes back, including rows[].
			replay := runImport(data, "false", testutil.WithHeader("Idempotency-Key", key))
			Expect(replay.StatusCode).To(Equal(http.StatusOK))
			replayBody := decodeImportResponse(replay)
			Expect(replayBody.Rows).To(HaveLen(len(firstBody.Rows)))
			Expect(replayBody.Rows).To(Equal(firstBody.Rows), "a replay must satisfy the same rows[] contract")
			Expect(replayBody.TotalQuantity).To(Equal(firstBody.TotalQuantity))

			db := tenv.ContextfulDB()
			var layers int64
			Expect(db.Model(&models.InventoryTransaction{}).
				Joins("JOIN inventory_items ii ON ii.id = inventory_transactions.inventory_item_id").
				Where("ii.inventory_id = ? AND inventory_transactions.transaction_type = ?",
					inventory.ID, models.InventoryTransactionTypeInitial).
				Count(&layers).Error).NotTo(HaveOccurred())
			Expect(layers).To(Equal(int64(1)), "a replay must not write a second layer")

			// A fresh key against an already-loaded inventory is refused.
			fresh := runImport(data, "false", testutil.WithHeader("Idempotency-Key", uuid.NewString()))
			Expect(fresh.StatusCode).To(Equal(http.StatusConflict))
			freshBody := testutil.ParseResponse(fresh)
			Expect(freshBody["key"]).To(Equal(pkg.ErrKeyInitialStockAlreadyImported))
			Expect(freshBody["code"]).To(Equal("conflict"))

			// The same key with a different file is a mismatch, never a false replay.
			otherName := fmt.Sprintf("IDEM2 %s", suffix)
			cleanupLoaded(otherName)
			otherData := singleSheet([]fixture.InitialStockRowSpec{
				{Name: otherName, Unit: unit.Name, Quantity: "9", Category: "NƯỚC"},
			})
			mismatch := runImport(otherData, "false", testutil.WithHeader("Idempotency-Key", key))
			Expect(mismatch.StatusCode).To(Equal(http.StatusConflict))
			Expect(testutil.ParseResponse(mismatch)["key"]).To(Equal(pkg.ErrKeyInitialStockKeyPayloadMismatch))
		})

		// A receipt written before per-row warnings existed stores the sheet's
		// product_type on every row and no warnings key at all. Replaying it verbatim
		// would report a type that was never applied as the effective one, which is the
		// lie the warning exists to remove. The payload below is the shape the pre-change
		// response marshalled: same keys, same order, warnings absent.
		It("restates a pre-change receipt so a replay tells the same truth a fresh run would", func() {
			db := tenv.ContextfulDB()
			matched := fixture.WithProduct(db, models.Product{
				Name: fmt.Sprintf("LEGMATCH %s", suffix), UnitID: unit.ID, Status: "active", ProductType: "CƠM",
			})
			created := fixture.WithProduct(db, models.Product{
				Name: fmt.Sprintf("LEGNEW %s", suffix), UnitID: unit.ID, Status: "active", ProductType: "NƯỚC",
			})
			// A row whose product no longer exists at all: no effective type is readable.
			goneID := created.ID + 1000000

			data := singleSheet([]fixture.InitialStockRowSpec{
				{Name: matched.Name, Unit: unit.Name, Quantity: "3", Category: "NƯỚC"},
			})
			sum := sha256.Sum256(data)

			key := uuid.NewString()
			legacy := fmt.Sprintf(`{
				"dry_run": false, "inventory_id": %d, "sheet_name": "TON",
				"blocking": [],
				"rows": [
					{"row":4,"name":%q,"unit":%q,"quantity":"3","product_type":"NƯỚC","product_id":%d,
					 "actions":["match_product","create_transaction"],
					 "current_quantity":"0","resulting_quantity":"3","unit_decimal_places":2},
					{"row":5,"name":%q,"unit":%q,"quantity":"1","product_type":"NƯỚC","product_id":%d,
					 "actions":["create_product","create_item","create_transaction"],
					 "current_quantity":"0","resulting_quantity":"1","unit_decimal_places":2},
					{"row":6,"name":"LEGGONE","unit":%q,"quantity":"2","product_type":"NƯỚC","product_id":%d,
					 "actions":["match_product","create_item","create_transaction"],
					 "current_quantity":"0","resulting_quantity":"2","unit_decimal_places":2}
				],
				"errors": [],
				"rows_processed": 3, "products_created": 1, "products_matched": 2,
				"items_created": 2, "transactions_created": 3, "rows_skipped": 0,
				"rows_ok": 3, "rows_failed": 0, "units_created": 0,
				"total_quantity": "6", "rows_on_items_with_existing_stock": 0
			}`, inventory.ID, matched.Name, unit.Name, matched.ID,
				created.Name, unit.Name, created.ID, unit.Name, goneID)
			Expect(json.Valid([]byte(legacy))).To(BeTrue())
			Expect(legacy).NotTo(ContainSubstring("warnings"), "the fixture must be the pre-change shape")

			receipt := &models.InitialStockImport{
				IdempotencyKey: key, InventoryID: inventory.ID, SheetName: "TON",
				FileName: "initial_stock.xlsx", FileSHA256: hex.EncodeToString(sum[:]),
				RowCount: 3, ResultSummary: json.RawMessage(legacy), CreatedBy: "legacy@cim.local",
			}
			Expect(db.Create(receipt).Error).NotTo(HaveOccurred())
			DeferCleanup(func() { db.Exec("DELETE FROM initial_stock_imports WHERE id = ?", receipt.ID) })

			replay := runImport(data, "false", testutil.WithHeader("Idempotency-Key", key))
			Expect(replay.StatusCode).To(Equal(http.StatusOK))
			body := decodeImportResponse(replay)
			Expect(body.Rows).To(HaveLen(3))

			// Matched: the product's own type, and the warning naming both values.
			Expect(body.Rows[0].ProductType).To(Equal("CƠM"),
				"a replay must not report the ignored sheet type as effective")
			Expect(body.Rows[0].Warnings).To(Equal([]string{
				pkg.RowMessage(pkg.WarnKeyInitialStockRowProductTypeIgnored, "NƯỚC", "CƠM")}))

			// Created: the sheet type was applied, so the stored value already stands.
			Expect(body.Rows[1].ProductType).To(Equal("NƯỚC"))
			Expect(body.Rows[1].Warnings).To(BeEmpty())

			// Product gone: no effective type can be stated, and the row says why.
			Expect(body.Rows[2].ProductType).To(BeEmpty())
			Expect(body.Rows[2].Warnings).To(Equal([]string{
				pkg.RowMessage(pkg.WarnKeyInitialStockRowProductTypeIgnoredUnreadable, "NƯỚC")}))

			// The recorded outcome itself is replayed untouched.
			Expect(body.RowsProcessed).To(Equal(3))
			Expect(body.ProductsMatched).To(Equal(2))
			Expect(body.TotalQuantity).To(Equal("6"))
			Expect(body.Rows[0].ResultingQuantity).To(Equal("3"))

			var layers int64
			Expect(db.Model(&models.InventoryTransaction{}).
				Joins("JOIN inventory_items ii ON ii.id = inventory_transactions.inventory_item_id").
				Where("ii.inventory_id = ?", inventory.ID).Count(&layers).Error).NotTo(HaveOccurred())
			Expect(layers).To(BeZero(), "a replay must write nothing")
		})

		// An unreadable product and a blank sheet cell are independent facts. Reporting
		// the first only when the second is non-blank leaves product_type:"" with no
		// warning, which reads as "the product has no type" — a claim the code cannot
		// make. The soft-deleted row pins the other half: it is still readable, so it
		// answers for its own type rather than falling into the unreadable branch.
		It("always says an unreadable product's type cannot be shown, blank sheet cell or not", func() {
			db := tenv.ContextfulDB()
			softDeleted := fixture.WithProduct(db, models.Product{
				Name: fmt.Sprintf("LEGSOFT %s", suffix), UnitID: unit.ID, Status: "active", ProductType: "CƠM",
			})
			Expect(db.Delete(&models.Product{}, softDeleted.ID).Error).NotTo(HaveOccurred())
			var stillThere models.Product
			Expect(db.Unscoped().First(&stillThere, softDeleted.ID).Error).NotTo(HaveOccurred())
			Expect(stillThere.DeletedAt.Valid).To(BeTrue(), "the fixture must be soft-deleted, not purged")

			goneID := softDeleted.ID + 1000000
			data := singleSheet([]fixture.InitialStockRowSpec{
				{Name: fmt.Sprintf("LEGBLANK %s", suffix), Unit: unit.Name, Quantity: "1", Category: "NƯỚC"},
			})
			sum := sha256.Sum256(data)
			key := uuid.NewString()

			legacy := fmt.Sprintf(`{
				"dry_run": false, "inventory_id": %d, "sheet_name": "TON",
				"blocking": [],
				"rows": [
					{"row":4,"name":"LEGBLANKGONE","unit":%q,"quantity":"1","product_type":"","product_id":%d,
					 "actions":["match_product","create_transaction"],
					 "current_quantity":"0","resulting_quantity":"1","unit_decimal_places":2},
					{"row":5,"name":%q,"unit":%q,"quantity":"1","product_type":"NƯỚC","product_id":%d,
					 "actions":["match_product","create_transaction"],
					 "current_quantity":"0","resulting_quantity":"1","unit_decimal_places":2}
				],
				"errors": [],
				"rows_processed": 2, "products_created": 0, "products_matched": 2,
				"items_created": 0, "transactions_created": 2, "rows_skipped": 0,
				"rows_ok": 2, "rows_failed": 0, "units_created": 0,
				"total_quantity": "2", "rows_on_items_with_existing_stock": 0
			}`, inventory.ID, unit.Name, goneID, softDeleted.Name, unit.Name, softDeleted.ID)
			Expect(json.Valid([]byte(legacy))).To(BeTrue())
			Expect(legacy).NotTo(ContainSubstring("warnings"))

			receipt := &models.InitialStockImport{
				IdempotencyKey: key, InventoryID: inventory.ID, SheetName: "TON",
				FileName: "initial_stock.xlsx", FileSHA256: hex.EncodeToString(sum[:]),
				RowCount: 2, ResultSummary: json.RawMessage(legacy), CreatedBy: "legacy@cim.local",
			}
			Expect(db.Create(receipt).Error).NotTo(HaveOccurred())
			DeferCleanup(func() { db.Exec("DELETE FROM initial_stock_imports WHERE id = ?", receipt.ID) })

			body := decodeImportResponse(runImport(data, "false", testutil.WithHeader("Idempotency-Key", key)))
			Expect(body.Rows).To(HaveLen(2))

			// Blank stored sheet type, product unreadable: still explained.
			Expect(body.Rows[0].ProductType).To(BeEmpty())
			Expect(body.Rows[0].Warnings).To(Equal([]string{
				pkg.RowMessage(pkg.WarnKeyInitialStockRowProductTypeUnreadable)}),
				"an empty type with no warning is indistinguishable from a product that has none")

			// Soft-deleted product: readable, so it answers for its own type.
			Expect(body.Rows[1].ProductType).To(Equal("CƠM"),
				"a soft-deleted product is still readable and is not the unreadable case")
			Expect(body.Rows[1].Warnings).To(Equal([]string{
				pkg.RowMessage(pkg.WarnKeyInitialStockRowProductTypeIgnored, "NƯỚC", "CƠM")}))
		})

		// ACCEPTANCE CRITERION, binding on the frontend contract: the refusal is scoped
		// to the INVENTORY, not to the idempotency key. The frontend holds its key in
		// memory only, so browser Back mid-apply, a refresh, a tab close or a file
		// re-attach all mint a fresh key; this guard is then the only thing preventing
		// a second load. Relaxing it to a key-scoped check must fail the build.
		It("refuses a second load per inventory even under a brand-new idempotency key", func() {
			name := fmt.Sprintf("PERINV %s", suffix)
			cleanupLoaded(name)
			data := singleSheet([]fixture.InitialStockRowSpec{
				{Name: name, Unit: unit.Name, Quantity: "12", Category: "NƯỚC"},
			})

			Expect(runImport(data, "false", testutil.WithHeader("Idempotency-Key", uuid.NewString())).StatusCode).
				To(Equal(http.StatusOK))

			// Every key the frontend could possibly present after losing its own.
			for _, header := range []testutil.RequestOptions{
				testutil.WithHeader("Idempotency-Key", uuid.NewString()),
				testutil.WithHeader("Idempotency-Key", uuid.NewString()),
			} {
				resp := runImport(data, "false", header)
				Expect(resp.StatusCode).To(Equal(http.StatusConflict),
					"a fresh key must not buy a second load of the same inventory")
				body := testutil.ParseResponse(resp)
				Expect(body["key"]).To(Equal(pkg.ErrKeyInitialStockAlreadyImported))
				Expect(body["code"]).To(Equal("conflict"))
			}

			// And with no key at all, which is what a client that never issued one sends.
			noKey := runImport(data, "false")
			Expect(noKey.StatusCode).To(Equal(http.StatusConflict))
			Expect(testutil.ParseResponse(noKey)["key"]).To(Equal(pkg.ErrKeyInitialStockAlreadyImported))

			db := tenv.ContextfulDB()
			var layers int64
			Expect(db.Model(&models.InventoryTransaction{}).
				Joins("JOIN inventory_items ii ON ii.id = inventory_transactions.inventory_item_id").
				Where("ii.inventory_id = ? AND inventory_transactions.transaction_type = ?",
					inventory.ID, models.InventoryTransactionTypeInitial).
				Count(&layers).Error).NotTo(HaveOccurred())
			Expect(layers).To(Equal(int64(1)), "exactly one opening-stock layer may ever exist per load")

			// The guard is per inventory, so a different inventory still loads, and it
			// may legitimately reuse a key already committed against the first.
			other := fixture.WithInventory(db, models.Inventory{
				Name: fmt.Sprintf("isi-other-%s", suffix), Location: "other", Status: models.InventoryStatusActive,
			})
			otherResp, err := developer.MakeRequest(http.MethodPost, initialStockImportPath, nil,
				testutil.WithAuth(), testutil.WithHeader("Idempotency-Key", uuid.NewString()),
				testutil.WithMultipartFormData(initialStockForm(data, other.ID, "TON", "false")))
			Expect(err).NotTo(HaveOccurred())
			Expect(otherResp.StatusCode).To(Equal(http.StatusOK),
				"the guard must not leak across inventories")
			DeferCleanup(func() {
				db.Exec(`DELETE FROM inventory_transactions WHERE inventory_item_id IN
					(SELECT id FROM inventory_items WHERE inventory_id = ?)`, other.ID)
				db.Exec(`DELETE FROM initial_stock_imports WHERE inventory_id = ?`, other.ID)
				db.Exec(`DELETE FROM inventory_items WHERE inventory_id = ?`, other.ID)
			})
		})

		It("replays a committed key even while a reconcile is open", func() {
			db := tenv.ContextfulDB()
			name := fmt.Sprintf("REPLAYOPEN %s", suffix)
			cleanupLoaded(name)
			data := singleSheet([]fixture.InitialStockRowSpec{
				{Name: name, Unit: unit.Name, Quantity: "8", Category: "NƯỚC"},
			})
			key := uuid.NewString()

			first := runImport(data, "false", testutil.WithHeader("Idempotency-Key", key))
			Expect(first.StatusCode).To(Equal(http.StatusOK))
			firstBody := decodeImportResponse(first)

			// A reconcile opens before the client retries a request whose response it lost.
			sub := &models.InventorySubmission{
				InventoryID: inventory.ID, SubmissionType: models.InventorySubmissionTypeReconcile,
				ProcessingStatus: models.InventorySubmissionStatusPending,
				ApprovalStatus:   models.InventorySubmissionApprovalStatusPending,
				ReconcileStatus:  models.ReconcileLifecycleStatusOpen,
			}
			Expect(db.Create(sub).Error).NotTo(HaveOccurred())
			DeferCleanup(func() { db.Unscoped().Delete(sub) })

			replay := runImport(data, "false", testutil.WithHeader("Idempotency-Key", key))
			Expect(replay.StatusCode).To(Equal(http.StatusOK),
				"a committed key writes nothing, so the reconcile guard has nothing to protect")
			Expect(decodeImportResponse(replay).Rows).To(Equal(firstBody.Rows))

			// A genuinely new load is still refused while that reconcile is open.
			fresh := runImport(data, "false", testutil.WithHeader("Idempotency-Key", uuid.NewString()))
			Expect(fresh.StatusCode).To(Equal(http.StatusConflict))
			Expect(testutil.ParseResponse(fresh)["key"]).To(Equal(pkg.ErrKeyInitialStockReconcileOpen))
		})

		// The committed-key contract is that the original result comes back. Anything
		// describing CURRENT state must therefore sit behind the receipt lookup, since
		// a replay writes nothing and has nothing to guard against.
		It("replays a committed key even after the inventory is deactivated", func() {
			db := tenv.ContextfulDB()
			name := fmt.Sprintf("REPLAYINACTIVE %s", suffix)
			cleanupLoaded(name)
			data := singleSheet([]fixture.InitialStockRowSpec{
				{Name: name, Unit: unit.Name, Quantity: "6", Category: "NƯỚC"},
			})
			key := uuid.NewString()

			first := runImport(data, "false", testutil.WithHeader("Idempotency-Key", key))
			Expect(first.StatusCode).To(Equal(http.StatusOK))
			firstBody := decodeImportResponse(first)

			Expect(db.Model(&models.Inventory{}).Where("id = ?", inventory.ID).
				Update("status", models.InventoryStatusInactive).Error).NotTo(HaveOccurred())

			replay := runImport(data, "false", testutil.WithHeader("Idempotency-Key", key))
			Expect(replay.StatusCode).To(Equal(http.StatusOK),
				"a committed key must replay regardless of the inventory's current state")
			Expect(decodeImportResponse(replay).Rows).To(Equal(firstBody.Rows))

			// A genuinely new load into an inactive inventory is still refused.
			fresh := runImport(data, "false", testutil.WithHeader("Idempotency-Key", uuid.NewString()))
			Expect(fresh.StatusCode).To(Equal(http.StatusNotFound))
			Expect(testutil.ParseResponse(fresh)["key"]).To(Equal(pkg.ErrKeyInitialStockInventoryNotFound))

			// And a dry run against it is refused too.
			dry := runImport(data, "true")
			Expect(dry.StatusCode).To(Equal(http.StatusNotFound))
			Expect(testutil.ParseResponse(dry)["key"]).To(Equal(pkg.ErrKeyInitialStockInventoryNotFound))
		})

		It("rejects an idempotency key longer than the column that stores it", func() {
			name := fmt.Sprintf("LONGKEY %s", suffix)
			cleanupLoaded(name)
			data := singleSheet([]fixture.InitialStockRowSpec{
				{Name: name, Unit: unit.Name, Quantity: "4", Category: "NƯỚC"},
			})

			resp := runImport(data, "false",
				testutil.WithHeader("Idempotency-Key", strings.Repeat("k", 256)))
			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest),
				"an oversized key must be a keyed 400, not a 500 from the receipt insert")
			Expect(testutil.ParseResponse(resp)["key"]).To(Equal(pkg.ErrKeyInitialStockKeyTooLong))

			db := tenv.ContextfulDB()
			var txns int64
			Expect(db.Model(&models.InventoryTransaction{}).
				Where("transaction_type = ?", models.InventoryTransactionTypeInitial).
				Count(&txns).Error).NotTo(HaveOccurred())
			Expect(txns).To(BeZero(), "rejected before any work begins")

			// The boundary value is accepted.
			ok := runImport(data, "false",
				testutil.WithHeader("Idempotency-Key", strings.Repeat("k", 255)))
			Expect(ok.StatusCode).To(Equal(http.StatusOK))
		})

		// VARCHAR(255) limits characters, not bytes, so the guard must count runes.
		// A byte count would reject keys that fit the column: Vietnamese characters
		// are up to 3 bytes each, so 128 of them exceed 255 bytes while using half
		// the column.
		It("accepts a multi-byte key that fits the column in characters but not in bytes", func() {
			db := tenv.ContextfulDB()
			name := fmt.Sprintf("MBKEY %s", suffix)
			cleanupLoaded(name)
			data := singleSheet([]fixture.InitialStockRowSpec{
				{Name: name, Unit: unit.Name, Quantity: "3", Category: "NƯỚC"},
			})

			// 200 characters, 400 bytes (Đ is U+0110, 2 bytes): well inside VARCHAR(255),
			// well past a byte check.
			multiByte := strings.Repeat("Đ", 200)
			Expect(utf8.RuneCountInString(multiByte)).To(Equal(200))
			Expect(len(multiByte)).To(BeNumerically(">", 255),
				"the fixture must actually exceed 255 bytes or it proves nothing")

			resp := runImport(data, "false", testutil.WithHeader("Idempotency-Key", multiByte))
			Expect(resp.StatusCode).To(Equal(http.StatusOK),
				"a key of 200 characters fits VARCHAR(255) and must be accepted")

			// It really was stored, and replays under the same key.
			var stored int64
			Expect(db.Model(&models.InitialStockImport{}).
				Where("inventory_id = ? AND idempotency_key = ?", inventory.ID, multiByte).
				Count(&stored).Error).NotTo(HaveOccurred())
			Expect(stored).To(Equal(int64(1)), "the multi-byte key must round-trip through the column")

			replay := runImport(data, "false", testutil.WithHeader("Idempotency-Key", multiByte))
			Expect(replay.StatusCode).To(Equal(http.StatusOK))

			// Over the character limit is still refused, multi-byte or not.
			over := runImport(data, "false",
				testutil.WithHeader("Idempotency-Key", strings.Repeat("Đ", 256)))
			Expect(over.StatusCode).To(Equal(http.StatusBadRequest))
			Expect(testutil.ParseResponse(over)["key"]).To(Equal(pkg.ErrKeyInitialStockKeyTooLong))
		})

		It("surfaces an already-loaded inventory as a dry-run blocking condition instead of an error", func() {
			name := fmt.Sprintf("BLOCK %s", suffix)
			cleanupLoaded(name)
			data := singleSheet([]fixture.InitialStockRowSpec{
				{Name: name, Unit: unit.Name, Quantity: "5", Category: "NƯỚC"},
			})
			Expect(runImport(data, "false", testutil.WithHeader("Idempotency-Key", uuid.NewString())).StatusCode).
				To(Equal(http.StatusOK))

			resp := runImport(data, "true")
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			body := decodeImportResponse(resp)
			Expect(body.Blocking).To(HaveLen(1))
			Expect(body.Blocking[0].Key).To(Equal(pkg.ErrKeyInitialStockAlreadyImported))
			Expect(body.Blocking[0].Message).NotTo(BeEmpty())
		})
	})

	Describe("row-level rejections", func() {
		It("rejects each bad row with a Vietnamese reason and its sheet row number, and writes nothing", func() {
			db := tenv.ContextfulDB()

			mismatchUnit := fixture.WithUnit(db, models.Unit{
				Name: fmt.Sprintf("ISIMM%s", suffix), Symbol: fmt.Sprintf("ISIMM%s", suffix),
				UnitType: "general", ConversionFactor: 1, Level: 1, DecimalPlaces: 2,
			})
			mismatchProduct := fixture.WithProduct(db, models.Product{
				Name: fmt.Sprintf("MISMATCH %s", suffix), UnitID: mismatchUnit.ID, Status: "active",
			})
			inactive := fixture.WithProduct(db, models.Product{
				Name: fmt.Sprintf("INACTIVE %s", suffix), UnitID: unit.ID, Status: "active",
			})
			inactiveItem := &models.InventoryItem{
				InventoryID: inventory.ID, ProductID: inactive.ID, UnitID: unit.ID,
				Quantity: decimal.Zero, Status: models.InventoryItemStatusInactive,
			}
			Expect(db.Create(inactiveItem).Error).NotTo(HaveOccurred())
			DeferCleanup(func() { db.Unscoped().Delete(inactiveItem) })

			// Two live products with the same name: unresolvable, never guessed.
			ambiguousName := fmt.Sprintf("AMBIG %s", suffix)
			for i := 0; i < 2; i++ {
				fixture.WithProduct(db, models.Product{Name: ambiguousName, UnitID: unit.ID, Status: "active"})
			}

			dupName := fmt.Sprintf("DUP %s", suffix)
			data := singleSheet([]fixture.InitialStockRowSpec{
				{Name: "", Unit: unit.Name, Quantity: "1", Category: "NƯỚC"},                                  // row 4
				{Name: fmt.Sprintf("TEXTQTY %s", suffix), Unit: unit.Name, Quantity: "abc", Category: "NƯỚC"}, // row 5
				{Name: fmt.Sprintf("NEGQTY %s", suffix), Unit: unit.Name, Quantity: "-3", Category: "NƯỚC"},   // row 6
				{Name: fmt.Sprintf("SCALE %s", suffix), Unit: unit.Name, Quantity: "1.234", Category: "NƯỚC"}, // row 7
				{Name: mismatchProduct.Name, Unit: unit.Name, Quantity: "2", Category: "NƯỚC"},                // row 8
				{Name: inactive.Name, Unit: unit.Name, Quantity: "2", Category: "NƯỚC"},                       // row 9
				{Name: ambiguousName, Unit: unit.Name, Quantity: "2", Category: "NƯỚC"},                       // row 10
				{Name: dupName, Unit: unit.Name, Quantity: "2", Category: "NƯỚC"},                             // row 11
				{Name: dupName, Unit: unit.Name, Quantity: "3", Category: "NƯỚC"},                             // row 12
			})

			// Dry run reports every rejection; a real run refuses the whole batch.
			dry := decodeImportResponse(runImport(data, "true"))
			Expect(dry.Errors).To(HaveLen(9))
			Expect(dry.Rows).To(BeEmpty())
			Expect(dry.RowsFailed).To(Equal(9))

			rowNumbers := make([]int, 0, len(dry.Errors))
			for _, e := range dry.Errors {
				rowNumbers = append(rowNumbers, e.Row)
				Expect(e.Location).To(Equal(fmt.Sprintf("row:%d", e.Row)), "location must be pinned to row:<n>")
				Expect(e.Message).NotTo(BeEmpty())
				Expect(e.Message).To(MatchRegexp(`[àáâãèéêìíòóôõùúýăđĩũơưạảấầắằẳẵặẹẻẽếềểễệỉịọỏốồổỗộớờởỡợụủứừửữựỳỵỷỹÀÁÂÃÈÉÊÌÍÒÓÔÕÙÚÝĂĐ]|không|Số|Tên|Đơn|Sản|Mục`),
					"row messages are Vietnamese and backend-owned: %q", e.Message)
			}
			Expect(rowNumbers).To(Equal([]int{4, 5, 6, 7, 8, 9, 10, 11, 12}), "errors are sorted by sheet row")

			apply := runImport(data, "false", testutil.WithHeader("Idempotency-Key", uuid.NewString()))
			Expect(apply.StatusCode).To(Equal(http.StatusBadRequest))
			applyBody := testutil.ParseResponse(apply)
			Expect(applyBody["code"]).To(Equal("validation"))
			locations, ok := applyBody["locations"].([]interface{})
			Expect(ok).To(BeTrue(), "a rejected apply must carry locations[]: %v", applyBody)
			Expect(locations).To(HaveLen(9))
			first, ok := locations[0].(map[string]interface{})
			Expect(ok).To(BeTrue())
			Expect(first["location"]).To(Equal("row:4"))
			Expect(first["row"]).To(BeNumerically("==", 4), "row must be a real int, not regex-only")

			var txns int64
			Expect(db.Model(&models.InventoryTransaction{}).
				Where("transaction_type = ?", models.InventoryTransactionTypeInitial).
				Count(&txns).Error).NotTo(HaveOccurred())
			Expect(txns).To(BeZero(), "a batch with any bad row is all-or-nothing")
		})

		// The preview merges rejected rows into the plan table, so an error entry that
		// carries only a row number and a reason renders with empty cells.
		It("carries the rejected row's sheet values, raw and never null", func() {
			textQty := fmt.Sprintf("RAWTEXT %s", suffix)
			noUnit := fmt.Sprintf("RAWNOUNIT %s", suffix)
			data := singleSheet([]fixture.InitialStockRowSpec{
				{Name: "", Unit: unit.Name, Quantity: "7", Category: "NƯỚC"},     // row 4: blank name cell
				{Name: textQty, Unit: unit.Name, Quantity: "1,5x", Category: ""}, // row 5: unparseable quantity
				{Name: noUnit, Unit: "", Quantity: "", Category: "NƯỚC"},         // row 6: blank unit and quantity
			})

			dry := decodeImportResponse(runImport(data, "true"))
			Expect(dry.Rows).To(BeEmpty())
			Expect(dry.Errors).To(HaveLen(3))

			Expect(dry.Errors[0].Row).To(Equal(4))
			Expect(dry.Errors[0].Name).To(Equal(""), "a blank cell is an empty string, not null")
			Expect(dry.Errors[0].Unit).To(Equal(unit.Name))
			Expect(dry.Errors[0].Quantity).To(Equal("7"))
			Expect(dry.Errors[0].Message).To(Equal(pkg.RowMessage(pkg.ErrKeyInitialStockRowNameRequired)))

			Expect(dry.Errors[1].Row).To(Equal(5))
			Expect(dry.Errors[1].Name).To(Equal(textQty))
			Expect(dry.Errors[1].Unit).To(Equal(unit.Name))
			Expect(dry.Errors[1].Quantity).To(Equal("1,5x"),
				"the cell is echoed as read, so the operator sees what to fix")

			Expect(dry.Errors[2].Row).To(Equal(6))
			Expect(dry.Errors[2].Name).To(Equal(noUnit))
			Expect(dry.Errors[2].Unit).To(Equal(""))
			Expect(dry.Errors[2].Quantity).To(Equal(""))

			// The values must be real JSON strings on every entry, never absent or null.
			body, err := io.ReadAll(runImport(data, "true").Body)
			Expect(err).NotTo(HaveOccurred())
			var raw struct {
				Errors []map[string]json.RawMessage `json:"errors"`
			}
			Expect(json.Unmarshal(body, &raw)).To(Succeed())
			Expect(raw.Errors).To(HaveLen(3))
			for i, entry := range raw.Errors {
				for _, key := range []string{"name", "unit", "quantity"} {
					Expect(string(entry[key])).To(HavePrefix(`"`),
						"errors[%d].%s must be a JSON string: %s", i, key, string(body))
				}
			}
		})
	})

	// Every value is checked against the width of the column that will store it, in
	// the PLAN phase, so a dry-run can never report a row as loadable that would then
	// fail the INSERT with a bodyless 500.
	Describe("storage limits", func() {
		It("rejects values that exceed their column, identically in dry-run and apply", func() {
			db := tenv.ContextfulDB()

			// An item already near the DECIMAL(10,2) ceiling, so the row only overflows
			// once added to the existing on-hand.
			nearMax := fixture.WithProduct(db, models.Product{
				Name: fmt.Sprintf("NEARMAX %s", suffix), UnitID: unit.ID, Status: "active",
			})
			nearMaxItem := &models.InventoryItem{
				InventoryID: inventory.ID, ProductID: nearMax.ID, UnitID: unit.ID,
				Quantity: decimal.RequireFromString("99999990"), Status: models.InventoryItemStatusActive,
			}
			Expect(db.Create(nearMaxItem).Error).NotTo(HaveOccurred())
			DeferCleanup(func() {
				db.Unscoped().Where("inventory_item_id = ?", nearMaxItem.ID).Delete(&models.InventoryTransaction{})
				db.Unscoped().Delete(nearMaxItem)
			})

			data := singleSheet([]fixture.InitialStockRowSpec{
				// quantity beyond DECIMAL(10,2)
				{Name: fmt.Sprintf("BIGQTY %s", suffix), Unit: unit.Name, Quantity: "12345678901", Category: "NƯỚC"},
				// fits alone, overflows once added to on-hand
				{Name: nearMax.Name, Unit: unit.Name, Quantity: "50", Category: "NƯỚC"},
				// unit label beyond units.symbol VARCHAR(20)
				{Name: fmt.Sprintf("LONGUNIT %s", suffix), Unit: "THUNGHATNHUACUCLONVOCUNG25KG", Quantity: "1", Category: "NƯỚC"},
				// product name beyond products.name VARCHAR(255)
				{Name: strings.Repeat("Đ", 300), Unit: unit.Name, Quantity: "1", Category: "NƯỚC"},
			})

			expectedKeys := []string{
				pkg.ErrKeyInitialStockRowQuantityTooLarge,
				pkg.ErrKeyInitialStockRowResultTooLarge,
				pkg.ErrKeyInitialStockRowUnitTooLong,
				pkg.ErrKeyInitialStockRowNameTooLong,
			}
			expectedMessages := make([]string, 0, len(expectedKeys))
			for _, key := range expectedKeys {
				Expect(pkg.GetErrorMessageByLang(key, pkg.LangVI)).NotTo(BeEmpty(),
					"row message key %q must exist in the catalog", key)
				expectedMessages = append(expectedMessages, pkg.GetErrorMessageByLang(key, pkg.LangVI))
			}

			// The preview must show all four, and must not claim any of them loadable.
			dry := decodeImportResponse(runImport(data, "true"))
			Expect(dry.Rows).To(BeEmpty(), "no row here is loadable, so the plan must be empty")
			Expect(dry.Errors).To(HaveLen(4))
			Expect(dry.RowsFailed).To(Equal(4))
			Expect(dry.TransactionsCreated).To(BeZero(),
				"a preview must never report a transaction for a row that cannot be stored")
			Expect(dry.UnitsCreated).To(BeZero(), "an over-length unit label must not be reported as creatable")

			rows := make([]int, 0, len(dry.Errors))
			for i, e := range dry.Errors {
				rows = append(rows, e.Row)
				// Messages come from the catalog, so a format-string change cannot silently
				// swap one reason for another.
				prefix := strings.SplitN(expectedMessages[i], "%", 2)[0]
				Expect(e.Message).To(HavePrefix(prefix),
					"row %d reason should be %q-shaped, got %q", e.Row, expectedMessages[i], e.Message)
			}
			Expect(rows).To(Equal([]int{4, 5, 6, 7}))

			// The apply agrees with the preview: a keyed 400 batch, no 500, nothing written.
			apply := runImport(data, "false", testutil.WithHeader("Idempotency-Key", uuid.NewString()))
			Expect(apply.StatusCode).To(Equal(http.StatusBadRequest),
				"an unstorable row must be a structured 400, never a bodyless 500")
			body := testutil.ParseResponse(apply)
			Expect(body["code"]).To(Equal("validation"))
			Expect(body["locations"]).To(HaveLen(4))

			var reloaded models.InventoryItem
			Expect(db.First(&reloaded, nearMaxItem.ID).Error).NotTo(HaveOccurred())
			Expect(reloaded.Quantity.Equal(decimal.RequireFromString("99999990"))).To(BeTrue(),
				"on-hand must be untouched, got %s", reloaded.Quantity)

			var txns int64
			Expect(db.Model(&models.InventoryTransaction{}).
				Where("transaction_type = ?", models.InventoryTransactionTypeInitial).
				Count(&txns).Error).NotTo(HaveOccurred())
			Expect(txns).To(BeZero())
		})

		It("accepts the largest storable quantity", func() {
			name := fmt.Sprintf("MAXQTY %s", suffix)
			cleanupLoaded(name)
			data := singleSheet([]fixture.InitialStockRowSpec{
				{Name: name, Unit: unit.Name, Quantity: "99999999.99", Category: "NƯỚC"},
			})
			resp := runImport(data, "false", testutil.WithHeader("Idempotency-Key", uuid.NewString()))
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(decodeImportResponse(resp).TotalQuantity).To(Equal("99999999.99"))
		})
	})

	// Every VARCHAR guard in this path must count characters, because that is what
	// VARCHAR(n) limits. A byte count is wrong in the rejecting direction: it refuses
	// values that fit the column. Only the idempotency key had boundary coverage, so
	// the other guards were correct but unpinned — the exact state the key was in when
	// it shipped byte-counted. Reverting any one of these to len() must fail here.
	Describe("rune-counted length guards", func() {
		// multiByte is 2 bytes per character, so any value longer than half a limit
		// exceeds that limit in bytes while still fitting it in characters.
		const multiByte = "Đ"

		type guardCase struct {
			what string
			// accept fits the column in characters but exceeds it in bytes.
			acceptChars int
			refuseChars int
			// message is the exact catalog row message expected for refuseChars,
			// or "" when the guard refuses at the request level instead.
			message func(value string, chars int) string
		}

		It("counts characters, not bytes, for every guarded value", func() {
			db := tenv.ContextfulDB()

			cases := []guardCase{
				{
					what: "product name", acceptChars: 200, refuseChars: 256,
					message: func(_ string, chars int) string {
						return pkg.RowMessage(pkg.ErrKeyInitialStockRowNameTooLong,
							chars, initialStockProductNameChars)
					},
				},
				{
					// Create path: the label becomes name AND symbol, so the narrower
					// units.symbol VARCHAR(20) governs. refuseChars is 60 deliberately —
					// 60 runes is 120 bytes, so a byte-counting syntax guard would refuse
					// it citing the 100 name limit instead, and the exact-equality
					// assertion below detects that. A 21-character case would not: it
					// produces the same message either way.
					what: "unit symbol", acceptChars: 15, refuseChars: 60,
					message: func(value string, chars int) string {
						return pkg.RowMessage(pkg.ErrKeyInitialStockRowUnitTooLong,
							value, chars, initialStockUnitSymbolChars)
					},
				},
				{
					// Past units.name VARCHAR(100) the syntax guard reports that limit.
					what: "unit name", acceptChars: 15, refuseChars: 150,
					message: func(value string, chars int) string {
						return pkg.RowMessage(pkg.ErrKeyInitialStockRowUnitTooLong,
							value, chars, initialStockUnitNameChars)
					},
				},
				{
					what: "product type", acceptChars: 15, refuseChars: 21,
					message: func(value string, _ int) string {
						return pkg.RowMessage(pkg.ErrKeyInitialStockRowProductTypeTooLong, value)
					},
				},
			}

			for _, tc := range cases {
				accept := strings.Repeat(multiByte, tc.acceptChars)
				refuse := strings.Repeat(multiByte, tc.refuseChars)
				Expect(len(accept)).To(BeNumerically(">", tc.acceptChars),
					"%s: the accepted fixture must be multi-byte or it proves nothing", tc.what)

				build := func(value string) []fixture.InitialStockRowSpec {
					row := fixture.InitialStockRowSpec{
						Name:     fmt.Sprintf("RUNE %s %s", tc.what, suffix),
						Unit:     unit.Name,
						Quantity: "2",
						Category: "NƯỚC",
					}
					switch tc.what {
					case "product name":
						row.Name = value
					case "unit symbol", "unit name":
						row.Unit = value
					case "product type":
						row.Category = value
					}
					return []fixture.InitialStockRowSpec{row}
				}

				// Fits the column in characters: must be planned, not rejected.
				ok := decodeImportResponse(runImport(singleSheet(build(accept)), "true"))
				Expect(ok.Errors).To(BeEmpty(),
					"%s: a %d-character (%d-byte) value fits its column and must be accepted, got %v",
					tc.what, tc.acceptChars, len(accept), ok.Errors)
				Expect(ok.Rows).To(HaveLen(1), "%s: the row must be planned", tc.what)

				// Past the character limit: refused, with that guard's own message.
				bad := decodeImportResponse(runImport(singleSheet(build(refuse)), "true"))
				Expect(bad.Rows).To(BeEmpty(), "%s: the row must not be planned", tc.what)
				Expect(bad.Errors).To(HaveLen(1), "%s: expected exactly one row error", tc.what)
				Expect(bad.Errors[0].Row).To(Equal(4))
				Expect(bad.Errors[0].Message).To(Equal(tc.message(refuse, tc.refuseChars)),
					"%s: refusal must carry its own keyed message", tc.what)
			}

			// Nothing above wrote anything: all of it ran as dry runs.
			var txns int64
			Expect(db.Model(&models.InventoryTransaction{}).
				Where("transaction_type = ?", models.InventoryTransactionTypeInitial).
				Count(&txns).Error).NotTo(HaveOccurred())
			Expect(txns).To(BeZero())
		})
	})

	// Every way a sheet ĐVT label can map onto the unit table. Three consecutive
	// review passes found defects here, so the mapping is modelled explicitly rather
	// than discovered one case at a time.
	Describe("unit resolution", func() {
		var db *gorm.DB
		BeforeEach(func() { db = tenv.ContextfulDB() })

		// newUnit creates a general unit outside the import path, where name and symbol
		// can differ and need not be normalized.
		newUnit := func(name, symbol string) *models.Unit {
			u := &models.Unit{
				Name: name, Symbol: symbol, UnitType: "general",
				ConversionFactor: 1, Level: 1, DecimalPlaces: 2,
			}
			Expect(db.Create(u).Error).NotTo(HaveOccurred())
			DeferCleanup(func() { db.Exec("DELETE FROM units WHERE id = ?", u.ID) })
			return u
		}

		// plan dry-runs one row with the given unit label and returns the response.
		plan := func(label string) *dto.InitialStockImportResponse {
			return decodeImportResponse(runImport(singleSheet([]fixture.InitialStockRowSpec{
				{Name: fmt.Sprintf("UR %s %s", label, suffix), Unit: label, Quantity: "2", Category: "NƯỚC"},
			}), "true"))
		}
		unitsCreatedBy := func(label string) int {
			resp := plan(label)
			Expect(resp.Errors).To(BeEmpty(), "label %q must resolve, got %v", label, resp.Errors)
			Expect(resp.Rows).To(HaveLen(1))
			return resp.UnitsCreated
		}
		// rejectionFor2 dry-runs one row naming an explicit product and unit label.
		rejectionFor2 := func(productName, label string) string {
			resp := decodeImportResponse(runImport(singleSheet([]fixture.InitialStockRowSpec{
				{Name: productName, Unit: label, Quantity: "3", Category: "NƯỚC"},
			}), "true"))
			Expect(resp.Rows).To(BeEmpty(), "row must be refused")
			Expect(resp.Errors).To(HaveLen(1))
			return resp.Errors[0].Message
		}
		rejectionFor := func(label string) string {
			resp := plan(label)
			Expect(resp.Rows).To(BeEmpty(), "label %q must be refused", label)
			Expect(resp.Errors).To(HaveLen(1))
			return resp.Errors[0].Message
		}

		It("creates a unit when nothing matches", func() {
			Expect(unitsCreatedBy(fmt.Sprintf("URNONE%s", suffix))).To(Equal(1))
		})

		It("reuses a single match, whether it matches on name or on symbol", func() {
			byName := newUnit(fmt.Sprintf("URNAME%s", suffix), fmt.Sprintf("URSYM%s", suffix))
			Expect(unitsCreatedBy(byName.Name)).To(BeZero(), "matched by name")
			Expect(unitsCreatedBy(byName.Symbol)).To(BeZero(), "matched by symbol")

			// A unit whose name and symbol normalize alike answers once, not twice, so it
			// must not read as ambiguous.
			same := newUnit(fmt.Sprintf("URSAME%s", suffix), fmt.Sprintf("URSAME%s", suffix))
			Expect(unitsCreatedBy(same.Name)).To(BeZero(), "name == symbol is one match, not two")
		})

		It("matches case-insensitively but never folds diacritics", func() {
			mixed := newUnit(fmt.Sprintf("Ur%s", suffix), fmt.Sprintf("ur%s", suffix))
			Expect(unitsCreatedBy(strings.ToUpper(mixed.Name))).To(BeZero(),
				"a stored mixed-case unit must be reused")

			// CUỐN and CUỘN differ only by diacritic and must stay distinct units.
			withDiacritic := newUnit(fmt.Sprintf("CUỐN%s", suffix), fmt.Sprintf("CUỐN%s", suffix))
			Expect(unitsCreatedBy(fmt.Sprintf("CUỘN%s", suffix))).To(Equal(1),
				"a diacritic variant of %q must create its own unit", withDiacritic.Name)
			DeferCleanup(func() {
				db.Exec("DELETE FROM units WHERE unit_type = 'general' AND name = ?",
					fmt.Sprintf("CUỘN%s", suffix))
			})
		})

		// The corruption path: the schema constrains only (unit_type, name), so two live
		// units can answer to one label and picking either silently binds the product and
		// its inventory item to a unit the sheet never named.
		It("refuses an ambiguous label instead of choosing one", func() {
			label := fmt.Sprintf("URAMB%s", suffix)
			byName := newUnit(label, fmt.Sprintf("URAMBS%s", suffix))
			bySymbol := newUnit(fmt.Sprintf("URAMBN%s", suffix), label)
			Expect(byName.ID).NotTo(Equal(bySymbol.ID))

			Expect(rejectionFor(label)).To(Equal(
				pkg.RowMessage(pkg.ErrKeyInitialStockRowUnitAmbiguous, label, 2)),
				"one unit named %q and another with that symbol must be a row error", label)

			// Several matching on symbol alone is equally ambiguous; symbols are not
			// uniquely constrained at all.
			third := fmt.Sprintf("URSYMDUP%s", suffix)
			newUnit(fmt.Sprintf("URS1%s", suffix), third)
			newUnit(fmt.Sprintf("URS2%s", suffix), third)
			Expect(rejectionFor(third)).To(Equal(
				pkg.RowMessage(pkg.ErrKeyInitialStockRowUnitAmbiguous, third, 2)))
		})

		It("refuses a soft-deleted match, but prefers a live one over it", func() {
			label := fmt.Sprintf("URDEL%s", suffix)
			gone := newUnit(label, label)
			Expect(db.Delete(gone).Error).NotTo(HaveOccurred())
			Expect(rejectionFor(label)).To(Equal(
				pkg.RowMessage(pkg.ErrKeyInitialStockRowUnitSoftDeleted, label, gone.ID)))

			// With a live row answering the same label, the live row wins.
			live := newUnit(fmt.Sprintf("URLIVEN%s", suffix), label)
			Expect(unitsCreatedBy(label)).To(BeZero(), "the live unit %d must win", live.ID)
		})

		It("ignores a match of another unit_type and creates a general unit", func() {
			label := fmt.Sprintf("URTYPE%s", suffix)
			other := &models.Unit{
				Name: label, Symbol: label, UnitType: "weight",
				ConversionFactor: 1, Level: 1, DecimalPlaces: 2,
			}
			Expect(db.Create(other).Error).NotTo(HaveOccurred())
			DeferCleanup(func() { db.Exec("DELETE FROM units WHERE id = ?", other.ID) })

			Expect(unitsCreatedBy(label)).To(Equal(1),
				"a unit of another type must not satisfy a general label")
		})

		// A row whose product or item already has a governing unit never consults the
		// general units, so neither an ambiguous general label nor a soft-deleted general
		// unit answering it may reject that row. Both verdicts have an accept limb here
		// and a refuse limb above, so hoisting either back above the governing-unit
		// check — or dropping it — fails.
		It("ignores general-unit ambiguity and soft-deletes when a governing unit decides the row", func() {
			label := fmt.Sprintf("URGA%s", suffix)

			// The governing unit is a live unit of another type, so it is never one of
			// the general matches below.
			governing := &models.Unit{
				Name: label, Symbol: fmt.Sprintf("URGAS%s", suffix), UnitType: "mass",
				ConversionFactor: 1, Level: 1, DecimalPlaces: 2,
			}
			Expect(db.Create(governing).Error).NotTo(HaveOccurred())
			DeferCleanup(func() { db.Exec("DELETE FROM units WHERE id = ?", governing.ID) })
			product := fixture.WithProduct(db, models.Product{
				Name: fmt.Sprintf("URGAP %s", suffix), UnitID: governing.ID, Status: "active",
			})

			// Two live general units also answer the label: ambiguous, but irrelevant.
			newUnit(label, fmt.Sprintf("URGA1%s", suffix))
			newUnit(fmt.Sprintf("URGA2%s", suffix), label)

			resp := decodeImportResponse(runImport(singleSheet([]fixture.InitialStockRowSpec{
				{Name: product.Name, Unit: label, Quantity: "2", Category: "NƯỚC"},
			}), "true"))
			Expect(resp.Errors).To(BeEmpty(),
				"the governing unit decides this row, so general ambiguity cannot reject it: %v",
				resp.Errors)
			Expect(resp.Rows).To(HaveLen(1))
			Expect(resp.UnitsCreated).To(BeZero())

			// Same shape for a soft-deleted general unit answering the label.
			delLabel := fmt.Sprintf("URGD%s", suffix)
			governing2 := &models.Unit{
				Name: delLabel, Symbol: fmt.Sprintf("URGDS%s", suffix), UnitType: "mass",
				ConversionFactor: 1, Level: 1, DecimalPlaces: 2,
			}
			Expect(db.Create(governing2).Error).NotTo(HaveOccurred())
			DeferCleanup(func() { db.Exec("DELETE FROM units WHERE id = ?", governing2.ID) })
			product2 := fixture.WithProduct(db, models.Product{
				Name: fmt.Sprintf("URGDP %s", suffix), UnitID: governing2.ID, Status: "active",
			})
			gone := newUnit(delLabel, delLabel)
			Expect(db.Delete(gone).Error).NotTo(HaveOccurred())

			resp2 := decodeImportResponse(runImport(singleSheet([]fixture.InitialStockRowSpec{
				{Name: product2.Name, Unit: delLabel, Quantity: "2", Category: "NƯỚC"},
			}), "true"))
			Expect(resp2.Errors).To(BeEmpty(),
				"a soft-deleted general unit must not reject a row a live unit governs: %v",
				resp2.Errors)
			Expect(resp2.Rows).To(HaveLen(1))
			Expect(resp2.UnitsCreated).To(BeZero())
		})

		// Regression guard: the product and item lookups are Unscoped, and GORM
		// propagates that into Preload("Unit"), so a soft-deleted unit can arrive as the
		// governing unit and bypass the general-unit soft-delete verdict entirely.
		It("refuses a row whose governing unit is itself soft-deleted", func() {
			label := fmt.Sprintf("URSD%s", suffix)
			gone := newUnit(label, label)
			product := fixture.WithProduct(db, models.Product{
				Name: fmt.Sprintf("URSDP %s", suffix), UnitID: gone.ID, Status: "active",
			})
			Expect(db.Delete(gone).Error).NotTo(HaveOccurred())

			// Path 1: an existing product whose unit_id points at the deleted unit.
			Expect(rejectionFor2(product.Name, label)).To(Equal(
				pkg.RowMessage(pkg.ErrKeyInitialStockRowUnitSoftDeleted, gone.Name, gone.ID)),
				"new stock must never be denominated in a deleted unit")

			// The message must name the unit that actually blocked the row. This check
			// fires ahead of the label-match check, so the sheet label can name a
			// different unit entirely, and pairing that label with this id would send the
			// operator to inspect the wrong unit on a one-shot 68-row load. Asserted on
			// the rendered text, not just the format arguments, so a wrong argument
			// cannot agree with itself.
			decoyLabel := fmt.Sprintf("URSDDECOY%s", suffix)
			msg := rejectionFor2(product.Name, decoyLabel)
			Expect(msg).To(ContainSubstring(gone.Name),
				"the blocking unit must be named, got %q", msg)
			Expect(msg).To(ContainSubstring(fmt.Sprintf("%d", gone.ID)),
				"the blocking unit id must be present, got %q", msg)
			Expect(msg).NotTo(ContainSubstring(decoyLabel),
				"the sheet label names a different unit and must not be reported as the blocker, got %q", msg)

			// Path 2: an existing inventory item on the deleted unit.
			item := &models.InventoryItem{
				InventoryID: inventory.ID, ProductID: product.ID, UnitID: gone.ID,
				Quantity: decimal.NewFromInt(4), Status: models.InventoryItemStatusActive,
			}
			Expect(db.Create(item).Error).NotTo(HaveOccurred())
			DeferCleanup(func() {
				db.Unscoped().Where("inventory_item_id = ?", item.ID).Delete(&models.InventoryTransaction{})
				db.Unscoped().Delete(item)
			})
			Expect(rejectionFor2(product.Name, label)).To(Equal(
				pkg.RowMessage(pkg.ErrKeyInitialStockRowUnitSoftDeleted, gone.Name, gone.ID)),
				"on-hand must not be raised on an item denominated in a deleted unit")

			// And an apply writes nothing.
			apply := runImport(singleSheet([]fixture.InitialStockRowSpec{
				{Name: product.Name, Unit: label, Quantity: "5", Category: "NƯỚC"},
			}), "false", testutil.WithHeader("Idempotency-Key", uuid.NewString()))
			Expect(apply.StatusCode).To(Equal(http.StatusBadRequest))
			var reloaded models.InventoryItem
			Expect(db.First(&reloaded, item.ID).Error).NotTo(HaveOccurred())
			Expect(reloaded.Quantity.Equal(decimal.NewFromInt(4))).To(BeTrue(),
				"on-hand must be untouched, got %s", reloaded.Quantity)
		})

		// Every validation must fire where its constraint binds. "No general match" is
		// not "a unit will be created": a product or item can govern the row with a unit
		// of any type, in which case nothing is written to the units table at all.
		It("binds to a governing unit of another type without applying the create-path width", func() {
			longName := strings.Repeat("Đ", 60)
			Expect(utf8.RuneCountInString(longName)).To(BeNumerically(">", initialStockUnitSymbolChars))
			Expect(utf8.RuneCountInString(longName)).To(BeNumerically("<=", initialStockUnitNameChars))

			// A governing unit that is deliberately NOT of the general type the import
			// searches, so the label has zero general matches.
			mass := &models.Unit{
				Name: longName, Symbol: fmt.Sprintf("URM%s", suffix), UnitType: "mass",
				ConversionFactor: 1, Level: 1, DecimalPlaces: 2,
			}
			Expect(db.Create(mass).Error).NotTo(HaveOccurred())
			DeferCleanup(func() { db.Exec("DELETE FROM units WHERE id = ?", mass.ID) })

			existing := fixture.WithProduct(db, models.Product{
				Name: fmt.Sprintf("URGOV %s", suffix), UnitID: mass.ID, Status: "active",
			})

			resp := decodeImportResponse(runImport(singleSheet([]fixture.InitialStockRowSpec{
				{Name: existing.Name, Unit: longName, Quantity: "2", Category: "NƯỚC"},
			}), "true"))
			Expect(resp.Errors).To(BeEmpty(),
				"a row governed by an existing unit writes no symbol, so the 20-character "+
					"create-path width must not apply: %v", resp.Errors)
			Expect(resp.Rows).To(HaveLen(1))
			Expect(resp.UnitsCreated).To(BeZero(), "the governing unit must be reused, not recreated")
			Expect(resp.Rows[0].Actions).To(ContainElement("match_product"))

			// The identical label with no product to govern it does create, and is
			// refused at the symbol width.
			refused := decodeImportResponse(runImport(singleSheet([]fixture.InitialStockRowSpec{
				{Name: fmt.Sprintf("URNOGOV %s", suffix), Unit: longName, Quantity: "2", Category: "NƯỚC"},
			}), "true"))
			Expect(refused.Rows).To(BeEmpty())
			Expect(refused.Errors).To(HaveLen(1))
			Expect(refused.Errors[0].Message).To(Equal(pkg.RowMessage(
				pkg.ErrKeyInitialStockRowUnitTooLong, longName, 60, initialStockUnitSymbolChars)),
				"with nothing governing it the same label must be refused on the create path")
		})

		// products.product_type is written only on create, so an existing product's row
		// must not be refused for a category too wide for a column it never reaches.
		It("does not apply the product_type width to a row matching an existing product", func() {
			existing := fixture.WithProduct(db, models.Product{
				Name: fmt.Sprintf("URPT %s", suffix), UnitID: unit.ID, Status: "active",
			})
			wide := strings.Repeat("X", 25)

			resp := decodeImportResponse(runImport(singleSheet([]fixture.InitialStockRowSpec{
				{Name: existing.Name, Unit: unit.Name, Quantity: "2", Category: wide},
			}), "true"))
			Expect(resp.Errors).To(BeEmpty(),
				"an existing product's type is never rewritten, so the width cannot bind: %v", resp.Errors)
			Expect(resp.Rows).To(HaveLen(1))

			// A new product does write it, and is refused.
			refused := decodeImportResponse(runImport(singleSheet([]fixture.InitialStockRowSpec{
				{Name: fmt.Sprintf("URPTNEW %s", suffix), Unit: unit.Name, Quantity: "2", Category: wide},
			}), "true"))
			Expect(refused.Rows).To(BeEmpty())
			Expect(refused.Errors).To(HaveLen(1))
			Expect(refused.Errors[0].Message).To(Equal(
				pkg.RowMessage(pkg.ErrKeyInitialStockRowProductTypeTooLong, wide)))
		})

		// Codex finding: the 20-character symbol width is a create-path constraint, so a
		// long name that already exists must be usable. 60 runes is 120 bytes, so this
		// also detects a byte-counting syntax guard, which would refuse it citing 100.
		It("accepts a long existing unit name without applying the symbol width", func() {
			longName := strings.Repeat("Đ", 60)
			Expect(utf8.RuneCountInString(longName)).To(BeNumerically("<=", initialStockUnitNameChars))
			Expect(len(longName)).To(BeNumerically(">", initialStockUnitNameChars),
				"the fixture must exceed the name limit in bytes or it proves nothing")

			existing := newUnit(longName, fmt.Sprintf("URLONG%s", suffix))
			Expect(unitsCreatedBy(longName)).To(BeZero(),
				"a %d-character existing unit name fits units.name and writes no symbol",
				utf8.RuneCountInString(existing.Name))
		})
	})

	Describe("unit matching", func() {
		It("matches an existing unit whose stored name differs only in case", func() {
			db := tenv.ContextfulDB()
			// Units created outside the import path can hold mixed case.
			mixed := &models.Unit{
				Name: fmt.Sprintf("Kg%s", suffix), Symbol: fmt.Sprintf("kg%s", suffix),
				UnitType: "general", ConversionFactor: 1, Level: 1, DecimalPlaces: 2,
			}
			Expect(db.Create(mixed).Error).NotTo(HaveOccurred())
			DeferCleanup(func() { db.Exec("DELETE FROM units WHERE id = ?", mixed.ID) })

			name := fmt.Sprintf("CASEUNIT %s", suffix)
			cleanupLoaded(name)
			data := singleSheet([]fixture.InitialStockRowSpec{
				{Name: name, Unit: strings.ToUpper(mixed.Name), Quantity: "4", Category: "NƯỚC"},
			})

			resp := runImport(data, "false", testutil.WithHeader("Idempotency-Key", uuid.NewString()))
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(decodeImportResponse(resp).UnitsCreated).To(BeZero(),
				"an existing unit differing only in case must be reused, not duplicated")

			var count int64
			Expect(db.Model(&models.Unit{}).Where("unit_type = ? AND UPPER(TRIM(name)) = ?",
				"general", strings.ToUpper(mixed.Name)).Count(&count).Error).NotTo(HaveOccurred())
			Expect(count).To(Equal(int64(1)), "no duplicate upper-case unit may be created")

			var item models.InventoryItem
			Expect(db.Joins("JOIN products p ON p.id = inventory_items.product_id").
				Where("inventory_items.inventory_id = ? AND UPPER(TRIM(p.name)) = ?",
					inventory.ID, strings.ToUpper(name)).First(&item).Error).NotTo(HaveOccurred())
			Expect(item.UnitID).To(Equal(mixed.ID), "the new item must bind to the existing unit")
		})
	})

	Describe("malformed input", func() {
		It("returns a keyed structured error, never a 500 and never a panic", func() {
			good := singleSheet([]fixture.InitialStockRowSpec{
				{Name: fmt.Sprintf("MAL %s", suffix), Unit: unit.Name, Quantity: "1", Category: "NƯỚC"},
			})

			By("unknown sheet name")
			resp := post(developer, initialStockImportPath, initialStockForm(good, inventory.ID, "NOPE", "true"))
			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
			Expect(testutil.ParseResponse(resp)["key"]).To(Equal(pkg.ErrKeyInitialStockSheetNotFound))

			By("header mismatch")
			wrongHeader := fixture.CreateInitialStockWorkbook([]fixture.InitialStockSheetSpec{{
				Name:   "TON",
				Header: []string{"A", "B", "C", "D"},
				Rows:   fixture.InitialStockRows("X", unit.Name, []string{"1"}, "NƯỚC"),
			}})
			resp = post(developer, initialStockImportPath, initialStockForm(wrongHeader, inventory.ID, "TON", "true"))
			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
			Expect(testutil.ParseResponse(resp)["key"]).To(Equal(pkg.ErrKeyInitialStockHeaderMismatch))

			By("no data rows")
			emptySheet := fixture.CreateInitialStockWorkbook([]fixture.InitialStockSheetSpec{{Name: "TON"}})
			resp = post(developer, initialStockImportPath, initialStockForm(emptySheet, inventory.ID, "TON", "true"))
			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
			Expect(testutil.ParseResponse(resp)["key"]).To(Equal(pkg.ErrKeyInitialStockNoDataRows))

			By("non-xlsx extension")
			form := initialStockForm(good, inventory.ID, "TON", "true")
			form.Files = initialStockFile(good, "stock.csv")
			resp = post(developer, initialStockImportPath, form)
			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
			Expect(testutil.ParseResponse(resp)["key"]).To(Equal(pkg.ErrKeyInitialStockInvalidFileType))

			By("xls, which excelize cannot read")
			form = initialStockForm(good, inventory.ID, "TON", "true")
			form.Files = initialStockFile(good, "stock.xls")
			resp = post(developer, initialStockImportPath, form)
			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
			Expect(testutil.ParseResponse(resp)["key"]).To(Equal(pkg.ErrKeyInitialStockInvalidFileType))

			By("empty file")
			form = initialStockForm(good, inventory.ID, "TON", "true")
			form.Files = initialStockFile([]byte{}, "stock.xlsx")
			resp = post(developer, initialStockImportPath, form)
			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
			Expect(testutil.ParseResponse(resp)["key"]).To(Equal(pkg.ErrKeyInitialStockEmptyFile))

			By("a renamed non-zip, which must be recovered rather than panic")
			form = initialStockForm(good, inventory.ID, "TON", "true")
			form.Files = initialStockFile([]byte("this is definitely not a zip archive"), "stock.xlsx")
			resp = post(developer, initialStockImportPath, form)
			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
			Expect(testutil.ParseResponse(resp)["key"]).To(Equal(pkg.ErrKeyInitialStockParseFailed))

			By("oversized upload")
			oversized := bytes.Repeat([]byte("x"), (10<<20)+1024)
			form = initialStockForm(oversized, inventory.ID, "TON", "true")
			resp = post(developer, initialStockImportPath, form)
			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest),
				"an oversized upload must be a keyed 400, never the generic 500 BodyLimit would produce")
			Expect(testutil.ParseResponse(resp)["key"]).To(Equal(pkg.ErrKeyInitialStockFileTooLarge))

			By("unknown inventory")
			resp = post(developer, initialStockImportPath, initialStockForm(good, 99999999, "TON", "true"))
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
			Expect(testutil.ParseResponse(resp)["key"]).To(Equal(pkg.ErrKeyInitialStockInventoryNotFound))

			By("missing inventory_id")
			form = initialStockForm(good, inventory.ID, "TON", "true")
			delete(form.Fields, "inventory_id")
			resp = post(developer, initialStockImportPath, form)
			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
			Expect(testutil.ParseResponse(resp)["key"]).To(Equal(pkg.ErrKeyInitialStockInventoryRequired))

			By("missing sheet_name")
			form = initialStockForm(good, inventory.ID, "TON", "true")
			delete(form.Fields, "sheet_name")
			resp = post(developer, initialStockImportPath, form)
			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
			Expect(testutil.ParseResponse(resp)["key"]).To(Equal(pkg.ErrKeyInitialStockSheetNameRequired))
		})
	})
})
