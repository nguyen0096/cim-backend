// Package runner orchestrates a simulation run: it sets up shared reference
// data once (serialized), then drives scenarios.
//
// PR-2 runs the full set of lifecycle scenarios serially in mock mode, weighted
// for broad state coverage. The worker pool, rate limiting, and duration knobs
// for load mode land in PR-3; the Run signature and scenario list are arranged
// so they slot in here.
package runner

import (
	"context"
	"fmt"
	"math/rand"

	"cim-backend/internal/simulate/client"
	"cim-backend/internal/simulate/config"
	"cim-backend/internal/simulate/report"
	"cim-backend/internal/simulate/scenario"
)

// Runner executes scenarios against the API.
type Runner struct {
	cfg    *config.Config
	client *client.Client
	report *report.Report
}

// New builds a Runner with its client and report wired to the config.
func New(cfg *config.Config) (*Runner, *report.Report) {
	rep := report.New()
	c := client.New(client.Options{
		BaseURL:        cfg.BaseURL,
		Timeout:        cfg.Timeout,
		Email:          cfg.Email,
		Password:       cfg.Password,
		FirebaseAPIKey: cfg.FirebaseAPIKey,
		Report:         rep,
	})
	return &Runner{cfg: cfg, client: c, report: rep}, rep
}

// Run verifies auth, seeds reference data, then drives the PR-1 scenarios.
func (r *Runner) Run(ctx context.Context) error {
	if err := r.client.VerifyAuth(ctx); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	env := &scenario.Env{
		Client: r.client,
		Report: r.report,
		Rand:   rand.New(rand.NewSource(r.cfg.Seed)),
	}

	// Reference data is created once, serialized, before any scenario runs so
	// scenarios only ever reference existing IDs.
	ref, err := scenario.EnsureRefData(ctx, env)
	if err != nil {
		return fmt.Errorf("reference data setup: %w", err)
	}
	env.RefIDs = ref

	return driveScenarios(ctx, env, mockSchedule(r.cfg.Volume()))
}

// scheduledScenario pairs a scenario with how many times to drive it.
type scheduledScenario struct {
	scenario scenario.Scenario
	runs     int
}

// mockSchedule builds the ordered scenario plan for a mock run, weighted so a
// single run leaves a BROAD spread of lifecycle states in the DB. The PO
// scenario runs first to seed the shared inventory; the payment and
// reconciliation scenarios each own their PO (and reconciliation its own
// inventory per iteration, so parked reconciles never collide on the
// one-active-pending guard) and drive PO `completed` / the reconcile lifecycle.
//
// Counts are derived from base (the configured volume) so scale still tunes the
// run, while each scenario's internal variant-cycling already fans every state
// out. The counts are >= each scenario's variant count at every scale so even
// the smallest run touches every reachable terminal/intermediate state.
func mockSchedule(base int) []scheduledScenario {
	if base < 1 {
		base = 1
	}
	// Reconciliation has 4 variants; ensure at least 4 runs so a small run still
	// produces open/closed/processed plus the reopen path.
	reconRuns := base + 3
	saleRuns := base + 3 // 4 variants
	payRuns := base      // payment is heavier (PO + receive + form + finalize)
	if payRuns < 1 {
		payRuns = 1
	}
	return []scheduledScenario{
		{&scenario.PurchaseOrderScenario{}, base},
		{&scenario.PaymentScenario{}, payRuns},
		{&scenario.ReconciliationScenario{}, reconRuns},
		{&scenario.SaleOrderScenario{}, saleRuns},
	}
}

// driveScenarios runs each scheduled scenario its configured number of times in
// order, accumulating failures. It returns an aggregate error if any iteration
// failed (so the caller exits non-zero) and aborts early on context cancel.
func driveScenarios(ctx context.Context, env *scenario.Env, plan []scheduledScenario) error {
	var firstErr error
	for _, sc := range plan {
		if err := driveScenario(ctx, env, sc.scenario, sc.runs); err != nil {
			if ctx.Err() != nil {
				return err
			}
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// driveScenario runs sc volume times. Per-iteration failures are recorded (in
// the report, via the client) and logged, but do NOT abort the run — the loop
// always completes so a partial seed is maximized. If any iteration failed it
// returns an aggregate error so the caller can exit non-zero; a fully
// successful run returns nil. A cancelled context aborts early and is returned.
func driveScenario(ctx context.Context, env *scenario.Env, sc scenario.Scenario, volume int) error {
	failed := 0
	for i := 0; i < volume; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := sc.Run(ctx, env); err != nil {
			failed++
			fmt.Printf("scenario %s iteration %d failed: %v\n", sc.Name(), i+1, err)
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d/%d %s iterations failed", failed, volume, sc.Name())
	}
	return nil
}
