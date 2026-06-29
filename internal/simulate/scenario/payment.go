package scenario

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
)

// Payment-receipt-form + revenue-expense lifecycle driver.
//
// A payment receipt form is filed against a purchase order, approved, and the
// day is then finalized. Approving a form is also what unlocks PO `completed`
// (UpdatePurchaseOrderStatus rejects completed unless an APPROVED form exists),
// so this scenario also drives a PO to its completed terminal state — the state
// PR-1 deliberately deferred.
//
//	POST /purchase-orders + receive                 (a fresh PO to pay for)
//	POST /payment-receipt-forms                      -> pending
//	PUT  /payment-receipt-forms/:id/approve          -> approved
//	PUT  /purchase-orders/:id/status (completed)     -> PO completed
//	POST /revenue-expenses/finalize                  (finalize the day)
//
// Each iteration owns a dedicated PO so completing it never interferes with the
// PO-scenario orders. IDs are read back from responses; none are assumed.
type PaymentScenario struct {
	iteration int
}

// Name implements Scenario.
func (s *PaymentScenario) Name() string { return "payment" }

// paymentFormResponse is the subset of a created/approved form we read back.
type paymentFormResponse struct {
	ID     uint   `json:"id"`
	Status string `json:"status"`
}

// Run drives one payment lifecycle and completes the PO it pays for.
func (s *PaymentScenario) Run(ctx context.Context, env *Env) error {
	s.iteration++
	ref := env.RefIDs
	if ref == nil || ref.InventoryID == 0 || len(ref.ProductIDs) == 0 {
		return fmt.Errorf("payment: reference data not initialized")
	}

	// 1. A dedicated PO to pay for (fully received so it is fully_delivered).
	po, err := createPO(ctx, env, s.iteration, ref.InventoryID)
	if err != nil {
		return err
	}
	if err := receivePO(ctx, env, po, true); err != nil {
		return err
	}

	// 2. File the payment receipt form (defaults to pending). full_name, details
	// and total_amount are required; date is set server-side.
	inv := ref.InventoryID
	payload := dto.PaymentReceiptFormPayload{
		InventoryID:     &inv,
		PurchaseOrderID: po.ID,
		FullName:        "SIM Automation",
		Department:      "SIM",
		Details:         fmt.Sprintf("SIM payment for PO %d", po.ID),
		TotalAmount:     125000,
	}
	var form paymentFormResponse
	if err := env.Client.Do(ctx, "POST", "/payment-receipt-forms", payload, &form, "POST /payment-receipt-forms"); err != nil {
		return fmt.Errorf("create payment form: %w", err)
	}
	if form.ID == 0 {
		return fmt.Errorf("payment: create form returned no id")
	}
	env.Report.Created("payment_receipt_form")

	// 3. Approve the form (pending -> approved).
	approvePath := fmt.Sprintf("/payment-receipt-forms/%d/approve", form.ID)
	if err := env.Client.Do(ctx, "PUT", approvePath, nil, nil, "PUT /payment-receipt-forms/:id/approve"); err != nil {
		return fmt.Errorf("approve payment form %d: %w", form.ID, err)
	}
	env.Report.Created("payment_receipt_form_approved")

	// 4. Now that an approved form exists, the PO can be completed.
	statusPath := fmt.Sprintf("/purchase-orders/%d/status", po.ID)
	if err := env.Client.Do(ctx, "PUT", statusPath, map[string]string{"status": "completed"}, nil, "PUT /purchase-orders/:id/status"); err != nil {
		return fmt.Errorf("complete PO %d: %w", po.ID, err)
	}
	env.Report.Created("purchase_order_completed")

	// 5. Finalize the day's revenue/expense (empty body finalizes today).
	// Finalize needs the `revenue_expense_excel` setting configured; on a fresh
	// local env it is not, and the server returns 500 with the
	// "settings not configured" domain error. That is an environment-config gap,
	// not a lifecycle failure: the payment lifecycle (form pending -> approved ->
	// PO completed) already completed. Classify ONLY that specific case as
	// expected via DoClassified so it is recorded as a NON-failure (it does not
	// inflate total_failures) and we skip it; every other finalize error is
	// recorded as a failure and fails the iteration.
	status, err := env.Client.DoClassified(ctx, "POST", "/revenue-expenses/finalize", struct{}{}, nil,
		"POST /revenue-expenses/finalize", isRevenueExpenseNotConfigured)
	if err != nil {
		return fmt.Errorf("finalize revenue expense: %w", err)
	}
	if status == http.StatusInternalServerError {
		// The only non-2xx DoClassified tolerates here: not-configured -> skipped.
		env.Report.Created("revenue_expense_skipped")
		return nil
	}
	env.Report.Created("revenue_expense_finalized")
	return nil
}

// isRevenueExpenseNotConfigured is the DoClassified predicate that recognises the
// "revenue expense settings not configured" domain error
// (pkg.ErrRevenueExpenseSettingsNotConfigured) from a finalize response. That
// error is an ErrorCodeInternal with NO `key` field, so it serializes as a
// generic HTTP 500 whose only distinguishing signal is its message. It therefore
// requires a 500 AND an exact match of the localized catalog message (EN or VI),
// so a 500 from any OTHER cause is NOT tolerated.
func isRevenueExpenseNotConfigured(status int, respBody []byte) bool {
	if status != http.StatusInternalServerError || len(respBody) == 0 {
		return false
	}
	body := string(respBody)
	msg := pkg.ErrorMessages[pkg.ErrKeyRevenueExpenseSettingsNotConfigured]
	for _, want := range []string{msg.EN, msg.VI} {
		if want != "" && strings.Contains(body, want) {
			return true
		}
	}
	return false
}
