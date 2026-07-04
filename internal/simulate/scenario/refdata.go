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
// Seeding is idempotent (lookup-or-create by name).
type RefData struct {
	SupplierIDs []uint
	UnitID      uint
	ProductIDs  []uint
	InventoryID uint
}

// Local request structs re-expressing the minimal JSON shape for endpoints whose
// bodies are not importable from internal/services/dto.

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

// entity is the minimal shape of a created/listed entity (id + name).
type entity struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

// listResponse is the paginated list/search envelope ({"data": [...]}).
type listResponse struct {
	Data []entity `json:"data"`
}

// Reference-data names. Stable so repeat runs reuse the same rows.
const (
	simUnitName      = "SIM Unit (each)"
	simInventoryName = "SIM Warehouse"
	supplierCount    = 2
	productCount     = 100

	// lookupPageSize is the page size used when paging list/search endpoints.
	lookupPageSize = 100
	// lookupMaxPages bounds paging; the 409-already-exists fallback covers rows
	// beyond it.
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

	// Seed starting stock for all products in one PO+receive.
	if err := seedInitialStock(ctx, env, ref, invID); err != nil {
		return nil, fmt.Errorf("seed initial stock: %w", err)
	}

	return ref, nil
}

// finder looks up an entity by name, returning (id, found, err).
type finder func(ctx context.Context, env *Env, name string) (uint, bool, error)

// createOrReuse looks the entity up and reuses it if found, else creates it. A
// 409 on create (a lost race or existing row) falls back to a second lookup, so
// re-running the seed is safe.
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

	// Already exists but the first lookup missed it: re-resolve and reuse.
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

// --- per-entity lookups ---

// findUnit searches /units case-insensitively, since the create endpoint
// uppercases names.
func findUnit(ctx context.Context, env *Env, name string) (uint, bool, error) {
	return pagedEnvelopeLookup(ctx, env, "/units", "GET /units", name, true)
}

// findSupplier searches /suppliers/search by exact name.
func findSupplier(ctx context.Context, env *Env, name string) (uint, bool, error) {
	return pagedEnvelopeLookup(ctx, env, "/suppliers/search", "GET /suppliers/search", name, false)
}

// findProduct searches /products/search by exact name.
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
