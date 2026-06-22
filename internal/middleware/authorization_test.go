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
