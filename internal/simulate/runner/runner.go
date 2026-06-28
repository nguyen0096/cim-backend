// Package runner orchestrates a simulation run: it sets up shared reference
// data once (serialized), then drives scenarios.
//
// PR-1 runs the purchase-order scenario serially Volume() times in mock mode.
// The worker pool, rate limiting, and duration knobs for load mode land in
// PR-3; the Run signature and scenario list are arranged so they slot in here.
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

	volume := r.cfg.Volume()
	po := &scenario.PurchaseOrderScenario{}
	return driveScenario(ctx, env, po, volume)
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
