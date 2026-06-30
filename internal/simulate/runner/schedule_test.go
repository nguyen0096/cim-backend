package runner

import (
	"testing"

	"cim-backend/internal/simulate/config"
)

// Mock seeding must stay scale-driven and ignore the load-only --volume/SIM_VOLUME
// knob: a leftover SIM_VOLUME must NOT blow up a mock run. mockSchedule is fed
// cfg.Volume() (scale-derived), never cfg.LoadVolume().
func TestMockScheduleIgnoresVolumeFlag(t *testing.T) {
	scaleOnly := &config.Config{Scale: config.ScaleSmall}
	withFlag := &config.Config{Scale: config.ScaleSmall, VolumeFlag: 5000}

	base := mockSchedule(scaleOnly.Volume())
	flagged := mockSchedule(withFlag.Volume())

	if len(base) != len(flagged) {
		t.Fatalf("schedule length changed with --volume: %d vs %d", len(base), len(flagged))
	}
	for i := range base {
		if base[i].runs != flagged[i].runs {
			t.Errorf("%s runs changed with --volume: %d -> %d (mock must stay scale-driven)",
				base[i].scenario.Name(), base[i].runs, flagged[i].runs)
		}
	}
}

// mockSchedule must drive every lifecycle, in the dependency-correct order
// (purchase_order first so reconciliation has inventory items), and give each
// scenario enough runs to fan out all of its variants even at the smallest
// scale.
func TestMockScheduleCoversAllLifecyclesInOrder(t *testing.T) {
	plan := mockSchedule(3)

	wantOrder := []string{"purchase_order", "payment", "reconciliation"}
	if len(plan) != len(wantOrder) {
		t.Fatalf("plan has %d scenarios, want %d", len(plan), len(wantOrder))
	}
	for i, want := range wantOrder {
		if got := plan[i].scenario.Name(); got != want {
			t.Errorf("plan[%d] = %q, want %q (order is load-bearing)", i, got, want)
		}
		if plan[i].runs < 1 {
			t.Errorf("plan[%d] (%s) has %d runs, want >= 1", i, want, plan[i].runs)
		}
	}
}

// Even the smallest run must give the reconciliation scenario at least 2 runs so
// both terminal variants (straight, reopen+adjust) appear.
func TestMockScheduleSmallStillFansAllVariants(t *testing.T) {
	plan := mockSchedule(1)
	byName := map[string]int{}
	for _, sc := range plan {
		byName[sc.scenario.Name()] = sc.runs
	}
	for _, name := range []string{"reconciliation"} {
		if byName[name] < 2 {
			t.Errorf("%s runs = %d at base=1, want >= 2 (its variant count)", name, byName[name])
		}
	}
}

// A non-positive base must not produce a zero/negative run count.
func TestMockScheduleClampsBase(t *testing.T) {
	for _, base := range []int{0, -5} {
		for _, sc := range mockSchedule(base) {
			if sc.runs < 1 {
				t.Errorf("base=%d: %s runs = %d, want >= 1", base, sc.scenario.Name(), sc.runs)
			}
		}
	}
}
