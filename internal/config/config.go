// Package config reads configuration from environment variables. Secrets are
// only ever read from the environment, never from files in the repository.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds every setting. Zero values are replaced by defaults in Load.
type Config struct {
	DatabaseURL      string        // OPENFANTA_DATABASE_URL
	SportmonksToken  string        // OPENFANTA_SPORTMONKS_TOKEN
	SportmonksURL    string        // OPENFANTA_SPORTMONKS_BASE_URL (default: the public v3 API)
	RequestsPerHour  int           // OPENFANTA_SPORTMONKS_REQUESTS_PER_HOUR (default 1800)
	HTTPAddr         string        // OPENFANTA_HTTP_ADDR (default ":8080")
	LogLevel         slog.Level    // OPENFANTA_LOG_LEVEL: debug, info, warn, error (default info)
	LogFormat        string        // OPENFANTA_LOG_FORMAT: json or text (default json)
	ShutdownTimeout  time.Duration // OPENFANTA_SHUTDOWN_TIMEOUT (default 15s)
	MigrateOnStartup bool          // OPENFANTA_MIGRATE_ON_STARTUP (default false)
}

// Load reads the configuration from getenv (os.Getenv in production).
func Load(getenv func(string) string) (Config, error) {
	c := Config{
		DatabaseURL:     getenv("OPENFANTA_DATABASE_URL"),
		SportmonksToken: getenv("OPENFANTA_SPORTMONKS_TOKEN"),
		SportmonksURL:   getenv("OPENFANTA_SPORTMONKS_BASE_URL"),
		HTTPAddr:        or(getenv("OPENFANTA_HTTP_ADDR"), ":8080"),
		LogFormat:       or(getenv("OPENFANTA_LOG_FORMAT"), "json"),
	}
	var errs []error
	var err error
	if c.RequestsPerHour, err = intVar(getenv, "OPENFANTA_SPORTMONKS_REQUESTS_PER_HOUR", 1800); err != nil {
		errs = append(errs, err)
	} else if c.RequestsPerHour <= 0 {
		errs = append(errs, errors.New("OPENFANTA_SPORTMONKS_REQUESTS_PER_HOUR must be positive"))
	}
	if err := c.LogLevel.UnmarshalText([]byte(or(getenv("OPENFANTA_LOG_LEVEL"), "info"))); err != nil {
		errs = append(errs, fmt.Errorf("OPENFANTA_LOG_LEVEL: %w", err))
	}
	if c.LogFormat != "json" && c.LogFormat != "text" {
		errs = append(errs, fmt.Errorf("OPENFANTA_LOG_FORMAT must be json or text, got %q", c.LogFormat))
	}
	if c.ShutdownTimeout, err = time.ParseDuration(or(getenv("OPENFANTA_SHUTDOWN_TIMEOUT"), "15s")); err != nil {
		errs = append(errs, fmt.Errorf("OPENFANTA_SHUTDOWN_TIMEOUT: %w", err))
	}
	if v := getenv("OPENFANTA_MIGRATE_ON_STARTUP"); v != "" {
		if c.MigrateOnStartup, err = strconv.ParseBool(v); err != nil {
			errs = append(errs, fmt.Errorf("OPENFANTA_MIGRATE_ON_STARTUP: %w", err))
		}
	}
	return c, errors.Join(errs...)
}

// FromEnv loads the configuration from the process environment.
func FromEnv() (Config, error) { return Load(os.Getenv) }

// RequireDatabase returns an error if no database URL is configured.
func (c Config) RequireDatabase() error {
	if c.DatabaseURL == "" {
		return errors.New("OPENFANTA_DATABASE_URL is required")
	}
	return nil
}

// RequireSportmonks returns an error if no Sportmonks token is configured.
func (c Config) RequireSportmonks() error {
	if c.SportmonksToken == "" {
		return errors.New("OPENFANTA_SPORTMONKS_TOKEN is required to fetch from Sportmonks")
	}
	return nil
}

// Logger builds the structured logger.
func (c Config) Logger() *slog.Logger {
	opts := &slog.HandlerOptions{Level: c.LogLevel}
	if c.LogFormat == "text" {
		return slog.New(slog.NewTextHandler(os.Stderr, opts))
	}
	return slog.New(slog.NewJSONHandler(os.Stderr, opts))
}

func or(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

func intVar(getenv func(string) string, name string, def int) (int, error) {
	v := getenv(name)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	return n, nil
}
