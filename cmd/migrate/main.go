// Command migrate applies goose database migrations embedded into the binary.
//
// Usage:
//
//	migrate up                 # apply all pending migrations
//	migrate down               # roll back the most recent migration
//	migrate status             # show migration status
//	migrate version            # print current DB version
//	migrate redo               # roll back and re-apply the latest migration
//	migrate create <name> sql  # scaffold a new migration file on disk
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	"github.com/pressly/goose/v3"

	"github.com/aashs25/hsbillnradius/internal/config"
	"github.com/aashs25/hsbillnradius/internal/platform/logger"
	"github.com/aashs25/hsbillnradius/migrations"
)

const migrationsDir = "." // path inside the embedded FS

func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("migrate failed", slog.Any("error", err))
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("missing command: expected one of up, down, status, version, redo, create")
	}
	command, cmdArgs := args[0], args[1:]

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	log := logger.New(logger.Options{
		Level:   cfg.App.LogLevel,
		Format:  cfg.App.LogFormat,
		AppName: cfg.App.Name + "-migrate",
		Env:     cfg.App.Env,
	})

	db, err := sql.Open("pgx", cfg.Postgres.DSN)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer func() { _ = db.Close() }()

	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(gooseLogger{log: log})
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}

	ctx := context.Background()
	if err := goose.RunContext(ctx, command, db, migrationsDir, cmdArgs...); err != nil {
		return fmt.Errorf("goose %s: %w", command, err)
	}
	return nil
}

// gooseLogger adapts slog to goose's logger interface.
type gooseLogger struct{ log *slog.Logger }

func (g gooseLogger) Printf(format string, v ...any) {
	g.log.Info(fmt.Sprintf(format, v...))
}

func (g gooseLogger) Fatalf(format string, v ...any) {
	g.log.Error(fmt.Sprintf(format, v...))
	os.Exit(1)
}
