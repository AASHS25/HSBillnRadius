// Package logger builds the structured slog logger used across all binaries.
// Every log line carries app and env attributes; request and job handlers add
// trace_id/tenant_id on top via slog.With.
package logger

import (
	"log/slog"
	"os"
)

// Options configures the logger without coupling this package to config.
type Options struct {
	// Level is one of debug, info, warn, error.
	Level string
	// Format is json or text. Unknown values fall back to json.
	Format string
	// AppName and Env are attached to every record.
	AppName string
	Env     string
}

// New returns a slog.Logger writing to stdout with the given options.
func New(opts Options) *slog.Logger {
	handlerOpts := &slog.HandlerOptions{Level: parseLevel(opts.Level)}

	var handler slog.Handler
	switch opts.Format {
	case "text":
		handler = slog.NewTextHandler(os.Stdout, handlerOpts)
	default:
		handler = slog.NewJSONHandler(os.Stdout, handlerOpts)
	}

	return slog.New(handler).With(
		slog.String("app", opts.AppName),
		slog.String("env", opts.Env),
	)
}

// parseLevel maps a level string to slog.Level, defaulting to Info.
func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
