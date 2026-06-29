package runner

import "testing"

// mockSchedule must drive every lifecycle, in the dependency-correct order
// (purchase_order first so reconciliation/sale-order have inventory items), and
// give each scenario enough runs to fan out all of its variants even at the
// smallest scale.
func TestMockScheduleCoversAllLifecyclesInOrder(t *testing.T) {
	plan := mockSchedule(3)

	wantOrder := []string{"purchase_order", "payment", "reconciliation", "sale_order"}
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

// Even the smallest run must give the 4-variant scenarios at least 4 runs so
// open/closed/processed (+ reopen) and ordered/served/completed/cancelled all
// appear.
func TestMockScheduleSmallStillFansAllVariants(t *testing.T) {
	plan := mockSchedule(1)
	byName := map[string]int{}
	for _, sc := range plan {
		byName[sc.scenario.Name()] = sc.runs
	}
	for _, name := range []string{"reconciliation", "sale_order"} {
		if byName[name] < 4 {
			t.Errorf("%s runs = %d at base=1, want >= 4 (its variant count)", name, byName[name])
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
