package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"pokerlab/internal/session"
	"pokerlab/internal/sim"
)

const (
	defaultHTTPAddr         = ":8080"
	defaultReadTimeout      = 5 * time.Second
	defaultWriteTimeout     = 30 * time.Second
	defaultIdleTimeout      = 60 * time.Second
	defaultStreamHeartbeat  = 15 * time.Second
	defaultSessionCookieAge = 24 * time.Hour
)

// Config centralizes the runtime knobs that should not stay hard-coded in handlers.
type Config struct {
	HTTP       HTTPConfig
	Session    SessionConfig
	Stream     StreamConfig
	Simulation SimulationConfig
}

type HTTPConfig struct {
	Addr         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

type SessionConfig struct {
	CookieName   string
	MaxTables    int
	CookieSecure bool
	CookieMaxAge time.Duration
}

type StreamConfig struct {
	HeartbeatInterval time.Duration
	ReplayLimit       int
	SubscriberBuffer  int
}

type SimulationConfig struct {
	IntraHandDelay   time.Duration
	HandPause        time.Duration
	HistoryLimit     int
	SubscriberBuffer int
}

// Default returns the local-development defaults for the assignment app.
func Default() Config {
	return Config{
		HTTP: HTTPConfig{
			Addr:         defaultHTTPAddr,
			ReadTimeout:  defaultReadTimeout,
			WriteTimeout: defaultWriteTimeout,
			IdleTimeout:  defaultIdleTimeout,
		},
		Session: SessionConfig{
			CookieName:   session.DefaultCookieName,
			MaxTables:    session.DefaultMaxTables,
			CookieMaxAge: defaultSessionCookieAge,
		},
		Stream: StreamConfig{
			HeartbeatInterval: defaultStreamHeartbeat,
			ReplayLimit:       sim.DefaultHistoryLimit,
			SubscriberBuffer:  sim.DefaultSubscriberBuffer,
		},
		Simulation: SimulationConfig{
			IntraHandDelay:   sim.DefaultIntraHandDelay,
			HandPause:        sim.DefaultHandPause,
			HistoryLimit:     sim.DefaultHistoryLimit,
			SubscriberBuffer: sim.DefaultSubscriberBuffer,
		},
	}
}

// LoadFromEnv overlays supported environment variables on top of defaults.
func LoadFromEnv() (Config, error) {
	cfg := Default()

	if value := os.Getenv("HTTP_ADDR"); value != "" {
		cfg.HTTP.Addr = value
	}
	if value := os.Getenv("PORT"); value != "" {
		cfg.HTTP.Addr = ":" + value
	}

	var err error
	if cfg.HTTP.ReadTimeout, err = durationFromEnv("HTTP_READ_TIMEOUT", cfg.HTTP.ReadTimeout); err != nil {
		return Config{}, err
	}
	if cfg.HTTP.WriteTimeout, err = durationFromEnv("HTTP_WRITE_TIMEOUT", cfg.HTTP.WriteTimeout); err != nil {
		return Config{}, err
	}
	if cfg.HTTP.IdleTimeout, err = durationFromEnv("HTTP_IDLE_TIMEOUT", cfg.HTTP.IdleTimeout); err != nil {
		return Config{}, err
	}

	if value := os.Getenv("SESSION_COOKIE_NAME"); value != "" {
		cfg.Session.CookieName = value
	}
	if cfg.Session.MaxTables, err = intFromEnv("SESSION_MAX_TABLES", cfg.Session.MaxTables); err != nil {
		return Config{}, err
	}
	if cfg.Session.CookieSecure, err = boolFromEnv("SESSION_COOKIE_SECURE", cfg.Session.CookieSecure); err != nil {
		return Config{}, err
	}
	if cfg.Session.CookieMaxAge, err = durationFromEnv("SESSION_COOKIE_MAX_AGE", cfg.Session.CookieMaxAge); err != nil {
		return Config{}, err
	}

	if cfg.Stream.HeartbeatInterval, err = durationFromEnv("STREAM_HEARTBEAT_INTERVAL", cfg.Stream.HeartbeatInterval); err != nil {
		return Config{}, err
	}
	if cfg.Stream.ReplayLimit, err = intFromEnv("STREAM_REPLAY_LIMIT", cfg.Stream.ReplayLimit); err != nil {
		return Config{}, err
	}
	if cfg.Stream.SubscriberBuffer, err = intFromEnv("STREAM_SUBSCRIBER_BUFFER", cfg.Stream.SubscriberBuffer); err != nil {
		return Config{}, err
	}

	if cfg.Simulation.IntraHandDelay, err = durationFromEnv("SIM_INTRA_HAND_DELAY", cfg.Simulation.IntraHandDelay); err != nil {
		return Config{}, err
	}
	if cfg.Simulation.HandPause, err = durationFromEnv("SIM_HAND_PAUSE", cfg.Simulation.HandPause); err != nil {
		return Config{}, err
	}
	if cfg.Simulation.HistoryLimit, err = intFromEnv("SIM_HISTORY_LIMIT", cfg.Simulation.HistoryLimit); err != nil {
		return Config{}, err
	}
	if cfg.Simulation.SubscriberBuffer, err = intFromEnv("SIM_SUBSCRIBER_BUFFER", cfg.Simulation.SubscriberBuffer); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func durationFromEnv(name string, fallback time.Duration) (time.Duration, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}

	return parsed, nil
}

func intFromEnv(name string, fallback int) (int, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}

	return parsed, nil
}

func boolFromEnv(name string, fallback bool) (bool, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s: %w", name, err)
	}

	return parsed, nil
}
