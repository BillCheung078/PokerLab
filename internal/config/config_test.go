package config

import (
	"testing"
	"time"
)

func TestDefaultProvidesExpectedRuntimeTuning(t *testing.T) {
	cfg := Default()

	if cfg.HTTP.Addr != ":8080" {
		t.Fatalf("HTTP.Addr = %q, want %q", cfg.HTTP.Addr, ":8080")
	}
	if cfg.Session.MaxTables != 8 {
		t.Fatalf("Session.MaxTables = %d, want %d", cfg.Session.MaxTables, 8)
	}
	if cfg.Stream.ReplayLimit != 64 {
		t.Fatalf("Stream.ReplayLimit = %d, want %d", cfg.Stream.ReplayLimit, 64)
	}
	if cfg.Simulation.IntraHandDelay != 550*time.Millisecond {
		t.Fatalf("Simulation.IntraHandDelay = %s, want %s", cfg.Simulation.IntraHandDelay, 550*time.Millisecond)
	}
}

func TestLoadFromEnvOverridesSelectedFields(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("SESSION_MAX_TABLES", "12")
	t.Setenv("STREAM_HEARTBEAT_INTERVAL", "20s")
	t.Setenv("SIM_HISTORY_LIMIT", "128")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}

	if cfg.HTTP.Addr != ":9090" {
		t.Fatalf("HTTP.Addr = %q, want %q", cfg.HTTP.Addr, ":9090")
	}
	if cfg.Session.MaxTables != 12 {
		t.Fatalf("Session.MaxTables = %d, want %d", cfg.Session.MaxTables, 12)
	}
	if cfg.Stream.HeartbeatInterval != 20*time.Second {
		t.Fatalf("Stream.HeartbeatInterval = %s, want %s", cfg.Stream.HeartbeatInterval, 20*time.Second)
	}
	if cfg.Simulation.HistoryLimit != 128 {
		t.Fatalf("Simulation.HistoryLimit = %d, want %d", cfg.Simulation.HistoryLimit, 128)
	}
}
