// Package scenario holds the business-lifecycle drivers the simulation runs
// against the API: idempotent reference-data setup plus the purchase-order,
// reconciliation, and payment lifecycles.
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
