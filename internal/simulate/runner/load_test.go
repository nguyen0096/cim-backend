package runner

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"cim-backend/internal/simulate/config"
	"cim-backend/internal/simulate/report"
	"cim-backend/internal/simulate/scenario"
)

// countingScenario records every Run call (atomically, since workers call it
// concurrently) and optionally fails. It carries no shared mutable state beyond
// the shared counter, so -race exercises the worker pool + report under load.
type countingScenario struct {
	name string
	runs *atomic.Int64
	rep  *report.Report
	fail bool
}

func (c *countingScenario) Name() string { return c.name }

func (c *countingScenario) Run(_ context.Context, _ *scenario.Env) error {
	c.runs.Add(1)
	if c.fail {
		c.rep.RecordCall(c.name, 500, time.Millisecond, errors.New("boom"))
		return errors.New("boom")
	}
	c.rep.RecordCall(c.name, 200, time.Millisecond, nil)
	return nil
}

// loadRunner builds a Runner wired to a counting scenario set (no network). The
// returned counter totals every scenario Run across all workers.
func loadRunner(cfg *config.Config, fail bool) (*Runner, *report.Report, *atomic.Int64) {
	rep := report.New()
	var runs atomic.Int64
	r := &Runner{
		cfg:    cfg,
		report: rep,
		newSet: func() []scenario.Scenario {
			return []scenario.Scenario{
				&countingScenario{name: "a", runs: &runs, rep: rep},
				&countingScenario{name: "b", runs: &runs, rep: rep},
			}
		},
	}
	if fail {
		r.newSet = func() []scenario.Scenario {
			return []scenario.Scenario{&countingScenario{name: "a", runs: &runs, rep: rep, fail: true}}
		}
	}
	return r, rep, &runs
}

func TestRunLoadHitsExactVolume(t *testing.T) {
	cfg := &config.Config{Mode: config.ModeLoad, Concurrency: 8, VolumeFlag: 200}
	r, rep, runs := loadRunner(cfg, false)

	if err := r.runLoad(context.Background(), &scenario.RefData{}); err != nil {
		t.Fatalf("runLoad: %v", err)
	}
	if got := runs.Load(); got != 200 {
		t.Errorf("ran %d scenarios, want exactly 200", got)
	}
	snap := rep.Snapshot()
	if snap.TotalCalls != 200 {
		t.Errorf("report calls = %d, want 200", snap.TotalCalls)
	}
	if snap.DurationSec <= 0 {
		t.Errorf("DurationSec = %v, want > 0 (load mode reports throughput)", snap.DurationSec)
	}
}

func TestRunLoadAggregatesFailures(t *testing.T) {
	cfg := &config.Config{Mode: config.ModeLoad, Concurrency: 4, VolumeFlag: 50}
	r, rep, runs := loadRunner(cfg, true)

	err := r.runLoad(context.Background(), &scenario.RefData{})
	if err == nil {
		t.Fatal("expected aggregate error when every iteration fails")
	}
	if got := runs.Load(); got != 50 {
		t.Errorf("ran %d scenarios, want 50", got)
	}
	if snap := rep.Snapshot(); snap.TotalFailures != 50 {
		t.Errorf("report failures = %d, want 50", snap.TotalFailures)
	}
}

// A duration-bounded run (no volume cap) must stop on the clock and return nil
// when the parent context was never cancelled (timeout is a normal stop).
func TestRunLoadStopsOnDuration(t *testing.T) {
	cfg := &config.Config{Mode: config.ModeLoad, Concurrency: 4, VolumeFlag: 0, Duration: 60 * time.Millisecond}
	r, _, runs := loadRunner(cfg, false)

	start := time.Now()
	if err := r.runLoad(context.Background(), &scenario.RefData{}); err != nil {
		t.Fatalf("duration-bounded run returned %v, want nil", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("run took %s, expected to stop near the 60ms duration", elapsed)
	}
	if runs.Load() == 0 {
		t.Error("expected some scenarios to run before the duration elapsed")
	}
}

// blockingScenario blocks inside Run until its context is cancelled, then
// returns the context error — modelling a scenario caught mid HTTP request when
// the duration timeout (or a SIGINT) fires.
type blockingScenario struct {
	name string
	runs *atomic.Int64
}

func (b *blockingScenario) Name() string { return b.name }

func (b *blockingScenario) Run(ctx context.Context, _ *scenario.Env) error {
	b.runs.Add(1)
	<-ctx.Done()
	return ctx.Err() // e.g. context.DeadlineExceeded on a duration timeout
}

// Regression for the P2: when --duration expires while workers are mid-Run, the
// scenario returns a context error. That timeout is the STOP condition, not a
// failure: the run must report SUCCESS (nil error -> exit 0).
func TestRunLoadDurationTimeoutMidRunIsNotAFailure(t *testing.T) {
	var runs atomic.Int64
	cfg := &config.Config{Mode: config.ModeLoad, Concurrency: 4, VolumeFlag: 0, Duration: 40 * time.Millisecond}
	r := &Runner{
		cfg:    cfg,
		report: report.New(),
		newSet: func() []scenario.Scenario {
			return []scenario.Scenario{&blockingScenario{name: "block", runs: &runs}}
		},
	}

	if err := r.runLoad(context.Background(), &scenario.RefData{}); err != nil {
		t.Fatalf("duration timeout mid-Run returned %v, want nil (timeout is a normal exit-0 stop)", err)
	}
	if runs.Load() == 0 {
		t.Error("expected workers to be inside Run when the duration fired")
	}
}

// A user SIGINT (parent ctx cancel) that lands mid-Run must still surface
// context.Canceled (exit 1), distinct from a duration timeout.
func TestRunLoadUserCancelMidRunStillFails(t *testing.T) {
	var runs atomic.Int64
	cfg := &config.Config{Mode: config.ModeLoad, Concurrency: 4, VolumeFlag: 0, Duration: time.Hour}
	r := &Runner{
		cfg:    cfg,
		report: report.New(),
		newSet: func() []scenario.Scenario {
			return []scenario.Scenario{&blockingScenario{name: "block", runs: &runs}}
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	if err := r.runLoad(ctx, &scenario.RefData{}); !errors.Is(err, context.Canceled) {
		t.Errorf("user-cancel mid-Run err = %v, want context.Canceled (exit 1)", err)
	}
}

// A cancelled parent context (SIGINT) must stop the pool and surface
// context.Canceled so the caller exits non-zero, while the summary stays
// printable.
func TestRunLoadGracefulShutdownOnCancel(t *testing.T) {
	cfg := &config.Config{Mode: config.ModeLoad, Concurrency: 4, VolumeFlag: 1_000_000}
	r, _, _ := loadRunner(cfg, false)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	err := r.runLoad(ctx, &scenario.RefData{})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled on SIGINT", err)
	}
}

func TestRunLoadRateLimited(t *testing.T) {
	// 100/s for ~100ms should emit on the order of 10 iterations, not the full
	// volume. Assert the rate clearly throttled below the volume cap.
	cfg := &config.Config{Mode: config.ModeLoad, Concurrency: 4, VolumeFlag: 10_000, Rate: 100, Duration: 120 * time.Millisecond}
	r, _, runs := loadRunner(cfg, false)

	if err := r.runLoad(context.Background(), &scenario.RefData{}); err != nil {
		t.Fatalf("runLoad: %v", err)
	}
	if got := runs.Load(); got >= 200 {
		t.Errorf("ran %d scenarios under 100/s for 120ms, want far fewer (rate not enforced)", got)
	}
}

// dispatch must emit exactly volume tokens when bounded and unthrottled.
func TestDispatchBoundedEmitsVolume(t *testing.T) {
	jobs := make(chan struct{}, 4)
	go dispatch(context.Background(), jobs, 25, 0)

	n := 0
	for range jobs {
		n++
	}
	if n != 25 {
		t.Errorf("dispatch emitted %d tokens, want 25", n)
	}
}

// dispatch with volume 0 (unbounded) must stop and close jobs when the context
// is cancelled.
func TestDispatchUnboundedStopsOnCancel(t *testing.T) {
	jobs := make(chan struct{}, 4)
	ctx, cancel := context.WithCancel(context.Background())
	go dispatch(ctx, jobs, 0, 0)

	// Drain a few then cancel; the channel must eventually close.
	go func() {
		<-jobs
		<-jobs
		cancel()
	}()

	done := make(chan struct{})
	go func() {
		for range jobs { //nolint:revive // draining until closed
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("dispatch did not close jobs after context cancel")
	}
}
