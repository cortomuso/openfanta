package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
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

func TestSettingsCommand(t *testing.T) {
	var out bytes.Buffer
	if err := run(context.Background(), []string{"settings"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.String(), `{"engine":`) || !strings.Contains(out.String(), `"game":"classic"`) {
		t.Errorf("output = %q", out.String())
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
	if err := run(context.Background(), []string{"settings"}, &out); err == nil {
		t.Error("invalid config should fail")
	}
}

func TestServeDrainsInFlightRequestsOnShutdown(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write([]byte("done"))
	})
	ctx, cancel := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() { served <- serve(ctx, ln, h, 5*time.Second, slog.New(slog.NewTextHandler(io.Discard, nil))) }()

	got := make(chan string, 1)
	go func() {
		resp, err := http.Get("http://" + ln.Addr().String() + "/")
		if err != nil {
			got <- "error: " + err.Error()
			return
		}
		defer func() { _ = resp.Body.Close() }()
		b, _ := io.ReadAll(resp.Body)
		got <- string(b)
	}()
	<-started
	cancel() // like SIGTERM while a request is in flight

	if body := <-got; body != "done" {
		t.Errorf("in-flight request got %q, want it to complete", body)
	}
	if err := <-served; err != nil {
		t.Errorf("serve returned %v", err)
	}
	if _, err := http.Get("http://" + ln.Addr().String() + "/"); err == nil {
		t.Error("server still accepting connections after shutdown")
	}
}

func TestServeShutdownTimeout(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	block := make(chan struct{})
	defer close(block)
	started := make(chan struct{})
	h := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { close(started); <-block })
	ctx, cancel := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() {
		served <- serve(ctx, ln, h, 50*time.Millisecond, slog.New(slog.NewTextHandler(io.Discard, nil)))
	}()
	go func() { _, _ = http.Get("http://" + ln.Addr().String() + "/") }()
	<-started
	cancel()
	if err := <-served; err == nil || !strings.Contains(err.Error(), "graceful shutdown") {
		t.Errorf("serve = %v, want a graceful shutdown timeout", err)
	}
}

func TestServeRequiresDatabase(t *testing.T) {
	t.Setenv("OPENFANTA_DATABASE_URL", "")
	if err := run(context.Background(), []string{"serve"}, io.Discard); err == nil {
		t.Error("serve without a database should fail")
	}
}
