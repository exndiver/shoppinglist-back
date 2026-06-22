package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/exndiver/shopping-backend/internal/config"
)

var logFile *os.File

// Init configures the process-wide slog default handler (JSON or text → файл из cfg.LogFile).
func Init(cfg config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level:     parseLevel(cfg.LogLevel),
		AddSource: cfg.LogLevel == "debug",
	}

	path := strings.TrimSpace(cfg.LogFile)
	if path == "" {
		path = "logs/logs.log"
	}

	var out io.Writer
	switch strings.ToLower(strings.TrimSpace(cfg.LogOutput)) {
	case "stdout":
		out = os.Stdout
	case "both":
		f, err := openLogFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "logging: cannot open log file %q: %v (using stdout only)\n", path, err)
			out = os.Stdout
		} else {
			logFile = f
			out = io.MultiWriter(os.Stdout, f)
		}
	default: // "file"
		f, err := openLogFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "logging: cannot open log file %q: %v (using stderr)\n", path, err)
			out = os.Stderr
		} else {
			logFile = f
			out = f
		}
	}

	var h slog.Handler
	switch strings.ToLower(strings.TrimSpace(cfg.LogFormat)) {
	case "text":
		h = slog.NewTextHandler(out, opts)
	default:
		h = slog.NewJSONHandler(out, opts)
	}

	logger := slog.New(h).With(
		slog.String("service.name", cfg.LogService),
		slog.String("service.version", cfg.AppVersion),
	)
	slog.SetDefault(logger)
	return logger
}

// Close releases the log file handle (defer from main, вызывается последним среди defers).
func Close() error {
	if logFile == nil {
		return nil
	}
	err := logFile.Close()
	logFile = nil
	return err
}

func openLogFile(path string) (*os.File, error) {
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	return os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
}

func parseLevel(s string) slog.Leveler {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Startup returns common attributes for process lifecycle logs.
func Startup(startAt time.Time) []slog.Attr {
	return []slog.Attr{
		slog.Time("process.started_at", startAt.UTC()),
	}
}
