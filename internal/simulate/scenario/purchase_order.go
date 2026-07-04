package scenario

import (
	"context"
	"fmt"

	"cim-backend/internal/services/dto"

	"github.com/shopspring/decimal"
)

// createPORequest is the JSON subset the create-PO handler reads.
type createPORequest struct {
	InventoryID uint           `json:"inventory_id"`
	Notes       string         `json:"notes"`
	Items       []createPOItem `json:"items"`
}

type createPOItem struct {
	ProductID  uint    `json:"product_id"`
	SupplierID uint    `json:"supplier_id"`
	UnitID     uint    `json:"unit_id"`
	Quantity   int     `json:"quantity"`
	UnitPrice  float64 `json:"unit_price"`
}

// poResponse is a created/updated PO's ID, status, and item IDs.
type poResponse struct {
	ID     uint   `json:"id"`
	Status string `json:"status"`
	Items  []struct {
		ID         uint `json:"id"`
		ProductID  uint `json:"product_id"`
		SupplierID uint `json:"supplier_id"`
	} `json:"items"`
}

// PurchaseOrderScenario drives the PO lifecycle (create -> receive -> optional
// status change), varying the variant across iterations to reach fully_delivered,
// partially_delivered, and cancelled.
type PurchaseOrderScenario struct {
	iteration int
}

// Name implements Scenario.
func (s *PurchaseOrderScenario) Name() string { return "purchase_order" }

// Run drives one PO lifecycle.
func (s *PurchaseOrderScenario) Run(ctx context.Context, env *Env) error {
	variant := s.iteration % 3
	s.iteration++

	ref := env.RefIDs
	if len(ref.ProductIDs) == 0 || len(ref.SupplierIDs) == 0 || ref.InventoryID == 0 {
		return fmt.Errorf("purchase_order: reference data not initialized")
	}

	// 1. Create the purchase order.
	po, err := createPO(ctx, env, s.iteration, ref.InventoryID)
	if err != nil {
		return err
	}

	// Variant 2: cancel without receiving.
	if variant == 2 {
		path := fmt.Sprintf("/purchase-orders/%d/status", po.ID)
		if err := env.Client.Do(ctx, "PUT", path, map[string]string{"status": "cancelled"}, nil, "PUT /purchase-orders/:id/status"); err != nil {
			return fmt.Errorf("cancel PO %d: %w", po.ID, err)
		}
		env.Report.Created("purchase_order_cancelled")
		return nil
	}

	// 2. Receive inventory: variant 0 full, variant 1 partial.
	full := variant != 1
	if err := receivePO(ctx, env, po, full); err != nil {
		return err
	}

	// 3. Read back the inventory items produced by receiving.
	if _, err := listInventoryItems(ctx, env, ref.InventoryID); err != nil {
		return fmt.Errorf("list inventory items: %w", err)
	}

	return nil
}

// poQty is the base per-item quantity a lifecycle PO orders (randomized up per
// line).
const poQty = 1000

// poMaxItems caps how many distinct products a single lifecycle PO buys.
const poMaxItems = 5

// seedQty is the per-product quantity stocked once at setup (seedInitialStock).
const seedQty = 100000

// createPO creates a purchase order against inventoryID from the reference
// products/suppliers/unit. Shared by the PO, payment, and reconciliation
// scenarios.
func createPO(ctx context.Context, env *Env, iteration int, inventoryID uint) (*poResponse, error) {
	ref := env.RefIDs
	n := len(ref.ProductIDs)
	// Buy a random distinct subset of products at randomized quantity/price.
	k := 1 + env.Rand.Intn(min(poMaxItems, n))
	idxs := env.Rand.Perm(n)[:k]

	create := createPORequest{
		InventoryID: inventoryID,
		Notes:       fmt.Sprintf("SIM PO iteration %d", iteration),
	}
	for _, idx := range idxs {
		create.Items = append(create.Items, createPOItem{
			ProductID:  ref.ProductIDs[idx],
			SupplierID: ref.SupplierIDs[env.Rand.Intn(len(ref.SupplierIDs))],
			UnitID:     ref.UnitID,
			Quantity:   poQty + env.Rand.Intn(poQty),
			UnitPrice:  float64(50 + env.Rand.Intn(450)),
		})
	}

	var po poResponse
	if err := env.Client.Do(ctx, "POST", "/purchase-orders", create, &po, "POST /purchase-orders"); err != nil {
		return nil, fmt.Errorf("create PO: %w", err)
	}
	env.Report.Created("purchase_order")
	return &po, nil
}

// seedInitialStock places one PO covering every reference product and fully
// receives it, so the inventory starts with stock across all products. Called
// once during ref-data setup.
func seedInitialStock(ctx context.Context, env *Env, ref *RefData, inventoryID uint) error {
	create := createPORequest{InventoryID: inventoryID, Notes: "SIM initial stock (all products)"}
	for i, pid := range ref.ProductIDs {
		create.Items = append(create.Items, createPOItem{
			ProductID:  pid,
			SupplierID: ref.SupplierIDs[i%len(ref.SupplierIDs)],
			UnitID:     ref.UnitID,
			Quantity:   seedQty,
			UnitPrice:  100,
		})
	}
	var po poResponse
	if err := env.Client.Do(ctx, "POST", "/purchase-orders", create, &po, "POST /purchase-orders"); err != nil {
		return fmt.Errorf("seed stock PO: %w", err)
	}
	env.Report.Created("purchase_order")
	return receivePO(ctx, env, &po, true)
}

// receivePO receives a PO's items. full receives the whole ordered quantity,
// otherwise half.
func receivePO(ctx context.Context, env *Env, po *poResponse, full bool) error {
	recv := dto.UpdatePurchaseOrderDeliveryStatusRequest{
		PurchaseOrderID:   po.ID,
		ConfirmationNotes: "SIM receive",
	}
	received := poQty
	if !full {
		received = poQty / 2
	}
	for _, item := range po.Items {
		recv.Items = append(recv.Items, struct {
			ID               uint            `json:"id" validate:"required"`
			ReceivedQuantity decimal.Decimal `json:"received_quantity" validate:"required"`
		}{ID: item.ID, ReceivedQuantity: decimal.NewFromInt(int64(received))})
	}
	// Read lock: receiving mutates item quantities, so it must not run during a
	// reconcile's write-locked apply window.
	inventoryMu.RLock()
	recvPath := fmt.Sprintf("/purchase-orders/%d/receive", po.ID)
	err := env.Client.Do(ctx, "PUT", recvPath, recv, nil, "PUT /purchase-orders/:id/receive")
	inventoryMu.RUnlock()
	if err != nil {
		return fmt.Errorf("receive PO %d: %w", po.ID, err)
	}
	env.Report.Created("purchase_order_received")
	return nil
}
