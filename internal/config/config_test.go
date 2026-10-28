package config

import (
	"log/slog"
	"strings"
	"testing"
	"time"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestDefaults(t *testing.T) {
	c, err := Load(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if c.HTTPAddr != ":8080" || c.RequestsPerHour != 1800 || c.LogLevel != slog.LevelInfo ||
		c.LogFormat != "json" || c.ShutdownTimeout != 15*time.Second || c.MigrateOnStartup {
		t.Errorf("defaults = %+v", c)
	}
	if c.RequireDatabase() == nil || c.RequireSportmonks() == nil {
		t.Error("missing database URL and token must be reported")
	}
	if c.Logger() == nil {
		t.Error("nil logger")
	}
}

func TestOverrides(t *testing.T) {
	c, err := Load(env(map[string]string{
		"OPENFANTA_DATABASE_URL": "postgres://x", "OPENFANTA_SPORTMONKS_TOKEN": "t",
		"OPENFANTA_HTTP_ADDR": ":9000", "OPENFANTA_LOG_LEVEL": "debug", "OPENFANTA_LOG_FORMAT": "text",
		"OPENFANTA_SPORTMONKS_REQUESTS_PER_HOUR": "100", "OPENFANTA_SHUTDOWN_TIMEOUT": "3s",
		"OPENFANTA_MIGRATE_ON_STARTUP": "true",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c.HTTPAddr != ":9000" || c.LogLevel != slog.LevelDebug || c.RequestsPerHour != 100 ||
		c.ShutdownTimeout != 3*time.Second || !c.MigrateOnStartup {
		t.Errorf("config = %+v", c)
	}
	if c.RequireDatabase() != nil || c.RequireSportmonks() != nil {
		t.Error("requirements should be met")
	}
	if c.Logger() == nil {
		t.Error("nil text logger")
	}
}

func TestInvalidValuesAreAllReported(t *testing.T) {
	_, err := Load(env(map[string]string{
		"OPENFANTA_LOG_LEVEL": "loud", "OPENFANTA_LOG_FORMAT": "xml",
		"OPENFANTA_SPORTMONKS_REQUESTS_PER_HOUR": "lots", "OPENFANTA_SHUTDOWN_TIMEOUT": "soon",
		"OPENFANTA_MIGRATE_ON_STARTUP": "maybe",
	}))
	if err == nil {
		t.Fatal("expected errors")
	}
	for _, name := range []string{"LOG_LEVEL", "LOG_FORMAT", "REQUESTS_PER_HOUR", "SHUTDOWN_TIMEOUT", "MIGRATE_ON_STARTUP"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error does not mention %s: %v", name, err)
		}
	}
	if _, err := Load(env(map[string]string{"OPENFANTA_SPORTMONKS_REQUESTS_PER_HOUR": "0"})); err == nil {
		t.Error("zero requests per hour accepted")
	}
}
