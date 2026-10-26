package config

import (
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("VYLO_API_TOKEN", "tok123")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() 不應報錯: %v", err)
	}
	if cfg.APIToken != "tok123" {
		t.Errorf("APIToken = %q, want %q", cfg.APIToken, "tok123")
	}
	if cfg.BaseURL != "https://vylo.cc" {
		t.Errorf("BaseURL 預設應為 https://vylo.cc, got %q", cfg.BaseURL)
	}
	if cfg.Steps != 2 {
		t.Errorf("Steps 預設應為 2, got %d", cfg.Steps)
	}
	if cfg.Port != "8080" {
		t.Errorf("Port 預設應為 8080, got %q", cfg.Port)
	}
	if cfg.TargetURL != "http://localhost:8080/thanks" {
		t.Errorf("TargetURL 預設應為 http://localhost:8080/thanks, got %q", cfg.TargetURL)
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("VYLO_API_TOKEN", "abc")
	t.Setenv("VYLO_BASE_URL", "http://mock.local")
	t.Setenv("VYLO_STEPS", "3")
	t.Setenv("PORT", "9000")
	t.Setenv("TARGET_URL", "https://example.com/landing")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() 不應報錯: %v", err)
	}
	if cfg.BaseURL != "http://mock.local" {
		t.Errorf("BaseURL = %q, want http://mock.local", cfg.BaseURL)
	}
	if cfg.Steps != 3 {
		t.Errorf("Steps = %d, want 3", cfg.Steps)
	}
	if cfg.Port != "9000" {
		t.Errorf("Port = %q, want 9000", cfg.Port)
	}
	if cfg.TargetURL != "https://example.com/landing" {
		t.Errorf("TargetURL = %q, want https://example.com/landing", cfg.TargetURL)
	}
}

func TestLoadMissingToken(t *testing.T) {
	if _, err := Load(); err == nil {
		t.Fatal("缺少 VYLO_API_TOKEN 時 Load() 應報錯")
	}
}

func TestLoadInvalidStepsFallsBackToDefault(t *testing.T) {
	t.Setenv("VYLO_API_TOKEN", "abc")

	cases := []string{"abc", "", "0", "4", "-1", "99"}
	for _, c := range cases {
		t.Setenv("VYLO_STEPS", c)
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Steps=%q 時 Load() 不應報錯: %v", c, err)
		}
		if cfg.Steps != 2 {
			t.Errorf("Steps=%q 應回落為 2, got %d", c, cfg.Steps)
		}
	}
}
