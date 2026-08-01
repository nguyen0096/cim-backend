package services

import (
	"context"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/shopspring/decimal"

	"cim-backend/internal/models"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
)

// planRow is one accepted sheet row with everything resolved against the DB.
type planRow struct {
	src        sheetRow
	quantity   decimal.Decimal
	product    *models.Product
	productNew bool
	unit       *models.Unit
	item       *models.InventoryItem
	itemNew    bool
	currentQty decimal.Decimal
	unitDP     int
	actions    []string
}

// importPlan is the resolved, write-ready form of an accepted sheet.
type importPlan struct {
	inventoryID  uint
	rowsRead     int
	rows         []*planRow
	newUnits     []*models.Unit
	newProducts  []*models.Product
	productTypes []string
}

// rowRejections accumulates per-row failures in sheet order, in the single row
// shape shared by the dry-run response and BatchError.Locations.
type rowRejections struct {
	failed map[int]bool
	locs   []pkg.BatchErrorLocation
}

func newRowRejections() *rowRejections {
	return &rowRejections{failed: make(map[int]bool)}
}

// add records the first rejection for a row; later ones are dropped so a row
// yields exactly one reason.
func (r *rowRejections) add(row int, key string, args ...interface{}) {
	if r.failed[row] {
		return
	}
	r.failed[row] = true
	r.locs = append(r.locs, pkg.NewRowLocation(row, pkg.RowMessage(key, args...)))
}

func (r *rowRejections) sorted() []pkg.BatchErrorLocation {
	sort.SliceStable(r.locs, func(i, j int) bool { return r.locs[i].Row < r.locs[j].Row })
	if r.locs == nil {
		return []pkg.BatchErrorLocation{}
	}
	return r.locs
}

// buildPlan resolves every row against the DB without writing. Callers inside a
// transaction get transaction-scoped reads, which is what makes the hydrated
// existing items safe to write back.
func (s *initialStockImportService) buildPlan(
	ctx context.Context,
	inventoryID uint,
	rows []sheetRow,
) (*importPlan, []pkg.BatchErrorLocation, error) {
	rej := newRowRejections()
	plan := &importPlan{inventoryID: inventoryID, rowsRead: len(rows)}

	quantities := s.validateRowSyntax(rows, rej)

	unitsByLabel, err := s.resolveUnits(ctx, rows, rej)
	if err != nil {
		return nil, nil, err
	}
	productsByKey, err := s.resolveProducts(ctx, rows, rej)
	if err != nil {
		return nil, nil, err
	}
	itemsByProduct, err := s.resolveItems(ctx, inventoryID, rows, productsByKey, rej)
	if err != nil {
		return nil, nil, err
	}

	newUnits := make(map[string]*models.Unit)
	newProducts := make(map[string]*models.Product)
	productTypes := make(map[string]string)

	for _, row := range rows {
		if rej.failed[row.SheetRow] {
			continue
		}
		nameKey := upperTrim(row.Name)
		unitLabel := upperTrim(row.Unit)

		matched := productsByKey[nameKey]
		item := itemsByProduct[nameKey]

		// products.product_type is written only when the product is created; an
		// existing product's type is never rewritten by this tool.
		if matched == nil {
			if n := utf8.RuneCountInString(row.ProductType); n > initialStockProductTypeMaxLen {
				rej.add(row.SheetRow, pkg.ErrKeyInitialStockRowProductTypeTooLong, row.ProductType)
				continue
			}
		}

		// The quantity lands on the item's balance, and inventory_items.unit_id is
		// independently mutable, so an existing item's unit governs; otherwise the
		// matched product's does.
		var governing *models.Unit
		switch {
		case item != nil && item.Unit != nil:
			governing = item.Unit
		case matched != nil && matched.Unit != nil:
			governing = matched.Unit
		}
		// The product and item lookups are Unscoped so a deleted row can be reported
		// rather than silently skipped, and GORM propagates that into Preload("Unit"),
		// so a soft-deleted unit can arrive here. Adopting it would denominate new stock
		// in a deleted unit and bypass the soft-delete verdict below, which only covers
		// the general-unit path.
		// Names the governing unit, not the sheet label: this fires ahead of the
		// label-match check, so the two can be different units and pairing the label
		// with this id would send the operator to inspect the wrong one.
		if governing != nil && governing.DeletedAt.Valid {
			rej.add(row.SheetRow, pkg.ErrKeyInitialStockRowUnitSoftDeleted, governing.Name, governing.ID)
			continue
		}
		if governing != nil && !unitLabelMatches(unitLabel, governing) {
			rej.add(row.SheetRow, pkg.ErrKeyInitialStockRowUnitMismatch, row.Unit, governing.Name)
			continue
		}

		// Only a row with no governing unit consults the general units, so their
		// verdicts — ambiguity, a soft-deleted blocker, and the symbol width that binds
		// on creation — apply here and nowhere earlier. A row that resolves through a
		// governing unit of any unit_type writes nothing to the units table.
		effective := governing
		if effective == nil {
			lookup := unitsByLabel[unitLabel]
			switch {
			case lookup.ambiguous > 0:
				rej.add(row.SheetRow, pkg.ErrKeyInitialStockRowUnitAmbiguous, row.Unit, lookup.ambiguous)
				continue
			case lookup.match != nil:
				effective = lookup.match
			case lookup.deletedID != 0:
				rej.add(row.SheetRow, pkg.ErrKeyInitialStockRowUnitSoftDeleted, row.Unit, lookup.deletedID)
				continue
			default:
				// This run creates the unit with the label as both name and symbol, so
				// the narrower symbol column governs, and only now.
				if n := utf8.RuneCountInString(row.Unit); n > initialStockUnitSymbolMaxLen {
					rej.add(row.SheetRow, pkg.ErrKeyInitialStockRowUnitTooLong,
						row.Unit, n, initialStockUnitSymbolMaxLen)
					continue
				}
			}
		}

		// A unit this run creates takes the units.decimal_places default of 2.
		unitDP := initialStockMaxDecimalPlaces
		if effective != nil {
			unitDP = effective.DecimalPlaces
		}
		allowed := unitDP
		if allowed > initialStockMaxDecimalPlaces {
			allowed = initialStockMaxDecimalPlaces
		}
		qty := quantities[row.SheetRow]
		if scale := pkg.DecimalPlaces(qty); scale > allowed {
			rej.add(row.SheetRow, pkg.ErrKeyInitialStockRowQuantityScale, scale, allowed)
			continue
		}

		// The load is additive, so the sum has to fit the column too, not just the row.
		current := decimal.Zero
		if item != nil {
			current = item.Quantity
		}
		if exceedsStorableQuantity(current.Add(qty)) {
			rej.add(row.SheetRow, pkg.ErrKeyInitialStockRowResultTooLarge, qty.String(), current.String())
			continue
		}

		pr := &planRow{src: row, quantity: qty, unitDP: allowed, item: item}

		if effective != nil {
			pr.unit = effective
		} else if pending, ok := newUnits[unitLabel]; ok {
			pr.unit = pending
		} else {
			created := &models.Unit{
				UnitType:         initialStockUnitType,
				Name:             unitLabel,
				Symbol:           unitLabel,
				ConversionFactor: 1,
			}
			newUnits[unitLabel] = created
			pr.unit = created
		}

		switch {
		case matched != nil:
			pr.product = matched
			pr.actions = append(pr.actions, dto.InitialStockActionMatchProduct)
		default:
			pending, ok := newProducts[nameKey]
			if !ok {
				pending = &models.Product{
					Name:        row.Name,
					ProductType: row.ProductType,
					Status:      "active",
				}
				newProducts[nameKey] = pending
			}
			pr.product = pending
			pr.productNew = true
			pr.actions = append(pr.actions, dto.InitialStockActionCreateProduct)
		}

		if item != nil {
			pr.currentQty = item.Quantity
		} else {
			pr.itemNew = true
			pr.actions = append(pr.actions, dto.InitialStockActionCreateItem)
		}

		if qty.IsPositive() {
			pr.actions = append(pr.actions, dto.InitialStockActionCreateTransaction)
		} else {
			pr.actions = append(pr.actions, dto.InitialStockActionSkipZeroQuantity)
		}

		if row.ProductType != "" {
			key := strings.ToLower(row.ProductType)
			if _, exists := productTypes[key]; !exists {
				productTypes[key] = row.ProductType
			}
		}

		plan.rows = append(plan.rows, pr)
	}

	plan.newUnits = orderedUnits(rows, newUnits)
	plan.newProducts = orderedProducts(rows, newProducts)
	for _, t := range productTypes {
		plan.productTypes = append(plan.productTypes, t)
	}
	sort.Strings(plan.productTypes)

	return plan, rej.sorted(), nil
}

// validateRowSyntax checks everything decidable from the sheet alone and returns
// the parsed quantity per accepted row.
func (s *initialStockImportService) validateRowSyntax(rows []sheetRow, rej *rowRejections) map[int]decimal.Decimal {
	nameCounts := make(map[string]int)
	for _, row := range rows {
		if key := upperTrim(row.Name); key != "" {
			nameCounts[key]++
		}
	}

	quantities := make(map[int]decimal.Decimal, len(rows))
	for _, row := range rows {
		if row.Name == "" {
			rej.add(row.SheetRow, pkg.ErrKeyInitialStockRowNameRequired)
			continue
		}
		if nameCounts[upperTrim(row.Name)] > 1 {
			rej.add(row.SheetRow, pkg.ErrKeyInitialStockRowDuplicateName, row.Name)
			continue
		}
		if utf8.RuneCountInString(row.Name) > initialStockProductNameMaxLen {
			rej.add(row.SheetRow, pkg.ErrKeyInitialStockRowNameTooLong,
				utf8.RuneCountInString(row.Name), initialStockProductNameMaxLen)
			continue
		}
		if row.Unit == "" {
			rej.add(row.SheetRow, pkg.ErrKeyInitialStockRowUnitRequired)
			continue
		}
		// Only the name column is a syntax concern: a label this long can still name an
		// existing unit, which the unit API permits and which writes nothing to the
		// narrower symbol column. The symbol width is enforced in resolveUnits, on the
		// create path only.
		if n := utf8.RuneCountInString(row.Unit); n > initialStockUnitNameMaxLen {
			rej.add(row.SheetRow, pkg.ErrKeyInitialStockRowUnitTooLong, row.Unit, n, initialStockUnitNameMaxLen)
			continue
		}
		qty := decimal.Zero
		if row.RawQuantity != "" {
			parsed, err := decimal.NewFromString(row.RawQuantity)
			if err != nil {
				rej.add(row.SheetRow, pkg.ErrKeyInitialStockRowQuantityInvalid, row.RawQuantity)
				continue
			}
			if parsed.IsNegative() {
				rej.add(row.SheetRow, pkg.ErrKeyInitialStockRowQuantityNegative, parsed.String())
				continue
			}
			if exceedsStorableQuantity(parsed) {
				rej.add(row.SheetRow, pkg.ErrKeyInitialStockRowQuantityTooLarge,
					parsed.String(), initialStockMaxIntegerDigits)
				continue
			}
			qty = parsed
		}
		quantities[row.SheetRow] = qty
	}
	return quantities
}

// maxStorableQuantity is the largest value the DECIMAL(10,2) quantity columns hold.
var maxStorableQuantity = decimal.RequireFromString("99999999.99")

// exceedsStorableQuantity reports whether a value would overflow those columns.
// Postgres raises a numeric-overflow error, which surfaces as a bodyless 500.
func exceedsStorableQuantity(q decimal.Decimal) bool {
	return q.GreaterThan(maxStorableQuantity)
}

// unitLookup is what matching one sheet label against the general units found.
// It records the outcome rather than acting on it: whether the label is usable
// depends on facts this lookup does not have, because a row whose product or item
// already has a governing unit never consults the general units at all.
type unitLookup struct {
	// match is the single live match, nil when there is none or when ambiguous.
	match *models.Unit
	// ambiguous is the live match count when more than one unit answers the label.
	ambiguous int
	// deletedID is a soft-deleted match that would block creating the label, or 0.
	deletedID uint
}

// resolveUnits maps each sheet label to its lookup outcome.
//
// A label can match on name or on symbol, and the schema uniquely constrains only
// (unit_type, name) — symbols are unconstrained — so two different live units can
// legitimately answer to one label (one named BOX, another with symbol BOX). Keeping
// whichever row the query returned first would bind a product and its inventory item
// to the wrong unit, so every match is counted.
//
// Matching trims and upper-cases but never folds diacritics, so CUỐN, CUỘN and CUÔN
// remain three distinct units.
func (s *initialStockImportService) resolveUnits(
	ctx context.Context,
	rows []sheetRow,
	rej *rowRejections,
) (map[string]unitLookup, error) {
	labels := distinct(rows, rej, func(r sheetRow) string { return upperTrim(r.Unit) })
	found, err := s.unitRepo.GetByTypeAndNamesUnscoped(ctx, initialStockUnitType, labels)
	if err != nil {
		return nil, err
	}

	live := make(map[string][]*models.Unit)
	deleted := make(map[string]*models.Unit)
	for i := range found {
		u := &found[i]
		// A unit whose name and symbol normalize alike answers once, not twice.
		seen := make(map[string]bool, 2)
		for _, key := range []string{upperTrim(u.Name), upperTrim(u.Symbol)} {
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			if u.DeletedAt.Valid {
				if _, exists := deleted[key]; !exists {
					deleted[key] = u
				}
				continue
			}
			live[key] = append(live[key], u)
		}
	}

	out := make(map[string]unitLookup, len(labels))
	for _, label := range labels {
		l := unitLookup{}
		switch matches := live[label]; {
		case len(matches) > 1:
			l.ambiguous = len(matches)
		case len(matches) == 1:
			// A live match wins over a soft-deleted one: the live row is what the stock
			// would be denominated in.
			l.match = matches[0]
		default:
			if blocker, ok := deleted[label]; ok {
				l.deletedID = blocker.ID
			}
		}
		out[label] = l
	}
	return out, nil
}

// resolveProducts maps each trimmed name to the single live product it matches.
// More than one live match, or only a soft-deleted match, is a row error: neither
// is safe to guess at.
func (s *initialStockImportService) resolveProducts(
	ctx context.Context,
	rows []sheetRow,
	rej *rowRejections,
) (map[string]*models.Product, error) {
	keys := distinct(rows, rej, func(r sheetRow) string { return upperTrim(r.Name) })
	found, err := s.productRepo.GetByUpperNamesUnscoped(ctx, keys)
	if err != nil {
		return nil, err
	}

	live := make(map[string][]*models.Product)
	deleted := make(map[string][]*models.Product)
	for i := range found {
		p := &found[i]
		key := upperTrim(p.Name)
		if p.DeletedAt.Valid {
			deleted[key] = append(deleted[key], p)
			continue
		}
		live[key] = append(live[key], p)
	}

	matched := make(map[string]*models.Product)
	for _, row := range rows {
		if rej.failed[row.SheetRow] {
			continue
		}
		key := upperTrim(row.Name)
		switch {
		case len(live[key]) > 1:
			rej.add(row.SheetRow, pkg.ErrKeyInitialStockRowProductAmbiguous, row.Name, len(live[key]))
		case len(live[key]) == 1:
			matched[key] = live[key][0]
		case len(deleted[key]) > 0:
			rej.add(row.SheetRow, pkg.ErrKeyInitialStockRowProductDeleted, row.Name, deleted[key][0].ID)
		}
	}
	return matched, nil
}

// resolveItems maps each matched product's name key to its active inventory item.
// A soft-deleted or inactive row still occupies idx_inventory_items_unique, so it
// is reported rather than skipped or resurrected.
func (s *initialStockImportService) resolveItems(
	ctx context.Context,
	inventoryID uint,
	rows []sheetRow,
	productsByKey map[string]*models.Product,
	rej *rowRejections,
) (map[string]*models.InventoryItem, error) {
	productIDs := make([]uint, 0, len(productsByKey))
	keyByProductID := make(map[uint]string, len(productsByKey))
	for key, product := range productsByKey {
		productIDs = append(productIDs, product.ID)
		keyByProductID[product.ID] = key
	}
	sort.Slice(productIDs, func(i, j int) bool { return productIDs[i] < productIDs[j] })

	items, err := s.inventoryItemRepo.GetByInventoryAndProductIDsUnscoped(ctx, inventoryID, productIDs)
	if err != nil {
		return nil, err
	}

	active := make(map[string]*models.InventoryItem)
	blocked := make(map[string]*models.InventoryItem)
	blockedKey := make(map[string]string)
	for _, item := range items {
		key, ok := keyByProductID[item.ProductID]
		if !ok {
			continue
		}
		switch {
		case item.DeletedAt.Valid:
			blocked[key] = item
			blockedKey[key] = pkg.ErrKeyInitialStockRowItemSoftDeleted
		case item.Status != models.InventoryItemStatusActive:
			blocked[key] = item
			blockedKey[key] = pkg.ErrKeyInitialStockRowItemInactive
		default:
			active[key] = item
		}
	}

	for _, row := range rows {
		if rej.failed[row.SheetRow] {
			continue
		}
		key := upperTrim(row.Name)
		if _, ok := active[key]; ok {
			continue
		}
		if blocker, ok := blocked[key]; ok {
			rej.add(row.SheetRow, blockedKey[key], blocker.ID)
		}
	}
	return active, nil
}

// response renders the plan into the wire shape shared by both modes.
func (p *importPlan) response(req dto.InitialStockImportRequest, blocking []dto.InitialStockBlocking) *dto.InitialStockImportResponse {
	resp := &dto.InitialStockImportResponse{
		DryRun:      req.DryRun,
		InventoryID: req.InventoryID,
		SheetName:   req.SheetName,
		Blocking:    blocking,
		Rows:        make([]dto.InitialStockImportRow, 0, len(p.rows)),
		Errors:      []pkg.BatchErrorLocation{},
	}

	total := decimal.Zero
	resp.RowsProcessed = p.rowsRead
	resp.ProductsCreated = len(p.newProducts)
	resp.UnitsCreated = len(p.newUnits)

	for _, row := range p.rows {
		resulting := row.currentQty.Add(row.quantity)
		resp.Rows = append(resp.Rows, dto.InitialStockImportRow{
			Row:               row.src.SheetRow,
			Name:              row.src.Name,
			Unit:              row.src.Unit,
			Quantity:          row.quantity.String(),
			ProductType:       row.src.ProductType,
			ProductID:         row.product.ID,
			Actions:           row.actions,
			CurrentQuantity:   row.currentQty.String(),
			ResultingQuantity: resulting.String(),
			UnitDecimalPlaces: row.unitDP,
		})

		total = total.Add(row.quantity)
		if !row.productNew {
			resp.ProductsMatched++
		}
		if row.itemNew {
			resp.ItemsCreated++
		}
		if row.quantity.IsPositive() {
			resp.TransactionsCreated++
		} else {
			resp.RowsSkipped++
		}
		if !row.currentQty.IsZero() {
			resp.RowsOnItemsWithExistingStock++
		}
	}

	resp.TotalQuantity = total.String()
	return resp
}

// persist writes the whole load. Must run inside the caller's transaction, after
// the advisory lock and the guards.
func (s *initialStockImportService) persist(ctx context.Context, plan *importPlan) error {
	if err := s.importRepo.CreateUnits(ctx, plan.newUnits); err != nil {
		return err
	}
	for _, product := range plan.newProducts {
		for _, row := range plan.rows {
			if row.product == product {
				product.UnitID = row.unit.ID
				break
			}
		}
	}
	if err := s.importRepo.CreateProducts(ctx, plan.newProducts); err != nil {
		return err
	}

	changes := make([]*models.InventoryItemChange, 0, len(plan.rows))
	txns := make([]*models.InventoryTransaction, 0, len(plan.rows))

	for _, row := range plan.rows {
		item := row.item
		if row.itemNew {
			item = &models.InventoryItem{
				InventoryID: plan.inventoryID,
				ProductID:   row.product.ID,
				UnitID:      row.unit.ID,
				Quantity:    row.quantity,
				Status:      models.InventoryItemStatusActive,
			}
			changes = append(changes, &models.InventoryItemChange{InventoryItem: item, OriginalQuantity: decimal.Zero})
		} else {
			// Hydrated inside the transaction, so only Quantity moves: the batch
			// upsert behind SaveInventoryItemChanges writes every column.
			original := item.Quantity
			item.Quantity = item.Quantity.Add(row.quantity)
			changes = append(changes, &models.InventoryItemChange{InventoryItem: item, OriginalQuantity: original})
		}

		if !row.quantity.IsPositive() {
			continue
		}
		txn := &models.InventoryTransaction{
			TransactionType: models.InventoryTransactionTypeInitial,
			Quantity:        row.quantity,
			// Keys its own row family in the in/out export, the same way a
			// reconcile stock-up does, so the reports foot on real movement.
			IsAdjustment: true,
		}
		if row.itemNew {
			txn.InventoryItem = item
		} else {
			txn.InventoryItemID = item.ID
		}
		txns = append(txns, txn)
	}

	return s.inventoryItemRepo.SaveInventoryItemChanges(ctx, changes, txns)
}

func upperTrim(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

// unitLabelMatches reports whether a sheet label denotes the given unit, comparing
// trimmed and upper-cased but never diacritic-folded.
func unitLabelMatches(label string, unit *models.Unit) bool {
	return label == upperTrim(unit.Name) || label == upperTrim(unit.Symbol)
}

// distinct collects the distinct non-empty keys of rows not already rejected,
// preserving sheet order.
func distinct(rows []sheetRow, rej *rowRejections, key func(sheetRow) string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		if rej.failed[row.SheetRow] {
			continue
		}
		k := key(row)
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	return out
}

func orderedUnits(rows []sheetRow, pending map[string]*models.Unit) []*models.Unit {
	out := make([]*models.Unit, 0, len(pending))
	emitted := make(map[string]bool)
	for _, row := range rows {
		key := upperTrim(row.Unit)
		if unit, ok := pending[key]; ok && !emitted[key] {
			emitted[key] = true
			out = append(out, unit)
		}
	}
	return out
}

func orderedProducts(rows []sheetRow, pending map[string]*models.Product) []*models.Product {
	out := make([]*models.Product, 0, len(pending))
	emitted := make(map[string]bool)
	for _, row := range rows {
		key := upperTrim(row.Name)
		if product, ok := pending[key]; ok && !emitted[key] {
			emitted[key] = true
			out = append(out, product)
		}
	}
	return out
}
