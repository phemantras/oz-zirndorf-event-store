// Command eventstore starts the OZ Zirndorf Event Store: it reads the
// configuration from the environment, applies the database migrations,
// recomputes the name keys of all locations and the derived values of all
// events, marks past events as archived and only then starts the HTTP
// server. The marking repeats daily.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // Europe/Berlin must resolve even without system zone data.

	"github.com/phemantras/oz-zirndorf-event-store/internal/adapter/admin"
	"github.com/phemantras/oz-zirndorf-event-store/internal/adapter/cleanup"
	"github.com/phemantras/oz-zirndorf-event-store/internal/adapter/postgres"
	publicapi "github.com/phemantras/oz-zirndorf-event-store/internal/adapter/publicapi/v1"
	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

const (
	exitCodeFailure = 1
	// shutdownTimeout bounds how long running requests may finish after
	// SIGINT or SIGTERM. It lies above requestTimeout, so a request that
	// waits until its deadline still ends with an answer before the
	// shutdown gives up.
	shutdownTimeout = 25 * time.Second
	// readHeaderTimeout protects the server against slow-header clients.
	readHeaderTimeout = 5 * time.Second
	// readTimeout bounds reading a whole request; the largest, an admin
	// form of at most 64 KiB, needs far less.
	readTimeout = 15 * time.Second
	// writeTimeout bounds handling a request and writing its response.
	writeTimeout = 30 * time.Second
	// requestTimeout bounds handling a request, its queries and its wait
	// for a pooled connection. It lies below writeTimeout, so the answer to
	// an expired request can still be written.
	requestTimeout = 20 * time.Second
	// idleTimeout closes keep-alive connections that wait for a next request.
	idleTimeout = 120 * time.Second
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	logger := newLogger(os.Stdout)

	err := run(ctx, logger, os.Getenv)
	stop()
	if err != nil {
		logger.Error("eventstore stopped with error", "error", err)
		os.Exit(exitCodeFailure)
	}
}

// newLogger returns a logger that writes JSON lines to out.
func newLogger(out io.Writer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(out, nil))
}

// run wires the service in startup order (configuration, database,
// migrations, recomputation of name keys and derived values, cleanup, HTTP
// server) and blocks until ctx is cancelled or a step fails. The cleanup
// runs once before the server listens and then daily; its failures are only
// logged.
func run(ctx context.Context, logger *slog.Logger, getenv func(string) string) error {
	cfg, err := loadConfig(getenv)
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	pool, err := postgres.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()

	applied, err := postgres.Migrate(ctx, pool)
	if err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}
	logger.Info("database migrations applied", "count", applied)

	locationRepo := postgres.NewLocationRepo(pool)
	tx := postgres.NewTxRunner(pool)
	locations := core.NewLocationService(tx, locationRepo)
	eventRepo := postgres.NewEventRepo(pool)
	events := core.NewEventService(tx, eventRepo, locationRepo)
	if err := recomputeDerived(ctx, locations, events, logger); err != nil {
		return err
	}
	jobCtx, stopJob := context.WithCancel(ctx)
	defer stopJob()
	startCleanup(jobCtx, cleanup.Job{Archiver: events, Clock: systemClock{}, Logger: logger}, cleanup.DailyInterval)

	listener, err := net.Listen("tcp", cfg.ListenAddress())
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.ListenAddress(), err)
	}
	// The admin shares both services with the recomputation, so it sees
	// their review marks.
	cases := useCases{locations: locations, events: events, imports: core.NewImportService(events)}
	return serve(ctx, newServer(pool, logger, newRouteHandlers(cfg, logger, cases)), listener, logger)
}

// startCleanup runs job once and then every interval in the background
// until ctx ends. A failing run is only logged and never stops the start.
func startCleanup(ctx context.Context, job cleanup.Job, interval time.Duration) {
	job.RunOnce(ctx)
	go job.RunDaily(ctx, interval)
}

// useCases are the core use cases the admin interface and the public API
// work with.
type useCases struct {
	locations admin.LocationUseCases
	events    eventUseCases
	imports   admin.ImportUseCases
}

// eventUseCases are the event use cases of the admin and the event query of
// the public API, both served by core.EventService.
type eventUseCases interface {
	admin.EventUseCases
	publicapi.EventLister
}

// newRouteHandlers builds the admin interface and the public API.
func newRouteHandlers(cfg config, logger *slog.Logger, cases useCases) routeHandlers {
	return routeHandlers{
		admin:  newAdminHandler(cfg, logger, cases),
		public: publicapi.NewHandler(publicapi.Config{Logger: logger, Events: cases.events, Clock: systemClock{}}),
	}
}

// newAdminHandler builds the admin interface from the validated
// configuration and the core use cases, with the wall clock as time source.
func newAdminHandler(cfg config, logger *slog.Logger, cases useCases) http.Handler {
	return admin.NewHandler(admin.Config{
		User:          cfg.AdminUser,
		PasswordHash:  cfg.AdminPasswordHash,
		SessionSecret: cfg.SessionSecret,
		Logger:        logger,
		Now:           time.Now,
		Locations:     cases.locations,
		Events:        cases.events,
		Clock:         systemClock{},
		Imports:       cases.imports,
	})
}

// newServer returns the HTTP server with all routes and timeouts; every
// request gets a deadline of requestTimeout.
func newServer(db pinger, logger *slog.Logger, handlers routeHandlers) *http.Server {
	return &http.Server{
		Handler:           withRequestDeadline(newRouter(db, logger, handlers), requestTimeout),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}
}

// serve answers requests on listener until ctx is cancelled, then shuts the
// server down gracefully within shutdownTimeout.
func serve(ctx context.Context, server *http.Server, listener net.Listener, logger *slog.Logger) error {
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	logger.Info("http server listening", "address", listener.Addr().String())

	select {
	case err := <-serveErr:
		return fmt.Errorf("serve http: %w", err)
	case <-ctx.Done():
	}

	logger.Info("shutting down http server")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shut down http server: %w", err)
	}
	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve http: %w", err)
	}
	return nil
}
