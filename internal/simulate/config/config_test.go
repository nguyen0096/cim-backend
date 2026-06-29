package config

import (
	"testing"
	"time"
)

func TestParseDefaults(t *testing.T) {
	t.Setenv("SIM_BASE_URL", "")
	t.Setenv("SIM_EMAIL", "")
	t.Setenv("SIM_PASSWORD", "")
	t.Setenv("FIREBASE_TEST_USER", "")
	t.Setenv("FIREBASE_TEST_PASSWORD", "")
	t.Setenv("FIREBASE_WEB_API_KEY", "")
	t.Setenv("SIM_MODE", "")
	t.Setenv("SIM_SCALE", "")

	c, err := Parse(nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if c.BaseURL != DefaultBaseURL {
		t.Errorf("BaseURL = %q, want %q", c.BaseURL, DefaultBaseURL)
	}
	if c.Mode != ModeMock {
		t.Errorf("Mode = %q, want %q", c.Mode, ModeMock)
	}
	if c.Scale != ScaleSmall {
		t.Errorf("Scale = %q, want %q", c.Scale, ScaleSmall)
	}
	if c.Timeout != DefaultTimeout {
		t.Errorf("Timeout = %s, want %s", c.Timeout, DefaultTimeout)
	}
}

func TestParseFlagsOverrideEnv(t *testing.T) {
	t.Setenv("SIM_BASE_URL", "http://env:9999")
	c, err := Parse([]string{"--base-url", "http://flag:1234/", "--mode", "mock", "--scale", "large", "--timeout", "5s"})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := c.Validate(); err != nil {
		// credentials missing but DryRun false -> expect a creds error, not a base-url one
		if c.BaseURL != "http://flag:1234" {
			t.Errorf("BaseURL = %q, want trimmed flag value", c.BaseURL)
		}
	}
	if c.Scale != ScaleLarge {
		t.Errorf("Scale = %q, want large", c.Scale)
	}
	if c.Timeout != 5*time.Second {
		t.Errorf("Timeout = %s, want 5s", c.Timeout)
	}
	if c.Volume() != 50 {
		t.Errorf("Volume = %d, want 50 for large", c.Volume())
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name:    "dry run needs no creds",
			cfg:     Config{BaseURL: "http://x", Timeout: time.Second, Mode: ModeMock, Scale: ScaleSmall, DryRun: true},
			wantErr: false,
		},
		{
			name:    "real run needs creds",
			cfg:     Config{BaseURL: "http://x", Timeout: time.Second, Mode: ModeMock, Scale: ScaleSmall},
			wantErr: true,
		},
		{
			name:    "real run with creds ok",
			cfg:     Config{BaseURL: "http://x", Timeout: time.Second, Mode: ModeMock, Scale: ScaleSmall, Email: "a@b.c", Password: "p", FirebaseAPIKey: "k"},
			wantErr: false,
		},
		{
			name:    "load mode with valid knobs ok",
			cfg:     Config{BaseURL: "http://x", Timeout: time.Second, Mode: ModeLoad, Scale: ScaleSmall, DryRun: true, Concurrency: 10},
			wantErr: false,
		},
		{
			name:    "load mode zero concurrency rejected",
			cfg:     Config{BaseURL: "http://x", Timeout: time.Second, Mode: ModeLoad, Scale: ScaleSmall, DryRun: true, Concurrency: 0},
			wantErr: true,
		},
		{
			name:    "load mode concurrency over max rejected",
			cfg:     Config{BaseURL: "http://x", Timeout: time.Second, Mode: ModeLoad, Scale: ScaleSmall, DryRun: true, Concurrency: MaxConcurrency + 1},
			wantErr: true,
		},
		{
			name:    "load mode negative rate rejected",
			cfg:     Config{BaseURL: "http://x", Timeout: time.Second, Mode: ModeLoad, Scale: ScaleSmall, DryRun: true, Concurrency: 1, Rate: -1},
			wantErr: true,
		},
		{
			name:    "negative volume rejected",
			cfg:     Config{BaseURL: "http://x", Timeout: time.Second, Mode: ModeMock, Scale: ScaleSmall, DryRun: true, VolumeFlag: -1},
			wantErr: true,
		},
		{
			name:    "bad mode",
			cfg:     Config{BaseURL: "http://x", Timeout: time.Second, Mode: "nope", Scale: ScaleSmall, DryRun: true},
			wantErr: true,
		},
		{
			name:    "bad scale",
			cfg:     Config{BaseURL: "http://x", Timeout: time.Second, Mode: ModeMock, Scale: "huge", DryRun: true},
			wantErr: true,
		},
		{
			name:    "empty base url",
			cfg:     Config{BaseURL: "", Timeout: time.Second, Mode: ModeMock, Scale: ScaleSmall, DryRun: true},
			wantErr: true,
		},
		{
			name:    "non-positive timeout",
			cfg:     Config{BaseURL: "http://x", Timeout: 0, Mode: ModeMock, Scale: ScaleSmall, DryRun: true},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.cfg
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateTrimsBaseURL(t *testing.T) {
	cfg := Config{BaseURL: "  http://x:8080/  ", Timeout: time.Second, Mode: ModeMock, Scale: ScaleSmall, DryRun: true}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.BaseURL != "http://x:8080" {
		t.Errorf("BaseURL = %q, want trimmed", cfg.BaseURL)
	}
}

// Volume() is what mock mode seeds: it must stay scale-driven and IGNORE the
// load-only --volume/SIM_VOLUME knob, so a leftover SIM_VOLUME can't blow up a
// mock seed. LoadVolume() is where the knob takes effect.
func TestVolumeIsScaleDrivenAndIgnoresVolumeFlag(t *testing.T) {
	c := Config{Scale: ScaleSmall, VolumeFlag: 0}
	if got := c.Volume(); got != 3 {
		t.Errorf("Volume with no flag = %d, want 3 (scale small)", got)
	}
	// The flag must NOT change mock's Volume().
	c.VolumeFlag = 5000
	if got := c.Volume(); got != 3 {
		t.Errorf("Volume with flag = %d, want 3 (mock stays scale-driven; flag is load-only)", got)
	}
}

func TestLoadVolume(t *testing.T) {
	// Explicit volume wins.
	explicit := Config{Scale: ScaleSmall, VolumeFlag: 777}
	if got := explicit.LoadVolume(); got != 777 {
		t.Errorf("LoadVolume with flag = %d, want 777", got)
	}
	// No flag, no duration -> scale fallback.
	fallback := Config{Scale: ScaleLarge}
	if got := fallback.LoadVolume(); got != 50 {
		t.Errorf("LoadVolume no flag/duration = %d, want 50 (scale large fallback)", got)
	}
	// No flag, duration set -> unbounded (0).
	durOnly := Config{Scale: ScaleSmall, Duration: time.Minute}
	if got := durOnly.LoadVolume(); got != 0 {
		t.Errorf("LoadVolume duration-only = %d, want 0 (unbounded)", got)
	}
}

func TestParseLoadKnobs(t *testing.T) {
	t.Setenv("SIM_CONCURRENCY", "")
	t.Setenv("SIM_VOLUME", "")
	t.Setenv("SIM_DURATION", "")
	t.Setenv("SIM_RATE", "")
	c, err := Parse([]string{"--mode", "load", "--concurrency", "32", "--volume", "500", "--duration", "45s", "--rate", "250"})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if c.Concurrency != 32 {
		t.Errorf("Concurrency = %d, want 32", c.Concurrency)
	}
	if c.VolumeFlag != 500 {
		t.Errorf("VolumeFlag = %d, want 500", c.VolumeFlag)
	}
	if c.Duration != 45*time.Second {
		t.Errorf("Duration = %s, want 45s", c.Duration)
	}
	if c.Rate != 250 {
		t.Errorf("Rate = %g, want 250", c.Rate)
	}
}

func TestParseConcurrencyDefault(t *testing.T) {
	t.Setenv("SIM_CONCURRENCY", "")
	c, err := Parse(nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if c.Concurrency != DefaultConcurrency {
		t.Errorf("Concurrency = %d, want default %d", c.Concurrency, DefaultConcurrency)
	}
}

func TestVolumeForScale(t *testing.T) {
	cases := map[Scale]int{ScaleSmall: 3, ScaleMedium: 15, ScaleLarge: 50, Scale("weird"): 3}
	for s, want := range cases {
		if got := volumeForScale(s); got != want {
			t.Errorf("volumeForScale(%q) = %d, want %d", s, got, want)
		}
	}
}
