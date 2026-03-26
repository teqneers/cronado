package logging

import (
	"log"
	"log/slog"
	"os"

	"github.com/teqneers/cronado/internal/config"
)

func InitializeLogger(cfg *config.Config) {
	log.SetFlags(log.Ldate)

	var slogLevel slog.Level
	switch cfg.Log.Level {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}
	slog.SetLogLoggerLevel(slogLevel)

	switch cfg.Log.Format {
	case "json":
		handler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slogLevel})
		slog.SetDefault(slog.New(handler))
	default:
		handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slogLevel})
		slog.SetDefault(slog.New(handler))
	}
}
