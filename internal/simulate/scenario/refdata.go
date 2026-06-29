package scenario

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"cim-backend/internal/simulate/client"
)

// RefData holds the IDs of the shared reference entities every scenario needs.
// Seeding is idempotent (lookup-or-create by name) so re-runs do not error on
// duplicates; it is additive by design — there is no cleanup.
type RefData struct {
	SupplierIDs []uint
	UnitID      uint
	ProductIDs  []uint
	InventoryID uint
	// MenuItemID is a single menu item the sale-order scenario references. Built
	// over the ref products so a sale order has something sellable.
	MenuItemID uint
}

// Local request structs. The supplier/inventory create endpoints bind directly
// to models, and product/unit bind to private handler structs, so none of these
// are importable from internal/services/dto — we re-express the minimal JSON
// shape (field values borrowed from the Ginkgo tests / seed data).

type createSupplierRequest struct {
	Name string `json:"name"`
}

type createUnitRequest struct {
	UnitType         string  `json:"unit_type"`
	Name             string  `json:"name"`
	Symbol           string  `json:"symbol"`
	ConversionFactor float64 `json:"conversion_factor"`
}

type createProductRequest struct {
	Name        string `json:"name"`
	ProductType string `json:"product_type"`
	UnitID      uint   `json:"unit_id"`
	Status      string `json:"status"`
}

type createInventoryRequest struct {
	Name     string `json:"name"`
	Location string `json:"location"`
}

type createMenuItemRequest struct {
	Name       string `json:"name"`
	ProductIDs []uint `json:"product_ids,omitempty"`
}

// entity is the minimal shape of a created/listed entity (id + name).
type entity struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

// listResponse is the paginated list/search envelope ({"data": [...]}). Used by
// units, suppliers/search and products/search.
type listResponse struct {
	Data []entity `json:"data"`
}

// Reference-data names. Stable so repeat runs reuse the same rows.
const (
	simUnitName      = "SIM Unit (each)"
	simInventoryName = "SIM Warehouse"
	simMenuItemName  = "SIM Menu Item"
	supplierCount    = 2
	productCount     = 2

	// lookupPageSize is the page size used when paging list/search endpoints.
	lookupPageSize = 100
	// lookupMaxPages bounds paging so a huge dev DB can't spin forever; the
	// 409-already-exists fallback covers any row beyond this.
	lookupMaxPages = 50
)

// EnsureRefData creates (or reuses) the suppliers, unit, products and inventory
// the lifecycle scenarios depend on. Safe to call repeatedly.
func EnsureRefData(ctx context.Context, env *Env) (*RefData, error) {
	ref := &RefData{}

	unitID, err := ensureUnit(ctx, env)
	if err != nil {
		return nil, fmt.Errorf("ensure unit: %w", err)
	}
	ref.UnitID = unitID

	for i := 0; i < supplierCount; i++ {
		name := fmt.Sprintf("SIM Supplier %d", i+1)
		id, err := ensureSupplier(ctx, env, name)
		if err != nil {
			return nil, fmt.Errorf("ensure supplier %q: %w", name, err)
		}
		ref.SupplierIDs = append(ref.SupplierIDs, id)
	}

	for i := 0; i < productCount; i++ {
		name := fmt.Sprintf("SIM Product %d", i+1)
		id, err := ensureProduct(ctx, env, name, unitID)
		if err != nil {
			return nil, fmt.Errorf("ensure product %q: %w", name, err)
		}
		ref.ProductIDs = append(ref.ProductIDs, id)
	}

	invID, err := ensureInventory(ctx, env, simInventoryName)
	if err != nil {
		return nil, fmt.Errorf("ensure inventory: %w", err)
	}
	ref.InventoryID = invID

	menuItemID, err := ensureMenuItem(ctx, env, simMenuItemName, ref.ProductIDs)
	if err != nil {
		return nil, fmt.Errorf("ensure menu item: %w", err)
	}
	ref.MenuItemID = menuItemID

	return ref, nil
}

// finder looks up an entity by name, returning (id, found, err).
type finder func(ctx context.Context, env *Env, name string) (uint, bool, error)

// createOrReuse is the shared idempotent pattern: look the entity up; if found,
// reuse it. Otherwise create it. If the create races/loses to an existing row
// (the server replies 409 Conflict, e.g. unit name standardization or a unique
// constraint), fall back to a second lookup and reuse instead of aborting —
// this is what makes re-running the seed safe.
func createOrReuse(ctx context.Context, env *Env, entityType, name, createPath, createLabel string, body any, find finder) (uint, error) {
	if id, ok, err := find(ctx, env, name); err != nil {
		return 0, err
	} else if ok {
		return id, nil
	}

	var created entity
	err := env.Client.Do(ctx, "POST", createPath, body, &created, createLabel)
	if err == nil {
		env.Report.Created(entityType)
		return created.ID, nil
	}
	if client.StatusOf(err) != http.StatusConflict {
		return 0, err
	}

	// Already exists (lookup missed it, e.g. server-side name normalization or
	// a row beyond our paging window): re-resolve and reuse.
	if id, ok, lookErr := find(ctx, env, name); lookErr != nil {
		return 0, lookErr
	} else if ok {
		return id, nil
	}
	return 0, fmt.Errorf("%s %q reported as existing (409) but lookup could not resolve it", entityType, name)
}

func ensureUnit(ctx context.Context, env *Env) (uint, error) {
	body := createUnitRequest{
		UnitType:         "general",
		Name:             simUnitName,
		Symbol:           "ea",
		ConversionFactor: 1,
	}
	return createOrReuse(ctx, env, "unit", simUnitName, "/units", "POST /units", body, findUnit)
}

func ensureSupplier(ctx context.Context, env *Env, name string) (uint, error) {
	return createOrReuse(ctx, env, "supplier", name, "/suppliers", "POST /suppliers", createSupplierRequest{Name: name}, findSupplier)
}

func ensureProduct(ctx context.Context, env *Env, name string, unitID uint) (uint, error) {
	body := createProductRequest{Name: name, ProductType: "general", UnitID: unitID, Status: "active"}
	return createOrReuse(ctx, env, "product", name, "/products", "POST /products", body, findProduct)
}

func ensureInventory(ctx context.Context, env *Env, name string) (uint, error) {
	body := createInventoryRequest{Name: name, Location: "SIM Location"}
	return createOrReuse(ctx, env, "inventory", name, "/inventories", "POST /inventories", body, findInventory)
}

// createDedicatedInventory creates a freshly-named inventory and returns its ID.
// Unlike ensureInventory it does NOT reuse an existing row: scenarios that need
// an isolated inventory (e.g. reconciliation, so parked active-pending reconciles
// never collide on the one-active-pending guard) call this per iteration. The
// caller supplies a unique name so re-runs stay additive.
func createDedicatedInventory(ctx context.Context, env *Env, name string) (uint, error) {
	body := createInventoryRequest{Name: name, Location: "SIM Location"}
	var created entity
	if err := env.Client.Do(ctx, "POST", "/inventories", body, &created, "POST /inventories"); err != nil {
		return 0, err
	}
	env.Report.Created("inventory")
	return created.ID, nil
}

func ensureMenuItem(ctx context.Context, env *Env, name string, productIDs []uint) (uint, error) {
	body := createMenuItemRequest{Name: name, ProductIDs: productIDs}
	return createOrReuse(ctx, env, "menu_item", name, "/menu-items", "POST /menu-items", body, findMenuItem)
}

// --- per-entity lookups (each tuned to its endpoint's response shape) ---

// findUnit searches /units (which honors q and returns {data:[...]}). The unit
// create endpoint uppercases names (Unit.StandardizeName), so the comparison is
// case-insensitive.
func findUnit(ctx context.Context, env *Env, name string) (uint, bool, error) {
	return pagedEnvelopeLookup(ctx, env, "/units", "GET /units", name, true)
}

// findSupplier uses /suppliers/search (the list route ignores q); envelope
// shape, exact-name match.
func findSupplier(ctx context.Context, env *Env, name string) (uint, bool, error) {
	return pagedEnvelopeLookup(ctx, env, "/suppliers/search", "GET /suppliers/search", name, false)
}

// findProduct uses /products/search (the list route ignores q); envelope shape.
func findProduct(ctx context.Context, env *Env, name string) (uint, bool, error) {
	return pagedEnvelopeLookup(ctx, env, "/products/search", "GET /products/search", name, false)
}

// findInventory pages GET /inventories, which returns a RAW JSON array (not a
// {data:[...]} envelope) and ignores q. Inventory names are unique, so paging
// to the match is sufficient.
func findInventory(ctx context.Context, env *Env, name string) (uint, bool, error) {
	for page := 1; page <= lookupMaxPages; page++ {
		path := fmt.Sprintf("/inventories?limit=%d&page=%d", lookupPageSize, page)
		var items []entity
		if err := env.Client.Do(ctx, "GET", path, nil, &items, "GET /inventories"); err != nil {
			return 0, false, err
		}
		for _, e := range items {
			if nameEqual(e.Name, name, false) {
				return e.ID, true, nil
			}
		}
		if len(items) < lookupPageSize {
			break // last page
		}
	}
	return 0, false, nil
}

// findMenuItem pages GET /menu-items, which returns a RAW JSON array (not a
// {data:[...]} envelope) and honors q. Menu-item names are not guaranteed
// unique, but the SIM name is stable so the first exact match is reused.
func findMenuItem(ctx context.Context, env *Env, name string) (uint, bool, error) {
	for page := 1; page <= lookupMaxPages; page++ {
		path := fmt.Sprintf("/menu-items?limit=%d&page=%d&q=%s", lookupPageSize, page, url.QueryEscape(name))
		var items []entity
		if err := env.Client.Do(ctx, "GET", path, nil, &items, "GET /menu-items"); err != nil {
			return 0, false, err
		}
		for _, e := range items {
			if nameEqual(e.Name, name, false) {
				return e.ID, true, nil
			}
		}
		if len(items) < lookupPageSize {
			break // last page
		}
	}
	return 0, false, nil
}

// pagedEnvelopeLookup pages a {data:[...]} endpoint that honors q, matching by
// exact (optionally case-insensitive) name.
func pagedEnvelopeLookup(ctx context.Context, env *Env, path, label, name string, caseInsensitive bool) (uint, bool, error) {
	for page := 1; page <= lookupMaxPages; page++ {
		q := fmt.Sprintf("%s?limit=%d&page=%d&q=%s", path, lookupPageSize, page, url.QueryEscape(name))
		var resp listResponse
		if err := env.Client.Do(ctx, "GET", q, nil, &resp, label); err != nil {
			return 0, false, err
		}
		for _, e := range resp.Data {
			if nameEqual(e.Name, name, caseInsensitive) {
				return e.ID, true, nil
			}
		}
		if len(resp.Data) < lookupPageSize {
			break // last page
		}
	}
	return 0, false, nil
}

func nameEqual(a, b string, caseInsensitive bool) bool {
	if caseInsensitive {
		return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
	}
	return a == b
}
