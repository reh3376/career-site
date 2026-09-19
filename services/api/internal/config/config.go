// Package config reads runtime configuration from the environment.
// One config struct, one loader; values are read once at start.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Addr            string
	Env             string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
	SidecarAddr     string
	SidecarTimeout  time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		Addr:            envOr("API_ADDR", ":8080"),
		Env:             envOr("API_ENV", "development"),
		ReadTimeout:     15 * time.Second,
		WriteTimeout:    30 * time.Second,
		ShutdownTimeout: 10 * time.Second,
		SidecarAddr:     envOr("SIDECAR_ADDR", "localhost:50051"),
		SidecarTimeout:  2 * time.Second,
	}
	if v := os.Getenv("API_READ_TIMEOUT_SECONDS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("API_READ_TIMEOUT_SECONDS: %w", err)
		}
		cfg.ReadTimeout = time.Duration(n) * time.Second
	}
	if v := os.Getenv("API_WRITE_TIMEOUT_SECONDS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("API_WRITE_TIMEOUT_SECONDS: %w", err)
		}
		cfg.WriteTimeout = time.Duration(n) * time.Second
	}
	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
