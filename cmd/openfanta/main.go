// Command openfanta runs the OpenFanta backend: ingestion, recomputation,
// migrations and the read-only HTTP API.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"golang.org/x/time/rate"

	"github.com/openfanta/openfanta/internal/api"
	"github.com/openfanta/openfanta/internal/config"
	"github.com/openfanta/openfanta/internal/ingest"
	"github.com/openfanta/openfanta/internal/provider/httpx"
	"github.com/openfanta/openfanta/internal/provider/sportmonks"
	"github.com/openfanta/openfanta/internal/rules"
	"github.com/openfanta/openfanta/internal/store"
	"github.com/openfanta/openfanta/internal/voto"
)

const usage = `usage: openfanta <command> [flags]

commands:
  serve       serve the read-only HTTP API
  migrate     apply database migrations
  ingest      fetch one matchday from the provider and compute fantavoti
  recompute   recompute stored fixtures under the current rule sets
  rulesets    list the loaded rule sets

Configuration comes from OPENFANTA_* environment variables; see .env.example.
`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, os.Args[1:], os.Stdout)
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "openfanta:", err)
		os.Exit(1)
	}
}

var errUsage = errors.New("invalid usage")

func run(ctx context.Context, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		fmt.Fprint(stdout, usage)
		return errUsage
	}
	cfg, err := config.FromEnv()
	if err != nil {
		return err
	}
	log := cfg.Logger()
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "serve":
		return runServe(ctx, cfg, log)
	case "migrate":
		if err := cfg.RequireDatabase(); err != nil {
			return err
		}
		if err := store.Migrate(ctx, cfg.DatabaseURL); err != nil {
			return err
		}
		log.InfoContext(ctx, "migrations applied")
		return nil
	case "ingest":
		return runIngest(ctx, cfg, log, rest, stdout)
	case "recompute":
		return runRecompute(ctx, cfg, log, rest, stdout)
	case "rulesets":
		reg, err := loadRules(cfg)
		if err != nil {
			return err
		}
		for _, rs := range reg.All() {
			fmt.Fprintf(stdout, "%-24s %s  %s\n", rs.Key(), rs.Hash()[:12], rs.Name)
		}
		return nil
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
		return nil
	default:
		fmt.Fprint(stdout, usage)
		return fmt.Errorf("%w: unknown command %q", errUsage, cmd)
	}
}

func loadRules(cfg config.Config) (*rules.Registry, error) {
	reg, err := rules.LoadBuiltin()
	if err != nil {
		return nil, err
	}
	if cfg.RulesetsDir == "" {
		return reg, nil
	}
	extra, err := rules.LoadFS(os.DirFS(cfg.RulesetsDir))
	if err != nil {
		return nil, fmt.Errorf("rule sets in %s: %w", cfg.RulesetsDir, err)
	}
	return rules.Merge(reg, extra)
}

// newService wires the pipeline. The Sportmonks client is only needed for
// commands that may hit the network.
func newService(ctx context.Context, cfg config.Config, log *slog.Logger, needProvider bool) (*ingest.Service, func(), error) {
	if err := cfg.RequireDatabase(); err != nil {
		return nil, nil, err
	}
	if needProvider {
		if err := cfg.RequireSportmonks(); err != nil {
			return nil, nil, err
		}
	}
	reg, err := loadRules(cfg)
	if err != nil {
		return nil, nil, err
	}
	if cfg.MigrateOnStartup {
		if err := store.Migrate(ctx, cfg.DatabaseURL); err != nil {
			return nil, nil, err
		}
	}
	st, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, nil, err
	}
	if err := st.SyncRuleSets(ctx, reg.All()); err != nil {
		st.Close()
		return nil, nil, err
	}
	limiter := rate.NewLimiter(rate.Every(time.Hour/time.Duration(cfg.RequestsPerHour)), 5)
	sm := sportmonks.New(cfg.SportmonksToken, httpx.New(limiter, log))
	if cfg.SportmonksURL != "" {
		sm.BaseURL = cfg.SportmonksURL
	}
	svc := &ingest.Service{
		Store: st, Provider: sm, Rules: reg, Voto: voto.NewProviderRating(),
		Logger: log, Now: time.Now,
	}
	return svc, st.Close, nil
}

func runIngest(ctx context.Context, cfg config.Config, log *slog.Logger, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("ingest", flag.ContinueOnError)
	fs.SetOutput(stdout)
	season := fs.String("season", "", "provider season ID (required), e.g. the Sportmonks season ID of Serie A 2026/2027")
	matchday := fs.Int("matchday", 0, "matchday number (required)")
	refresh := fs.Bool("refresh", false, "re-fetch everything, including finished fixtures, to pick up upstream corrections")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *season == "" || *matchday <= 0 {
		fs.Usage()
		return fmt.Errorf("%w: --season and --matchday are required", errUsage)
	}
	svc, closeFn, err := newService(ctx, cfg, log, true)
	if err != nil {
		return err
	}
	defer closeFn()
	rep, err := svc.IngestMatchday(ctx, *season, *matchday, ingest.Options{Refresh: *refresh})
	if err != nil {
		return err
	}
	return printReport(stdout, rep)
}

func runRecompute(ctx context.Context, cfg config.Config, log *slog.Logger, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("recompute", flag.ContinueOnError)
	fs.SetOutput(stdout)
	season := fs.String("season-id", "", "limit to one OpenFanta season ID (UUID)")
	reparse := fs.Bool("reparse", false, "re-parse stored raw payloads with the current parser first (no network)")
	reason := fs.String("reason", "recompute", "reason recorded on new runs")
	if err := fs.Parse(args); err != nil {
		return err
	}
	opts := ingest.RecomputeOptions{Reparse: *reparse, Reason: *reason}
	if *season != "" {
		id, err := uuid.Parse(*season)
		if err != nil {
			return fmt.Errorf("--season-id: %w", err)
		}
		opts.SeasonID = &id
	}
	svc, closeFn, err := newService(ctx, cfg, log, false)
	if err != nil {
		return err
	}
	defer closeFn()
	rep, err := svc.Recompute(ctx, opts)
	if err != nil {
		return err
	}
	return printReport(stdout, rep)
}

func printReport(w io.Writer, rep ingest.Report) error {
	created := 0
	for _, f := range rep.Fixtures {
		status := f.Status
		if status == "" {
			status = "-"
		}
		line := fmt.Sprintf("%-10s %s  status=%-8s new_runs=%d cached=%v", f.Ref, f.ID, status, f.RunsCreated, f.FromCache)
		if f.Err != nil {
			line += "  ERROR: " + f.Err.Error()
		}
		fmt.Fprintln(w, line)
		created += f.RunsCreated
	}
	fmt.Fprintf(w, "%d fixtures, %d new runs, %d failed\n", len(rep.Fixtures), created, rep.Failed())
	if rep.Failed() > 0 {
		return fmt.Errorf("%d fixtures failed", rep.Failed())
	}
	return nil
}

func runServe(ctx context.Context, cfg config.Config, log *slog.Logger) error {
	if err := cfg.RequireDatabase(); err != nil {
		return err
	}
	if cfg.MigrateOnStartup {
		if err := store.Migrate(ctx, cfg.DatabaseURL); err != nil {
			return err
		}
	}
	reg, err := loadRules(cfg)
	if err != nil {
		return err
	}
	st, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer st.Close()
	if err := st.SyncRuleSets(ctx, reg.All()); err != nil {
		return err
	}
	ln, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return err
	}
	return serve(ctx, ln, api.NewHandler(api.NewServer(st.Queries(), st.Ping), log), cfg.ShutdownTimeout, log)
}

// serve runs srv on ln until ctx is cancelled, then drains in-flight requests
// for up to grace before returning.
func serve(ctx context.Context, ln net.Listener, h http.Handler, grace time.Duration, log *slog.Logger) error {
	srv := &http.Server{
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return context.WithoutCancel(ctx) },
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	log.InfoContext(ctx, "listening", "addr", ln.Addr().String())

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	log.Info("shutting down", "grace", grace)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), grace)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	if err := <-errc; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	log.Info("stopped")
	return nil
}
