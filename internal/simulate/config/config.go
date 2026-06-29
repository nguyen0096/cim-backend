// Package config parses simulation configuration from flags and environment
// variables. The simulate command is a developer tool that drives the CIM API
// over HTTP to seed mock data and (in later PRs) load-test the server.
package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Mode selects what the simulation does.
type Mode string

const (
	// ModeMock seeds a development database with realistic business data,
	// driving entities across their lifecycle states. Low concurrency,
	// deterministic where possible.
	ModeMock Mode = "mock"
	// ModeLoad stresses the API with many concurrent scenarios via a worker
	// pool, bounded by volume and/or duration and optionally rate-limited.
	ModeLoad Mode = "load"
)

// Scale is a coarse knob for how much data a run produces.
type Scale string

const (
	ScaleSmall  Scale = "small"
	ScaleMedium Scale = "medium"
	ScaleLarge  Scale = "large"
)

// Config holds all simulation settings.
type Config struct {
	// API
	BaseURL string
	Timeout time.Duration

	// Auth (Firebase email/password sign-in).
	Email          string
	Password       string
	FirebaseAPIKey string

	// Run shape.
	Mode  Mode
	Scale Scale
	Seed  int64

	// Load-mode knobs (ignored in mock mode).
	//
	// Concurrency is the number of worker goroutines driving scenarios in
	// parallel. Volume, when > 0, caps the total number of scenario iterations
	// the pool runs (0 = derive from --scale). Duration, when > 0, stops the run
	// after that wall-clock time regardless of volume (0 = run until volume is
	// reached). Rate, when > 0, caps the scenario start rate to that many
	// iterations per second across the whole pool (0 = unthrottled).
	Concurrency int
	VolumeFlag  int
	Duration    time.Duration
	Rate        float64

	// Output.
	JSONOutput bool

	// DryRun validates configuration and prints the plan without making any
	// network calls. Useful for CI / `--help`-style verification when no live
	// server or Firebase credentials are available.
	DryRun bool
}

// Default values and bounds shared by flags and tests.
const (
	DefaultBaseURL = "http://localhost:8080"
	DefaultTimeout = 30 * time.Second

	// DefaultConcurrency is the worker count for load mode when unset.
	DefaultConcurrency = 10
	// MaxConcurrency bounds the worker pool so a typo can't fork-bomb the host.
	MaxConcurrency = 1000
)

// volumeForScale returns the number of purchase-order lifecycles a run drives
// for the given scale. Kept here (not in the runner) so it is pure and unit
// testable, and so PR-2/PR-3 scenarios can reuse the same scaling.
func volumeForScale(s Scale) int {
	switch s {
	case ScaleLarge:
		return 50
	case ScaleMedium:
		return 15
	default: // ScaleSmall and unknown
		return 3
	}
}

// Volume is the scale-derived number of primary lifecycle iterations. It is
// always scale-driven and is what MOCK mode seeds; the --volume/SIM_VOLUME knob
// is load-mode-only (see LoadVolume) and deliberately does NOT change mock
// seeding.
func (c *Config) Volume() int {
	return volumeForScale(c.Scale)
}

// LoadVolume is the iteration budget for a LOAD run. An explicit --volume (> 0)
// wins. --volume 0 means "no volume cap": with a duration set the clock bounds
// the run (0 = unbounded), otherwise it falls back to the scale-derived volume
// so a bare `--mode load` still terminates. Only consulted in load mode.
func (c *Config) LoadVolume() int {
	if c.VolumeFlag > 0 {
		return c.VolumeFlag
	}
	if c.Duration > 0 {
		return 0 // unbounded; duration stops the run
	}
	return c.Volume() // scale fallback
}

// Validate checks required fields and normalizes values. It does not touch the
// network, so it is safe to call in DryRun mode and in tests.
func (c *Config) Validate() error {
	c.BaseURL = strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if c.BaseURL == "" {
		return fmt.Errorf("base URL is required (--base-url or SIM_BASE_URL)")
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive, got %s", c.Timeout)
	}

	switch c.Mode {
	case ModeMock, ModeLoad:
	default:
		return fmt.Errorf("invalid mode %q (want mock or load)", c.Mode)
	}

	switch c.Scale {
	case ScaleSmall, ScaleMedium, ScaleLarge:
	default:
		return fmt.Errorf("invalid scale %q (want small, medium or large)", c.Scale)
	}

	// Load-mode bounds. These are only enforced for load mode; mock mode is
	// always single-threaded and ignores the knobs.
	if c.VolumeFlag < 0 {
		return fmt.Errorf("volume must be >= 0, got %d", c.VolumeFlag)
	}
	if c.Mode == ModeLoad {
		if c.Concurrency < 1 || c.Concurrency > MaxConcurrency {
			return fmt.Errorf("concurrency must be between 1 and %d, got %d", MaxConcurrency, c.Concurrency)
		}
		if c.Rate < 0 {
			return fmt.Errorf("rate must be >= 0, got %g", c.Rate)
		}
		if c.Duration < 0 {
			return fmt.Errorf("duration must be >= 0, got %s", c.Duration)
		}
		// A load run always has a stop condition: an explicit --volume, a
		// --duration, or (failing both) the scale-derived volume fallback.
	}

	// Credentials are only required for a real run.
	if !c.DryRun {
		if c.Email == "" || c.Password == "" {
			return fmt.Errorf("email and password are required (--email/--password or SIM_EMAIL/SIM_PASSWORD)")
		}
		if c.FirebaseAPIKey == "" {
			return fmt.Errorf("Firebase web API key is required (FIREBASE_WEB_API_KEY)")
		}
	}
	return nil
}

// Parse builds a Config from the given args (typically os.Args[1:]) with
// environment-variable fallbacks. It does not call Validate.
func Parse(args []string) (*Config, error) {
	c := &Config{}
	fs := flag.NewFlagSet("simulate", flag.ContinueOnError)

	fs.StringVar(&c.BaseURL, "base-url", env("SIM_BASE_URL", DefaultBaseURL), "API base URL (without /api/v1)")
	fs.DurationVar(&c.Timeout, "timeout", envDuration("SIM_TIMEOUT", DefaultTimeout), "per-request HTTP timeout")
	fs.StringVar(&c.Email, "email", firstEnv("SIM_EMAIL", "FIREBASE_TEST_USER"), "login email")
	fs.StringVar(&c.Password, "password", firstEnv("SIM_PASSWORD", "FIREBASE_TEST_PASSWORD"), "login password")
	fs.StringVar(&c.FirebaseAPIKey, "firebase-api-key", env("FIREBASE_WEB_API_KEY", ""), "Firebase web API key")

	mode := fs.String("mode", env("SIM_MODE", string(ModeMock)), "mode: mock|load")
	scale := fs.String("scale", env("SIM_SCALE", string(ScaleSmall)), "scale: small|medium|large")
	fs.Int64Var(&c.Seed, "seed", envInt64("SIM_SEED", time.Now().UnixNano()), "RNG seed for reproducible runs")

	// Load-mode knobs (ignored in mock mode).
	fs.IntVar(&c.Concurrency, "concurrency", envInt("SIM_CONCURRENCY", DefaultConcurrency), "load mode: number of concurrent worker goroutines")
	fs.IntVar(&c.VolumeFlag, "volume", envInt("SIM_VOLUME", 0), "total scenario iterations to run (0 = derive from --scale)")
	fs.DurationVar(&c.Duration, "duration", envDuration("SIM_DURATION", 0), "load mode: stop after this wall-clock time (0 = run until volume reached)")
	fs.Float64Var(&c.Rate, "rate", envFloat("SIM_RATE", 0), "load mode: cap scenario starts per second across the pool (0 = unthrottled)")

	fs.BoolVar(&c.JSONOutput, "json", false, "emit report as JSON")
	fs.BoolVar(&c.DryRun, "dry-run", false, "validate config and print the plan without making network calls")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	c.Mode = Mode(strings.ToLower(strings.TrimSpace(*mode)))
	c.Scale = Scale(strings.ToLower(strings.TrimSpace(*scale)))
	return c, nil
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// firstEnv returns the first non-empty value among the given env keys.
func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func envInt64(key string, def int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}
