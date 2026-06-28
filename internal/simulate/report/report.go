// Package report aggregates the outcome of a simulation run: per-entity created
// counts, per-endpoint call counts, failures, and request latency. It is
// thread-safe so the runner can fan out across goroutines (PR-3) without each
// scenario having to track its own stats.
package report

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Failure captures a single failed request for the summary.
type Failure struct {
	Endpoint string `json:"endpoint"`
	Status   int    `json:"status"`
	Message  string `json:"message"`
}

// Report accumulates run statistics.
type Report struct {
	mu sync.Mutex

	created       map[string]int
	calls         map[string]int
	failures      []Failure
	totalFailures int
	latencies     []time.Duration
	maxFailures   int
}

// New returns an empty Report.
func New() *Report {
	return &Report{
		created:     map[string]int{},
		calls:       map[string]int{},
		maxFailures: 50, // cap sampled failures so a flood of errors stays printable
	}
}

// RecordCall records one request outcome: its endpoint label, HTTP status,
// latency, and (if it failed) a sampled error message.
func (r *Report) RecordCall(endpoint string, status int, latency time.Duration, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls[endpoint]++
	r.latencies = append(r.latencies, latency)
	if err != nil {
		r.totalFailures++
		if len(r.failures) < r.maxFailures {
			r.failures = append(r.failures, Failure{Endpoint: endpoint, Status: status, Message: truncate(err.Error(), 300)})
		}
	}
}

// Created bumps the created-count for an entity type (e.g. "purchase_order").
func (r *Report) Created(entity string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.created[entity]++
}

// Summary is a serializable snapshot of the run.
type Summary struct {
	Created       map[string]int `json:"created"`
	Calls         map[string]int `json:"calls"`
	TotalCalls    int            `json:"total_calls"`
	TotalFailures int            `json:"total_failures"`
	Failures      []Failure      `json:"failures,omitempty"`
	Latency       LatencyStats   `json:"latency"`
}

// LatencyStats holds simple latency aggregates (ms).
type LatencyStats struct {
	Count int     `json:"count"`
	P50ms float64 `json:"p50_ms"`
	P95ms float64 `json:"p95_ms"`
	Maxms float64 `json:"max_ms"`
}

// Snapshot returns an immutable view of the current stats.
func (r *Report) Snapshot() Summary {
	r.mu.Lock()
	defer r.mu.Unlock()

	created := make(map[string]int, len(r.created))
	for k, v := range r.created {
		created[k] = v
	}
	calls := make(map[string]int, len(r.calls))
	total := 0
	for k, v := range r.calls {
		calls[k] = v
		total += v
	}
	failures := make([]Failure, len(r.failures))
	copy(failures, r.failures)

	return Summary{
		Created:       created,
		Calls:         calls,
		TotalCalls:    total,
		TotalFailures: r.totalFailures,
		Failures:      failures,
		Latency:       latencyStats(r.latencies),
	}
}

func latencyStats(ds []time.Duration) LatencyStats {
	if len(ds) == 0 {
		return LatencyStats{}
	}
	sorted := make([]time.Duration, len(ds))
	copy(sorted, ds)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return LatencyStats{
		Count: len(sorted),
		P50ms: ms(percentile(sorted, 0.50)),
		P95ms: ms(percentile(sorted, 0.95)),
		Maxms: ms(sorted[len(sorted)-1]),
	}
}

// percentile returns the p-quantile (0..1) of a sorted slice using
// nearest-rank.
func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(p * float64(len(sorted)))
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000.0 }

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// Print writes a human-readable summary to w. If asJSON is true it emits JSON
// instead.
func (s Summary) String(asJSON bool) string {
	if asJSON {
		b, _ := json.MarshalIndent(s, "", "  ")
		return string(b)
	}

	out := "\n=== Simulation report ===\n"
	out += "\nCreated:\n"
	for _, k := range sortedKeys(s.Created) {
		out += fmt.Sprintf("  %-20s %d\n", k, s.Created[k])
	}
	out += "\nCalls by endpoint:\n"
	for _, k := range sortedKeys(s.Calls) {
		out += fmt.Sprintf("  %-40s %d\n", k, s.Calls[k])
	}
	out += fmt.Sprintf("\nTotal calls: %d  failures: %d\n", s.TotalCalls, s.TotalFailures)
	out += fmt.Sprintf("Latency (ms): p50=%.1f p95=%.1f max=%.1f (n=%d)\n",
		s.Latency.P50ms, s.Latency.P95ms, s.Latency.Maxms, s.Latency.Count)
	if len(s.Failures) > 0 {
		out += "\nFailures (sampled):\n"
		for _, f := range s.Failures {
			out += fmt.Sprintf("  [%d] %s: %s\n", f.Status, f.Endpoint, f.Message)
		}
	}
	return out
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
