// Package config loads service configuration from environment variables.
package config

import "fmt"

// Config holds all runtime configuration for the service.
type Config struct {
	HTTPAddr    string
	PostgresDSN string
	NATSURL     string
	ContentPath string
	LogLevel    string
}

// Load reads configuration from environment variables and returns a validated Config.
// POSTGRES_DSN is required; all other vars have defaults.
func Load() (Config, error) {
	c := Config{
		HTTPAddr:    lookupOrDefault("HTTP_ADDR", ":8080"),
		PostgresDSN: lookupEnv("POSTGRES_DSN"),
		NATSURL:     lookupOrDefault("NATS_URL", "nats://nats.platform.svc.cluster.local:4222"),
		ContentPath: lookupOrDefault("CONTENT_PATH", "/etc/job-hunt-content"),
		LogLevel:    lookupOrDefault("LOG_LEVEL", "info"),
	}

	if c.PostgresDSN == "" {
		return Config{}, fmt.Errorf("POSTGRES_DSN is required")
	}

	return c, nil
}
