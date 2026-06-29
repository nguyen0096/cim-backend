// Package runner orchestrates a simulation run: it sets up shared reference
// data once (serialized), then drives scenarios.
//
// Mock mode runs the full set of lifecycle scenarios serially, weighted for
// broad state coverage. Load mode drives scenarios concurrently through a worker
// pool bounded by volume and/or duration and optionally rate-limited, and
// reports latency percentiles plus throughput. Both modes share the reference-
// data setup and the thread-safe report; a cancelled context (SIGINT/SIGTERM)
// stops either cleanly and the caller still prints the summary.
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
	// newSet builds a fresh, independent scenario set for one worker. A field so
	// tests can inject stub scenarios; defaults to newScenarioSet.
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

// Run verifies auth, seeds reference data once (serialized), then drives the
// scenarios for the configured mode. A cancelled context aborts the run and is
// returned; per-iteration failures are aggregated into a non-nil error so the
// caller exits non-zero while still printing the summary.
func (r *Runner) Run(ctx context.Context) error {
	if err := r.client.VerifyAuth(ctx); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Reference data is created once, serialized, before any scenario runs so
	// scenarios (including concurrent load-mode workers) only ever reference
	// existing IDs. A dedicated env is used here because the per-worker envs
	// built below each carry their own RNG.
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

// newScenarioSet returns a fresh, independent set of scenario instances. Each
// instance carries its own mutable iteration/nonce state, so a worker that owns
// its own set never races another worker on that state.
func newScenarioSet() []scenario.Scenario {
	return []scenario.Scenario{
		&scenario.PurchaseOrderScenario{},
		&scenario.PaymentScenario{},
		&scenario.ReconciliationScenario{},
		&scenario.SaleOrderScenario{},
	}
}

// runLoad drives scenarios concurrently through a worker pool.
//
// Concurrency model:
//   - A buffered jobs channel carries one token per scenario iteration. A single
//     dispatcher goroutine emits up to Volume tokens (or unbounded when only a
//     duration is set), optionally paced by a token-bucket rate limiter, and
//     closes the channel when the budget is spent or the context is cancelled.
//   - N workers each own an independent scenario set and RNG (so no per-scenario
//     mutable state or RNG is shared) and a shared, concurrency-safe Client and
//     Report. Each token a worker pulls drives one scenario, round-robining
//     across the set so every lifecycle is exercised.
//   - The context (SIGINT/SIGTERM) cancels dispatcher and workers; the report is
//     still complete and the caller prints the summary.
//
// Per-entity contention is avoided structurally: the reconciliation scenario
// uses a per-instance, process-unique nonce for its dedicated inventories (so
// concurrent workers never collide on the one-active-pending guard), the
// purchase-order/payment scenarios own their own POs per iteration, and the
// idempotent ref-data + selling-price seeding tolerate 409 races.
func (r *Runner) runLoad(ctx context.Context, ref *scenario.RefData) error {
	workers := r.cfg.Concurrency
	if workers < 1 {
		workers = 1
	}

	jobs := make(chan struct{}, workers)
	start := time.Now()

	// Bound the run by duration (if set) on top of the parent context.
	runCtx := ctx
	var cancel context.CancelFunc
	if r.cfg.Duration > 0 {
		runCtx, cancel = context.WithTimeout(ctx, r.cfg.Duration)
		defer cancel()
	}

	// Dispatcher: emit one token per iteration up to the volume budget, paced by
	// the rate limiter. An explicit --volume (>0) is the budget; --volume 0 with
	// a duration means "unbounded, stop on the clock"; a bare load run (no volume,
	// no duration) falls back to the scale-derived volume so it still terminates.
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
				// An error caused by runCtx being cancelled (a duration timeout
				// or a user SIGINT that landed mid-iteration) is the STOP
				// signal, not a scenario failure: don't count it. A genuine
				// scenario error (runCtx still live) is a real failure. The
				// duration-timeout-vs-user-cancel distinction is made after the
				// pool drains, from the parent ctx.
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
		// User-requested shutdown (SIGINT/SIGTERM) on the PARENT context: exit
		// non-zero. A duration timeout cancels only the child runCtx, so the
		// parent stays live and this branch is skipped (normal exit-0 stop).
		return ctxErr
	}
	if failedCount > 0 {
		return fmt.Errorf("%d/%d load iterations failed", failedCount, ranCount)
	}
	return nil
}

// dispatch emits scenario tokens onto jobs and closes it when done. It stops at
// the volume budget (when volume > 0), or on context cancel; when volume <= 0 it
// runs unbounded until the (duration-bounded) context is cancelled. When rate >
// 0 it paces emission with a time.Ticker token bucket; otherwise it emits as
// fast as workers consume.
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

	// bounded is true when a finite volume caps the run. When false (volume <= 0)
	// the duration-bounded context is the only stop condition.
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
