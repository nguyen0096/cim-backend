// Command simulate drives the CIM API over HTTP to seed mock data (and, in
// later PRs, load-test the server). It authenticates with a Firebase
// email/password sign-in, mints a Bearer token (refreshing on expiry/401), and
// drives full business lifecycles via /api/v1.
//
// Usage:
//
//	go run ./cmd/simulate --mode mock --scale small
//	go run ./cmd/simulate --dry-run            # validate config, no network
//
// Credentials come from flags or env (SIM_EMAIL/SIM_PASSWORD or the
// FIREBASE_TEST_USER/FIREBASE_TEST_PASSWORD pair, plus FIREBASE_WEB_API_KEY).
// A .env file in the working directory is loaded automatically.
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
	fmt.Printf("  scale    : %s (volume=%d PO lifecycles)\n", cfg.Scale, cfg.Volume())
	fmt.Printf("  timeout  : %s\n", cfg.Timeout)
	fmt.Printf("  seed     : %d\n", cfg.Seed)
	fmt.Println("  scenarios: refdata (idempotent), purchase_order (create -> receive -> inventory items)")
}
