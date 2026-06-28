package report

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestRecordAndSnapshot(t *testing.T) {
	r := New()
	r.RecordCall("POST /x", 201, 10*time.Millisecond, nil)
	r.RecordCall("POST /x", 500, 20*time.Millisecond, errors.New("boom"))
	r.Created("widget")
	r.Created("widget")

	s := r.Snapshot()
	if s.Calls["POST /x"] != 2 {
		t.Errorf("calls = %d, want 2", s.Calls["POST /x"])
	}
	if s.TotalCalls != 2 {
		t.Errorf("total = %d, want 2", s.TotalCalls)
	}
	if s.TotalFailures != 1 {
		t.Errorf("failures = %d, want 1", s.TotalFailures)
	}
	if s.Created["widget"] != 2 {
		t.Errorf("created widget = %d, want 2", s.Created["widget"])
	}
	if len(s.Failures) != 1 || s.Failures[0].Status != 500 {
		t.Errorf("unexpected failures: %+v", s.Failures)
	}
}

func TestLatencyPercentiles(t *testing.T) {
	r := New()
	for i := 1; i <= 100; i++ {
		r.RecordCall("e", 200, time.Duration(i)*time.Millisecond, nil)
	}
	s := r.Snapshot()
	if s.Latency.Count != 100 {
		t.Errorf("count = %d, want 100", s.Latency.Count)
	}
	if s.Latency.Maxms != 100 {
		t.Errorf("max = %.1f, want 100", s.Latency.Maxms)
	}
	// nearest-rank p50 of 1..100ms -> ~51ms
	if s.Latency.P50ms < 45 || s.Latency.P50ms > 55 {
		t.Errorf("p50 = %.1f, want ~51", s.Latency.P50ms)
	}
	if s.Latency.P95ms < 90 || s.Latency.P95ms > 99 {
		t.Errorf("p95 = %.1f, want ~96", s.Latency.P95ms)
	}
}

func TestFailureCap(t *testing.T) {
	r := New()
	for i := 0; i < 100; i++ {
		r.RecordCall("e", 500, time.Millisecond, errors.New("x"))
	}
	s := r.Snapshot()
	if len(s.Failures) != 50 {
		t.Errorf("sampled failures = %d, want capped at 50", len(s.Failures))
	}
	if s.TotalFailures != 100 {
		t.Errorf("total failures = %d, want 100", s.TotalFailures)
	}
}

func TestConcurrentRecord(t *testing.T) {
	r := New()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.RecordCall("e", 200, time.Millisecond, nil)
			r.Created("c")
		}()
	}
	wg.Wait()
	s := r.Snapshot()
	if s.TotalCalls != 50 || s.Created["c"] != 50 {
		t.Errorf("calls=%d created=%d, want 50/50", s.TotalCalls, s.Created["c"])
	}
}

func TestSummaryStringJSON(t *testing.T) {
	r := New()
	r.RecordCall("e", 200, time.Millisecond, nil)
	s := r.Snapshot()
	if out := s.String(true); out == "" || out[0] != '{' {
		t.Errorf("JSON output malformed: %q", out)
	}
	if out := s.String(false); out == "" {
		t.Errorf("text output empty")
	}
}
