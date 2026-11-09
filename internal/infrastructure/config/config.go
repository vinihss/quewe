package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr string
	LogLevel string

	AppEnv        string
	DebugAuthStub bool

	DatabaseURL string
	DBEnabled   bool

	LeaseDuration     time.Duration
	HeartbeatInterval time.Duration
	ReaperInterval    time.Duration
	PollWaitSeconds   int
	MaxReserve        int
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:          envOrDefault("HTTP_ADDR", ":8080"),
		LogLevel:          strings.ToLower(envOrDefault("LOG_LEVEL", "info")),
		AppEnv:            strings.ToLower(envOrDefault("APP_ENV", "")),
		DebugAuthStub:     envBool("DEBUG_AUTH_STUB"),
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		DBEnabled:         envBool("DB_ENABLED"),
		LeaseDuration:     60 * time.Second,
		HeartbeatInterval: 20 * time.Second,
		ReaperInterval:    15 * time.Second,
		PollWaitSeconds:   20,
		MaxReserve:        1,
	}
	// If DATABASE_URL provided, enable DB.
	if cfg.DatabaseURL != "" {
		cfg.DBEnabled = true
	}
	if v := os.Getenv("LEASE_DURATION"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.LeaseDuration = d
		}
	}
	if v := os.Getenv("HEARTBEAT_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.HeartbeatInterval = d
		}
	}
	if v := os.Getenv("REAPER_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.ReaperInterval = d
		}
	}
	if v := os.Getenv("POLL_WAIT_SECONDS"); v != "" {
		var n int
		fmt.Sscanf(v, "%d", &n)
		if n > 0 {
			cfg.PollWaitSeconds = n
		}
	}
	if v := os.Getenv("MAX_RESERVE"); v != "" {
		var n int
		fmt.Sscanf(v, "%d", &n)
		if n > 0 {
			cfg.MaxReserve = n
		}
	}
	if err := cfg.Validateste(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c *Config) applyDefaults() {
	if c.LeaseDuration == 0 {
		c.LeaseDuration = 60 * time.Second
	}
	if c.HeartbeatInterval == 0 {
		c.HeartbeatInterval = 20 * time.Second
	}
	if c.ReaperInterval == 0 {
		c.ReaperInterval = 15 * time.Second
	}
	if c.PollWaitSeconds == 0 {
		c.PollWaitSeconds = 20
	}
	if c.MaxReserve == 0 {
		c.MaxReserve = 1
	}
}

func (c Config) Validateste() error {
	c.applyDefaults()
	if c.HTTPAddr == "" {
		return fmt.Errorf("config: HTTP_ADDR is required")
	}
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("config: invalid LOG_LEVEL %q", c.LogLevel)
	}
	if c.DebugAuthStub && c.AppEnv != "dev" {
		return fmt.Errorf("config: DEBUG_AUTH_STUB requires APP_ENV=dev")
	}
	if c.DBEnabled && c.DatabaseURL == "" {
		return fmt.Errorf("config: DATABASE_URL required when DB enabled")
	}
	if c.LeaseDuration <= 0 || c.HeartbeatInterval <= 0 || c.ReaperInterval <= 0 {
		return fmt.Errorf("config: durations must be positive")
	}
	if c.PollWaitSeconds <= 0 || c.PollWaitSeconds > int(c.LeaseDuration.Seconds()) {
		return fmt.Errorf("config: POLL_WAIT_SECONDS must be <= lease duration")
	}
	if c.MaxReserve <= 0 {
		return fmt.Errorf("config: MAX_RESERVE must be > 0")
	}
	return nil
}

func (c Config) AllowDebugAuthStub() bool {
	return c.DebugAuthStub && c.AppEnv == "dev"
}

func envOrDefault(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

func envBool(key string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	return v == "1" || v == "true" || v == "yes"
}
