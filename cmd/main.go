// Command expenses runs the personal expense analyzer: a goplatform
// application with Postgres storage, a JSON API and the embedded React
// frontend, all on one HTTP port.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"
	// Receipt timestamps are parsed in Europe/Moscow; embedding the zone
	// database keeps that working on a Windows host without one.
	_ "time/tzdata"

	"github.com/ALexfonSchneider/goplatform/pkg/config"
	"github.com/ALexfonSchneider/goplatform/pkg/observe"
	"github.com/ALexfonSchneider/goplatform/pkg/platform"
	"github.com/ALexfonSchneider/goplatform/pkg/postgres"
	"github.com/ALexfonSchneider/goplatform/pkg/server"

	"github.com/ALexfonSchneider/expenses/internal/adapters/handlers/api"
	"github.com/ALexfonSchneider/expenses/internal/adapters/handlers/spa"
	"github.com/ALexfonSchneider/expenses/internal/adapters/lkdr"
	"github.com/ALexfonSchneider/expenses/internal/adapters/postgresrepo"
	"github.com/ALexfonSchneider/expenses/internal/adapters/yandexpdf"
	"github.com/ALexfonSchneider/expenses/internal/app"
	"github.com/ALexfonSchneider/expenses/migrations"
	"github.com/ALexfonSchneider/expenses/web"
)

// Config holds the application configuration.
type Config struct {
	Server struct {
		Addr string `koanf:"addr"`
	} `koanf:"server"`
	Observe struct {
		ServiceName  string `koanf:"service_name"`
		OTLPEndpoint string `koanf:"otlp_endpoint"`
	} `koanf:"observe"`
	Postgres struct {
		DSN string `koanf:"dsn"`
	} `koanf:"postgres"`
	Upload struct {
		MaxSizeMB int64 `koanf:"max_size_mb"`
	} `koanf:"upload"`
	Receipts struct {
		// SyncInterval is a Go duration ("6h", "30m"); empty or "0" disables
		// the scheduler and leaves only the manual button.
		SyncInterval string `koanf:"sync_interval"`
	} `koanf:"receipts"`
}

func main() {
	if err := run(); err != nil {
		slog.Error("expenses: fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	loader, err := config.NewLoader(
		config.WithFiles("config/config.yaml", "config/config.postgres.yaml"),
		config.WithEnvPrefix("EXPENSES"),
	)
	if err != nil {
		return fmt.Errorf("config loader: %w", err)
	}
	var cfg Config
	if err := loader.Load(&cfg); err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if cfg.Upload.MaxSizeMB <= 0 {
		cfg.Upload.MaxSizeMB = 20
	}
	syncInterval := 6 * time.Hour
	if raw := cfg.Receipts.SyncInterval; raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return fmt.Errorf("receipts.sync_interval: %w", err)
		}
		syncInterval = d
	}

	obs, err := observe.New(
		observe.WithServiceName(cfg.Observe.ServiceName),
		observe.WithOTLPEndpoint(cfg.Observe.OTLPEndpoint),
		observe.WithBaseHandler(slog.NewJSONHandler(os.Stdout, nil)),
	)
	if err != nil {
		return fmt.Errorf("observe: %w", err)
	}
	logger := obs.Logger()

	application := platform.New(platform.WithLogger(logger))
	if err := application.Register("observe", obs); err != nil {
		return err
	}

	db, err := postgres.New(
		postgres.WithDSN(cfg.Postgres.DSN),
		postgres.WithTracerProvider(obs.TracerProvider()),
		postgres.WithLogger(logger),
	)
	if err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	if err := application.Register("postgres", db); err != nil {
		return err
	}

	// Schema migrations run once Postgres is up and before the HTTP server
	// accepts requests. BeforeStart fires per component in registration
	// order, so hooking "server" guarantees "postgres" has already started.
	application.OnBeforeStart(func(ctx context.Context, name string) error {
		if name != "server" {
			return nil
		}
		if err := db.Migrate(ctx, migrations.FS); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
		logger.InfoContext(ctx, "migrations applied")
		return nil
	})

	parser, err := yandexpdf.New()
	if err != nil {
		return fmt.Errorf("parser: %w", err)
	}
	repo := postgresrepo.New(db)
	receiptSource, err := lkdr.New(repo.Settings)
	if err != nil {
		return fmt.Errorf("lkdr: %w", err)
	}
	svc, err := app.NewService(app.Deps{
		Logger:              logger,
		Parser:              parser,
		Statements:          repo.Statements,
		Transactions:        repo.Transactions,
		Categories:          repo.Categories,
		Analytics:           repo.Analytics,
		Budgets:             repo.Budgets,
		ReceiptSource:       receiptSource,
		ReceiptSessions:     repo.Settings,
		ReceiptSync:         repo.Settings,
		Receipts:            repo.Receipts,
		ReceiptSyncInterval: syncInterval,
		Now:                 time.Now,
	})
	if err != nil {
		return fmt.Errorf("service: %w", err)
	}
	// The service runs the receipt synchronization in the background;
	// registering it between postgres and the server stops that job before
	// the pool closes and after the HTTP server stops taking requests.
	if err := application.Register("app", svc); err != nil {
		return err
	}

	// Checking the receipt archive session waits on an external service
	// that can take tens of seconds, longer than the default write timeout.
	srv, err := server.New(
		server.WithAddr(cfg.Server.Addr),
		server.WithLogger(logger),
		server.WithHealthCheckers(application.HealthCheckers()),
		server.WithReadTimeout(30*time.Second),
		server.WithWriteTimeout(2*time.Minute),
	)
	if err != nil {
		return fmt.Errorf("server: %w", err)
	}
	srv.Use(observe.HTTPMiddleware(obs.TracerProvider(), obs.MeterProvider()))

	apiHandler, err := api.New(svc, logger, api.WithMaxUpload(cfg.Upload.MaxSizeMB<<20))
	if err != nil {
		return fmt.Errorf("api: %w", err)
	}
	srv.Mount("/api/v1", apiHandler.Routes())

	ui, err := spa.New(web.Dist)
	if err != nil {
		return fmt.Errorf("spa: %w", err)
	}
	srv.Mount("/", ui)

	if err := application.Register("server", srv); err != nil {
		return err
	}

	logger.Info("starting expenses", "addr", cfg.Server.Addr)
	return application.Run(context.Background())
}
