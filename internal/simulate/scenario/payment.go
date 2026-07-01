package scenario

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
)

// PaymentScenario files a payment receipt form against a fresh PO, approves it,
// completes the PO (which requires an approved form), and finalizes the day's
// revenue/expense. Each iteration owns a dedicated PO.
type PaymentScenario struct {
	iteration int
}

// Name implements Scenario.
func (s *PaymentScenario) Name() string { return "payment" }

// paymentFormResponse is a payment form's ID and status.
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

	// 1. A dedicated PO to pay for (fully received).
	po, err := createPO(ctx, env, s.iteration, ref.InventoryID)
	if err != nil {
		return err
	}
	if err := receivePO(ctx, env, po, true); err != nil {
		return err
	}

	// 2. File the payment receipt form (defaults to pending).
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

	// 4. Complete the PO (now that an approved form exists).
	statusPath := fmt.Sprintf("/purchase-orders/%d/status", po.ID)
	if err := env.Client.Do(ctx, "PUT", statusPath, map[string]string{"status": "completed"}, nil, "PUT /purchase-orders/:id/status"); err != nil {
		return fmt.Errorf("complete PO %d: %w", po.ID, err)
	}
	env.Report.Created("purchase_order_completed")

	// 5. Finalize the day's revenue/expense (empty body finalizes today). When the
	// revenue-expense Excel settings are unavailable (common in dev), finalize
	// returns 500; those specific errors are tolerated as skipped, not failures.
	status, err := env.Client.DoClassified(ctx, "POST", "/revenue-expenses/finalize", struct{}{}, nil,
		"POST /revenue-expenses/finalize", isRevenueExpenseSettingsUnavailable)
	if err != nil {
		return fmt.Errorf("finalize revenue expense: %w", err)
	}
	if status == http.StatusInternalServerError {
		env.Report.Created("revenue_expense_skipped")
		return nil
	}
	env.Report.Created("revenue_expense_finalized")
	return nil
}

// revenueExpenseSettingsErrorKeys are the revenue-expense settings errors that
// mean the Excel-settings integration is unusable here; finalize is skipped on
// any of them.
var revenueExpenseSettingsErrorKeys = []string{
	pkg.ErrKeyRevenueExpenseSettingsNotConfigured,
	pkg.ErrKeyFailedToGetRevenueExpenseSettings,
	pkg.ErrKeyFailedToParseRevenueExpenseSettings,
	pkg.ErrKeyFilePathNotFoundInSettings,
	pkg.ErrKeySheetNameNotFoundInSettings,
}

// isRevenueExpenseSettingsUnavailable recognises a revenue-expense settings error
// from a finalize response. Those errors carry no key and serialize as a generic
// 500, so it requires a 500 and an exact match of a localized (EN/VI) catalog
// message; a 500 from any other cause is not tolerated.
func isRevenueExpenseSettingsUnavailable(status int, respBody []byte) bool {
	if status != http.StatusInternalServerError || len(respBody) == 0 {
		return false
	}
	body := string(respBody)
	for _, key := range revenueExpenseSettingsErrorKeys {
		msg := pkg.ErrorMessages[key]
		for _, want := range []string{msg.EN, msg.VI} {
			if want != "" && strings.Contains(body, want) {
				return true
			}
		}
	}
	return false
}
