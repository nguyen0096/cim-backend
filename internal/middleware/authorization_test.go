package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

// resolve runs extractResourceAndAction for a given method+path through a real
// echo context, exercising the production custom-route-mapping table.
func resolve(method, path string) (string, string) {
	e := echo.New()
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	return extractResourceAndAction(c)
}

// TestExtractResourceAndAction_ReconcileInitiate locks the RBAC gating for the
// epic #38 Part 2 endpoint: the initiate route must resolve to
// inventory-submissions:initiate_reconciliation, NOT the generic create action
// and NOT the inventory-items resource it would otherwise fall back to.
func TestExtractResourceAndAction_ReconcileInitiate(t *testing.T) {
	resource, action := resolve(http.MethodPost, "/api/v1/inventories/7/reconcile/initiate")
	assert.Equal(t, "inventory-submissions", resource)
	assert.Equal(t, "initiate_reconciliation", action)
}

// TestExtractResourceAndAction_ReconcileGeneric ensures the new, more specific
// initiate mapping did not disturb the existing reconcile mapping: a plain
// reconcile POST still maps to inventory-submissions with the create action.
func TestExtractResourceAndAction_ReconcileGeneric(t *testing.T) {
	resource, action := resolve(http.MethodPost, "/api/v1/inventories/7/reconcile")
	assert.Equal(t, "inventory-submissions", resource)
	assert.Equal(t, "create", action)
}

// TestExtractResourceAndAction_ReconciliationChildItems locks the RBAC gating for
// the epic #38 Part 4 staff child-item routes: each (method, path) must resolve to
// inventory-submissions with its own explicit recon_item_* action, distinct from
// the generic create/update/delete actions, so the routes are gated independently.
func TestExtractResourceAndAction_ReconciliationChildItems(t *testing.T) {
	cases := []struct {
		name, method, path, wantAction string
	}{
		{"create", http.MethodPost, "/api/v1/inventories/submissions/50/reconciliation-items", "recon_item_create"},
		{"update", http.MethodPut, "/api/v1/inventories/submissions/50/reconciliation-items/777", "recon_item_update"},
		{"delete", http.MethodDelete, "/api/v1/inventories/submissions/50/reconciliation-items/777", "recon_item_delete"},
		{"close", http.MethodPost, "/api/v1/inventories/submissions/50/close", "recon_manage"},
		{"reopen", http.MethodPost, "/api/v1/inventories/submissions/50/reopen", "recon_manage"},
		{"start-processing", http.MethodPost, "/api/v1/inventories/submissions/50/start-processing", "recon_manage"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resource, action := resolve(tc.method, tc.path)
			assert.Equal(t, "inventory-submissions", resource)
			assert.Equal(t, tc.wantAction, action)
		})
	}
}

// TestExtractResourceAndAction_SubmissionProcessUnaffected ensures the new
// child-item mappings (also 4 segments under /inventories/submissions/*/...) do
// not shadow the existing submission process route. The process route has no
// custom mapping (its permission is enforced in-service via the approve action),
// so it keeps falling through to the default resource resolution exactly as
// before the Part 4 mappings were added — the child-item patterns only match the
// distinct ".../reconciliation-items..." last segment.
func TestExtractResourceAndAction_SubmissionProcessUnaffected(t *testing.T) {
	resource, action := resolve(http.MethodPost, "/api/v1/inventories/submissions/50/process")
	// Unchanged pre-existing fallback behavior (documented, not asserted as ideal).
	assert.Equal(t, "inventory-items", resource)
	assert.Equal(t, "create", action)
}
