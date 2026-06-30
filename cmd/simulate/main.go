// Command simulate drives the CIM API over HTTP in two modes: seeding mock data
// (broad lifecycle-state coverage) and load testing the server. It authenticates
// with a Firebase email/password sign-in, mints a Bearer token (refreshing on
// expiry/401), and drives full business lifecycles via /api/v1.
//
// Usage:
//
//	go run ./cmd/simulate --mode mock --scale small
//	go run ./cmd/simulate --dry-run            # validate config, no network
//
// Load mode runs scenarios concurrently through a worker pool, bounded by a
// volume budget and/or a wall-clock duration, optionally rate-limited, and
// prints latency percentiles (p50/p95/p99/max) plus throughput. --json emits the
// whole summary machine-readably.
//
//	# 50 workers, 5000 iterations
//	go run ./cmd/simulate --mode load --concurrency 50 --volume 5000
//	# 30s soak at 200 starts/sec, JSON summary
//	go run ./cmd/simulate --mode load --concurrency 50 --duration 30s --rate 200 --json
//
// Knobs (flag / SIM_* env): --concurrency (workers, def 10, 1..1000), --volume
// (total iterations; 0 = derive from --scale, or unbounded when only --duration
// is set), --duration (stop on the clock), --rate (cap iteration starts/sec
// across the pool; 0 = unthrottled). SIGINT/SIGTERM cancels the pool gracefully
// and still prints the summary; the process exits non-zero if any iteration
// failed.
//
// Concurrency safety: the report and token provider are mutex-safe; each worker
// owns its own scenario instances (so per-scenario iteration state is never
// shared) and its own RNG. All traffic targets one shared inventory; since only
// one active-pending reconcile is allowed per inventory, the reconcile lifecycle
// is serialized by a package mutex (each runs to a terminal state). The PO/payment
// scenarios own their POs per iteration; idempotent ref-data seeding tolerates
// 409 races.
//
// Docker resource probe (the point of load mode is to surface OOM/bottlenecks):
// bring the API up under a memory cap and watch it while the sim hammers it.
//
//	# docker-compose.yml: set a limit on the api service, e.g.
//	#   api:
//	#     mem_limit: 512m
//	#     deploy: { resources: { limits: { memory: 512m } } }
//	docker compose up -d api
//	docker stats $(docker compose ps -q api)         # in another terminal
//	go run ./cmd/simulate --mode load --concurrency 100 --duration 2m --json
//
// Credentials come from flags or env (SIM_EMAIL/SIM_PASSWORD or the
// FIREBASE_TEST_USER/FIREBASE_TEST_PASSWORD pair, plus FIREBASE_WEB_API_KEY).
// A .env file in the working directory is loaded automatically. Seeding is
// additive (idempotent ref-data, no cleanup) so re-runs are safe.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"cim-backend/internal/simulate/config"
	"cim-backend/internal/simulate/runner"

	"github.com/joho/godotenv"
)

func main() {
	// Best-effort .env load (mirrors cmd/auth); missing file is fine.
	_ = godotenv.Load()

	cfg, err := config.Parse(os.Args[1:])
	if err != nil {
		// -h/--help is a successful, intentional invocation: usage was already
		// printed, exit 0. Any other parse error is a usage error: exit 2.
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		os.Exit(2)
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(2)
	}

	if cfg.DryRun {
		printPlan(cfg)
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	run, rep := runner.New(cfg)
	runErr := run.Run(ctx)

	fmt.Println(rep.Snapshot().String(cfg.JSONOutput))

	if runErr != nil {
		fmt.Fprintf(os.Stderr, "simulation error: %v\n", runErr)
		os.Exit(1)
	}
}

func printPlan(cfg *config.Config) {
	fmt.Println("Simulation plan (dry run, no network calls):")
	fmt.Printf("  base URL : %s%s\n", cfg.BaseURL, "/api/v1")
	fmt.Printf("  mode     : %s\n", cfg.Mode)
	fmt.Printf("  scale    : %s (base volume=%d lifecycles)\n", cfg.Scale, cfg.Volume())
	fmt.Printf("  timeout  : %s\n", cfg.Timeout)
	fmt.Printf("  seed     : %d\n", cfg.Seed)
	if cfg.Mode == config.ModeLoad {
		dur := "until volume reached"
		if cfg.Duration > 0 {
			dur = cfg.Duration.String()
		}
		rate := "unthrottled"
		if cfg.Rate > 0 {
			rate = fmt.Sprintf("%g/s", cfg.Rate)
		}
		vol := fmt.Sprintf("%d", cfg.LoadVolume())
		if cfg.LoadVolume() == 0 {
			vol = "unbounded (clock-bounded)"
		}
		fmt.Printf("  load     : concurrency=%d volume=%s duration=%s rate=%s\n",
			cfg.Concurrency, vol, dur, rate)
	}
	fmt.Println("  scenarios: refdata (idempotent), then weighted for broad state coverage:")
	fmt.Println("    - purchase_order : create -> receive -> inventory items (order_placed/partially_delivered/fully_delivered/cancelled)")
	fmt.Println("    - payment        : PO + receive -> form (pending->approved) -> PO completed -> revenue-expense finalize (skipped if not configured)")
	fmt.Println("    - reconciliation : shared inventory (serialized) -> initiate -> labeled count -> close [-> reopen -> adjust(update) -> close] -> start-processing (processed/drift)")
}
