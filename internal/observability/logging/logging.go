package logging

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/usenorn/norn/internal/config"
)

func New(cfg config.App) (*slog.Logger, error) {
	level, err := parseLevel(cfg.LogLevel)
	if err != nil {
		return nil, err
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})

	return slog.New(redactHandler{inner: handler}).With(
		slog.String("service", cfg.Name),
		slog.String("version", cfg.Version),
		slog.String("env", cfg.Env),
	), nil
}

func parseLevel(name string) (slog.Level, error) {
	switch strings.ToLower(name) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unknown log level %q", name)
	}
}
