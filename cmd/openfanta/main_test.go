package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestUsage(t *testing.T) {
	t.Setenv("OPENFANTA_DATABASE_URL", "")
	var out bytes.Buffer
	if err := run(context.Background(), nil, &out); !errors.Is(err, errUsage) || !strings.Contains(out.String(), "usage:") {
		t.Fatalf("no args: %v, %q", err, out.String())
	}
	out.Reset()
	if err := run(context.Background(), []string{"bogus"}, &out); !errors.Is(err, errUsage) {
		t.Fatalf("unknown command: %v", err)
	}
	if err := run(context.Background(), []string{"help"}, &out); err != nil {
		t.Fatalf("help: %v", err)
	}
}

func TestRulesetsCommand(t *testing.T) {
	var out bytes.Buffer
	if err := run(context.Background(), []string{"rulesets"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "classic@1.0.0") {
		t.Errorf("output = %q", out.String())
	}
}

func TestExtraRulesetsDir(t *testing.T) {
	t.Setenv("OPENFANTA_RULESETS_DIR", t.TempDir())
	var out bytes.Buffer
	if err := run(context.Background(), []string{"rulesets"}, &out); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENFANTA_RULESETS_DIR", "/does/not/exist")
	if err := run(context.Background(), []string{"rulesets"}, &out); err == nil {
		t.Fatal("missing rule set dir should fail")
	}
}

func TestCommandsRequireConfiguration(t *testing.T) {
	t.Setenv("OPENFANTA_DATABASE_URL", "")
	t.Setenv("OPENFANTA_SPORTMONKS_TOKEN", "")
	var out bytes.Buffer
	cases := map[string][]string{
		"migrate":           {"migrate"},
		"ingest flags":      {"ingest"},
		"ingest no db":      {"ingest", "--season", "1", "--matchday", "1"},
		"recompute no db":   {"recompute"},
		"recompute bad id":  {"recompute", "--season-id", "nope"},
		"bad log level env": nil,
	}
	for name, args := range cases {
		if args == nil {
			continue
		}
		if err := run(context.Background(), args, &out); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	t.Setenv("OPENFANTA_DATABASE_URL", "postgres://localhost/x")
	if err := run(context.Background(), []string{"ingest", "--season", "1", "--matchday", "1"}, &out); err == nil ||
		!strings.Contains(err.Error(), "SPORTMONKS_TOKEN") {
		t.Errorf("ingest without token: %v", err)
	}
	t.Setenv("OPENFANTA_LOG_LEVEL", "loud")
	if err := run(context.Background(), []string{"rulesets"}, &out); err == nil {
		t.Error("invalid config should fail")
	}
}
