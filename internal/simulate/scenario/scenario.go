// Package scenario holds the business-lifecycle drivers the simulation runs
// against the API. Each scenario reads real IDs back from responses and chains
// calls in the correct order; none assume fixed IDs.
//
// PR-1 ships idempotent reference-data setup plus the purchase-order lifecycle.
// Reconciliation, sale-order, and payment scenarios slot in here in PR-2.
package scenario

import (
	"context"
	"math/rand"

	"cim-backend/internal/simulate/client"
	"cim-backend/internal/simulate/report"
)

// Env is the shared dependency set handed to every scenario.
type Env struct {
	Client *client.Client
	Report *report.Report
	Rand   *rand.Rand
	RefIDs *RefData // populated by EnsureRefData before scenarios run
}

// Scenario is one business lifecycle that can be driven end-to-end.
type Scenario interface {
	// Name identifies the scenario in logs/reports.
	Name() string
	// Run drives the lifecycle once.
	Run(ctx context.Context, env *Env) error
}
