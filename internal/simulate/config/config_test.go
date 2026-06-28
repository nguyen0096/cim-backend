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
			name:    "load mode not implemented",
			cfg:     Config{BaseURL: "http://x", Timeout: time.Second, Mode: ModeLoad, Scale: ScaleSmall, DryRun: true},
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

func TestVolumeForScale(t *testing.T) {
	cases := map[Scale]int{ScaleSmall: 3, ScaleMedium: 15, ScaleLarge: 50, Scale("weird"): 3}
	for s, want := range cases {
		if got := volumeForScale(s); got != want {
			t.Errorf("volumeForScale(%q) = %d, want %d", s, got, want)
		}
	}
}
