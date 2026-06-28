package runner

import (
	"context"
	"errors"
	"testing"

	"cim-backend/internal/simulate/report"
	"cim-backend/internal/simulate/scenario"
)

// stubScenario fails the first failFirst of its Run calls, then succeeds. Each
// failed call records a failure in the report so we can assert it propagated.
type stubScenario struct {
	failFirst int
	calls     int
	rep       *report.Report
}

func (s *stubScenario) Name() string { return "stub" }

func (s *stubScenario) Run(_ context.Context, _ *scenario.Env) error {
	s.calls++
	if s.calls <= s.failFirst {
		s.rep.RecordCall("stub", 500, 0, errors.New("boom"))
		return errors.New("boom")
	}
	s.rep.RecordCall("stub", 200, 0, nil)
	return nil
}

func TestDriveScenarioAggregatesFailures(t *testing.T) {
	rep := report.New()
	env := &scenario.Env{Report: rep}
	const volume, fail = 5, 2
	sc := &stubScenario{failFirst: fail, rep: rep}

	err := driveScenario(context.Background(), env, sc, volume)
	if err == nil {
		t.Fatal("expected aggregate error when iterations fail")
	}

	// Loop must complete all iterations despite early failures.
	if sc.calls != volume {
		t.Errorf("scenario ran %d times, want %d (must not abort early)", sc.calls, volume)
	}
	// Report still records the failures.
	snap := rep.Snapshot()
	if snap.TotalFailures != fail {
		t.Errorf("report failures = %d, want %d", snap.TotalFailures, fail)
	}
	if snap.TotalCalls != volume {
		t.Errorf("report calls = %d, want %d", snap.TotalCalls, volume)
	}
}

func TestDriveScenarioAllSuccessReturnsNil(t *testing.T) {
	rep := report.New()
	env := &scenario.Env{Report: rep}
	sc := &stubScenario{failFirst: 0, rep: rep}

	if err := driveScenario(context.Background(), env, sc, 4); err != nil {
		t.Errorf("all-success run returned %v, want nil", err)
	}
	if sc.calls != 4 {
		t.Errorf("scenario ran %d times, want 4", sc.calls)
	}
}

func TestDriveScenarioContextCancel(t *testing.T) {
	rep := report.New()
	env := &scenario.Env{Report: rep}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancelled

	sc := &stubScenario{failFirst: 0, rep: rep}
	err := driveScenario(ctx, env, sc, 5)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if sc.calls != 0 {
		t.Errorf("scenario ran %d times after cancel, want 0", sc.calls)
	}
}
