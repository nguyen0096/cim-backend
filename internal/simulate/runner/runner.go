// Package runner orchestrates a simulation run: it sets up shared reference data
// once, then drives scenarios in mock mode (serial) or load mode (concurrent
// worker pool). A cancelled context stops either cleanly.
package runner

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

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
	// newSet builds a fresh scenario set for one worker; a field so tests can
	// inject stubs.
	newSet func() []scenario.Scenario
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
	return &Runner{cfg: cfg, client: c, report: rep, newSet: newScenarioSet}, rep
}

// Run verifies auth, seeds reference data once, then drives the scenarios for the
// configured mode. Per-iteration failures aggregate into a non-nil error; a
// cancelled context aborts and is returned.
func (r *Runner) Run(ctx context.Context) error {
	if err := r.client.VerifyAuth(ctx); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Reference data is created once, before any scenario runs, so scenarios only
	// ever reference existing IDs.
	setupEnv := &scenario.Env{
		Client: r.client,
		Report: r.report,
		Rand:   rand.New(rand.NewSource(r.cfg.Seed)),
	}
	ref, err := scenario.EnsureRefData(ctx, setupEnv)
	if err != nil {
		return fmt.Errorf("reference data setup: %w", err)
	}

	if r.cfg.Mode == config.ModeLoad {
		return r.runLoad(ctx, ref)
	}

	setupEnv.RefIDs = ref
	return driveScenarios(ctx, setupEnv, mockSchedule(r.cfg.Volume()))
}

// newScenarioSet returns a fresh set of scenario instances, each with its own
// mutable iteration state, so a worker never races another on that state.
func newScenarioSet() []scenario.Scenario {
	return []scenario.Scenario{
		&scenario.PurchaseOrderScenario{},
		&scenario.PaymentScenario{},
		&scenario.ReconciliationScenario{},
	}
}

// runLoad drives scenarios concurrently through a worker pool. A single
// dispatcher emits iteration tokens onto a jobs channel; N workers each own an
// independent scenario set and RNG and share the concurrency-safe Client and
// Report, round-robining across their set. A cancelled context stops both.
func (r *Runner) runLoad(ctx context.Context, ref *scenario.RefData) error {
	workers := r.cfg.Concurrency
	if workers < 1 {
		workers = 1
	}

	jobs := make(chan struct{}, workers)
	start := time.Now()

	// Bound the run by duration (if set).
	runCtx := ctx
	var cancel context.CancelFunc
	if r.cfg.Duration > 0 {
		runCtx, cancel = context.WithTimeout(ctx, r.cfg.Duration)
		defer cancel()
	}

	budget := r.cfg.LoadVolume()
	go dispatch(runCtx, jobs, budget, r.cfg.Rate)

	var (
		wg          sync.WaitGroup
		mu          sync.Mutex
		failedCount int
		ranCount    int
	)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			set := r.newSet()
			env := &scenario.Env{
				Client: r.client,
				Report: r.report,
				Rand:   rand.New(rand.NewSource(r.cfg.Seed + int64(workerID) + 1)),
				RefIDs: ref,
			}
			i := 0
			for range jobs {
				if runCtx.Err() != nil {
					return
				}
				sc := set[i%len(set)]
				i++
				err := sc.Run(runCtx, env)
				mu.Lock()
				ranCount++
				// A cancellation error is the stop signal, not a failure; only
				// count errors while runCtx is still live.
				if err != nil && runCtx.Err() == nil {
					failedCount++
				}
				mu.Unlock()
			}
		}(w)
	}

	wg.Wait()
	if cancel != nil {
		cancel()
	}
	r.report.SetDuration(time.Since(start))

	if ctxErr := ctx.Err(); ctxErr != nil {
		// Parent-context cancel (user shutdown) exits non-zero; a duration timeout
		// cancels only runCtx, so this branch is skipped.
		return ctxErr
	}
	if failedCount > 0 {
		return fmt.Errorf("%d/%d load iterations failed", failedCount, ranCount)
	}
	return nil
}

// dispatch emits scenario tokens onto jobs and closes it when done. It stops at
// the volume budget (when volume > 0) or on context cancel; when volume <= 0 it
// runs until the context is cancelled. rate > 0 paces emission via a ticker.
func dispatch(ctx context.Context, jobs chan<- struct{}, volume int, rate float64) {
	defer close(jobs)

	var tick <-chan time.Time
	if rate > 0 {
		interval := time.Duration(float64(time.Second) / rate)
		if interval <= 0 {
			interval = time.Nanosecond
		}
		t := time.NewTicker(interval)
		defer t.Stop()
		tick = t.C
	}

	bounded := volume > 0
	for n := 0; !bounded || n < volume; n++ {
		if tick != nil {
			select {
			case <-ctx.Done():
				return
			case <-tick:
			}
		}
		select {
		case <-ctx.Done():
			return
		case jobs <- struct{}{}:
		}
	}
}

// scheduledScenario pairs a scenario with how many times to drive it.
type scheduledScenario struct {
	scenario scenario.Scenario
	runs     int
}

// mockSchedule builds the ordered scenario plan for a mock run, with counts
// derived from base (the configured volume) and set so even the smallest run
// exercises every scenario variant.
func mockSchedule(base int) []scheduledScenario {
	if base < 1 {
		base = 1
	}
	// At least 2 runs so a small run exercises both reconcile variants.
	reconRuns := base + 1
	payRuns := base
	if payRuns < 1 {
		payRuns = 1
	}
	return []scheduledScenario{
		{&scenario.PurchaseOrderScenario{}, base},
		{&scenario.PaymentScenario{}, payRuns},
		{&scenario.ReconciliationScenario{}, reconRuns},
	}
}

// driveScenarios runs each scheduled scenario its configured number of times,
// returning an aggregate error if any iteration failed and aborting on context
// cancel.
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

// driveScenario runs sc volume times. Per-iteration failures are logged but do
// not abort the loop; it returns an aggregate error if any failed, and aborts
// early on context cancel.
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
