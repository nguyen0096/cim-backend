package scenario

import (
	"context"
	"fmt"

	"cim-backend/internal/services/dto"

	"github.com/shopspring/decimal"
)

// Purchase-order create payload. The handler binds to models.PurchaseOrder but
// only reads this JSON subset; field values mirror test/purchase_order_test.go.

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

// poResponse is the subset of the created/updated PO we read back. We never
// assume item IDs — receive uses the IDs the server returns here.
type poResponse struct {
	ID     uint   `json:"id"`
	Status string `json:"status"`
	Items  []struct {
		ID         uint `json:"id"`
		ProductID  uint `json:"product_id"`
		SupplierID uint `json:"supplier_id"`
	} `json:"items"`
}

// PurchaseOrderScenario drives the full PO lifecycle:
//
//	POST /purchase-orders  -> PUT /purchase-orders/:id/receive  [-> PUT /purchase-orders/:id/status]
//
// Receiving produces inventory items as a side effect; the scenario then reads
// them back via GET /inventories/:id/inventory-items.
//
// Across iterations it varies the receive amount to exercise the terminal/
// intermediate states: fully_delivered, partially_delivered, and cancelled.
type PurchaseOrderScenario struct {
	iteration int
}

// Name implements Scenario.
func (s *PurchaseOrderScenario) Name() string { return "purchase_order" }

// Run drives one PO lifecycle. The variant cycles each call so a multi-volume
// run covers a spread of states.
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

	// Variant 2: cancel without receiving (terminal: cancelled).
	if variant == 2 {
		path := fmt.Sprintf("/purchase-orders/%d/status", po.ID)
		if err := env.Client.Do(ctx, "PUT", path, map[string]string{"status": "cancelled"}, nil, "PUT /purchase-orders/:id/status"); err != nil {
			return fmt.Errorf("cancel PO %d: %w", po.ID, err)
		}
		env.Report.Created("purchase_order_cancelled")
		return nil
	}

	// 2. Receive inventory. variant 0 = full (fully_delivered),
	// variant 1 = partial (partially_delivered).
	full := variant != 1
	if err := receivePO(ctx, env, po, full); err != nil {
		return err
	}

	// 3. Read back the inventory items produced by receiving, by real ID.
	if _, err := listInventoryItems(ctx, env, ref.InventoryID); err != nil {
		return fmt.Errorf("list inventory items: %w", err)
	}

	return nil
}

// poQty is the per-item quantity each simulated PO orders. Receiving the full
// amount produces inventory items with this live quantity (the reconcile
// baseline); a half receive leaves the PO partially_delivered.
const poQty = 100

// createPO creates a purchase order against inventoryID from the reference
// products/suppliers/unit and returns the server's response (real PO id + item
// ids). Shared by the PO, payment, and reconciliation scenarios so all build a
// valid PO the same way; the target inventory is explicit so a scenario can
// seed a dedicated inventory.
func createPO(ctx context.Context, env *Env, iteration int, inventoryID uint) (*poResponse, error) {
	ref := env.RefIDs
	create := createPORequest{
		InventoryID: inventoryID,
		Notes:       fmt.Sprintf("SIM PO iteration %d", iteration),
		Items: []createPOItem{
			{ProductID: ref.ProductIDs[0], SupplierID: ref.SupplierIDs[0], UnitID: ref.UnitID, Quantity: poQty, UnitPrice: 100},
		},
	}
	if len(ref.ProductIDs) > 1 {
		create.Items = append(create.Items, createPOItem{
			ProductID: ref.ProductIDs[1], SupplierID: supplierAt(ref, 1), UnitID: ref.UnitID, Quantity: poQty, UnitPrice: 250,
		})
	}

	var po poResponse
	if err := env.Client.Do(ctx, "POST", "/purchase-orders", create, &po, "POST /purchase-orders"); err != nil {
		return nil, fmt.Errorf("create PO: %w", err)
	}
	env.Report.Created("purchase_order")
	return &po, nil
}

// receivePO receives a PO's items by their real (server-returned) ids. full
// receives the whole ordered quantity (fully_delivered); otherwise half
// (partially_delivered).
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
	recvPath := fmt.Sprintf("/purchase-orders/%d/receive", po.ID)
	if err := env.Client.Do(ctx, "PUT", recvPath, recv, nil, "PUT /purchase-orders/:id/receive"); err != nil {
		return fmt.Errorf("receive PO %d: %w", po.ID, err)
	}
	env.Report.Created("purchase_order_received")
	return nil
}

// supplierAt returns the supplier ID at index i, falling back to the first.
func supplierAt(ref *RefData, i int) uint {
	if i < len(ref.SupplierIDs) {
		return ref.SupplierIDs[i]
	}
	return ref.SupplierIDs[0]
}
