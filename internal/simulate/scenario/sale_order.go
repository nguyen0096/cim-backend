package scenario

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"cim-backend/internal/services/dto"
	"cim-backend/internal/simulate/client"

	"github.com/shopspring/decimal"
)

// Sale-order + selling-price lifecycle driver.
//
// Selling prices are global (inventory_id NULL) and form a ledger keyed by
// effective_from. The scenario ensures one global price per product idempotently
// (list-by-product, create only if none exists) up front, then drives sale
// orders through their states:
//
//	POST /selling-prices                         (idempotent, per product)
//	POST /sale-orders                            -> ordered
//	PUT  /sale-orders/:id/status                 -> served
//	PUT  /sale-orders/:id   (status=completed)   -> new version (versioned)
//	PUT  /sale-orders/:id   (status=cancelled)   -> in-place
//
// Update semantics (from sale_order_service): a `completed` update creates a NEW
// version (previous_order_id chain, is_latest flips); `cancelled`/`served` update
// in place. Read the created sale-order ID back from the response; never assume.
//
// Variant cycling spreads orders across states for broad mock coverage:
//
//	variant 0: create -> status served                         (served)
//	variant 1: create -> update completed (new version)        (completed)
//	variant 2: create -> update cancelled (in place)           (cancelled)
//	variant 3: create only                                     (ordered)
type SaleOrderScenario struct {
	iteration    int
	pricesSeeded bool
}

// Name implements Scenario.
func (s *SaleOrderScenario) Name() string { return "sale_order" }

// createSaleOrderRequest is the JSON the CreateSaleOrder handler binds to
// (models.SaleOrder). customer_id and order_number auto-generate when empty.
// Items associate menu items by id; the server upserts the join rows.
type createSaleOrderRequest struct {
	InventoryID uint                 `json:"inventory_id"`
	Tag         int                  `json:"tag"`
	Notes       string               `json:"notes"`
	Items       []saleOrderItemInput `json:"items"`
}

type saleOrderItemInput struct {
	MenuItems []idRef `json:"menu_items"`
}

type idRef struct {
	ID uint `json:"id"`
}

// updateSaleOrderRequest is the body for PUT /sale-orders/:id. Status drives the
// versioned-vs-in-place behaviour server-side.
type updateSaleOrderRequest struct {
	Status string               `json:"status"`
	Notes  string               `json:"notes"`
	Items  []saleOrderItemInput `json:"items"`
}

type saleOrderResponse struct {
	ID          uint   `json:"id"`
	OrderNumber string `json:"order_number"`
	Status      string `json:"status"`
}

// Run drives one sale-order lifecycle. The first call also seeds the per-product
// global selling prices (idempotent).
func (s *SaleOrderScenario) Run(ctx context.Context, env *Env) error {
	ref := env.RefIDs
	if ref == nil || ref.InventoryID == 0 || ref.MenuItemID == 0 || len(ref.ProductIDs) == 0 {
		return fmt.Errorf("sale_order: reference data not initialized (need inventory, menu item, products)")
	}

	if !s.pricesSeeded {
		if err := s.ensureSellingPrices(ctx, env); err != nil {
			return fmt.Errorf("ensure selling prices: %w", err)
		}
		s.pricesSeeded = true
	}

	variant := s.iteration % 4
	s.iteration++

	create := createSaleOrderRequest{
		InventoryID: ref.InventoryID,
		Tag:         (variant % 3) + 1,
		Notes:       fmt.Sprintf("SIM sale order iteration %d", s.iteration),
		Items:       []saleOrderItemInput{{MenuItems: []idRef{{ID: ref.MenuItemID}}}},
	}
	var so saleOrderResponse
	if err := env.Client.Do(ctx, "POST", "/sale-orders", create, &so, "POST /sale-orders"); err != nil {
		return fmt.Errorf("create sale order: %w", err)
	}
	if so.ID == 0 {
		return fmt.Errorf("sale_order: create returned no id")
	}
	env.Report.Created("sale_order")

	switch variant {
	case 0: // served (status-only transition)
		path := fmt.Sprintf("/sale-orders/%d/status", so.ID)
		if err := env.Client.Do(ctx, "PUT", path, map[string]string{"status": "served"}, nil, "PUT /sale-orders/:id/status"); err != nil {
			return fmt.Errorf("serve sale order %d: %w", so.ID, err)
		}
		env.Report.Created("sale_order_served")
	case 1: // completed -> new version
		upd := updateSaleOrderRequest{
			Status: "completed",
			Notes:  "SIM completed",
			Items:  []saleOrderItemInput{{MenuItems: []idRef{{ID: ref.MenuItemID}}}},
		}
		path := fmt.Sprintf("/sale-orders/%d", so.ID)
		if err := env.Client.Do(ctx, "PUT", path, upd, nil, "PUT /sale-orders/:id"); err != nil {
			return fmt.Errorf("complete sale order %d: %w", so.ID, err)
		}
		env.Report.Created("sale_order_completed")
	case 2: // cancelled -> in place
		upd := updateSaleOrderRequest{
			Status: "cancelled",
			Notes:  "SIM cancelled",
			Items:  []saleOrderItemInput{{MenuItems: []idRef{{ID: ref.MenuItemID}}}},
		}
		path := fmt.Sprintf("/sale-orders/%d", so.ID)
		if err := env.Client.Do(ctx, "PUT", path, upd, nil, "PUT /sale-orders/:id"); err != nil {
			return fmt.Errorf("cancel sale order %d: %w", so.ID, err)
		}
		env.Report.Created("sale_order_cancelled")
	default: // variant 3: leave as ordered
	}
	return nil
}

// ensureSellingPrices creates one global selling price per ref product if the
// product has none yet (idempotent: ListByProductID returns the ledger). Global
// prices require inventory_id to be null/omitted.
func (s *SaleOrderScenario) ensureSellingPrices(ctx context.Context, env *Env) error {
	effectiveFrom := time.Now().Format("2006-01-02")
	for i, productID := range env.RefIDs.ProductIDs {
		has, err := productHasSellingPrice(ctx, env, productID)
		if err != nil {
			return err
		}
		if has {
			continue
		}
		req := dto.CreateSellingPriceRequest{
			ProductID:     productID,
			Price:         decimal.NewFromInt(int64(150 + i*50)),
			EffectiveFrom: effectiveFrom,
			Notes:         "SIM global selling price",
		}
		if err := env.Client.Do(ctx, "POST", "/selling-prices", req, nil, "POST /selling-prices"); err != nil {
			// A duplicate effective_from across re-runs is harmless (ledger allows
			// it); only abort on a non-conflict error.
			if client.StatusOf(err) == http.StatusConflict {
				continue
			}
			return err
		}
		env.Report.Created("selling_price")
	}
	return nil
}

// productHasSellingPrice reports whether the product already has any selling
// price in the ledger (GET /selling-prices?product_id= returns a raw array).
func productHasSellingPrice(ctx context.Context, env *Env, productID uint) (bool, error) {
	path := fmt.Sprintf("/selling-prices?product_id=%d", productID)
	var prices []struct {
		ID uint `json:"id"`
	}
	if err := env.Client.Do(ctx, "GET", path, nil, &prices, "GET /selling-prices"); err != nil {
		return false, err
	}
	return len(prices) > 0, nil
}
