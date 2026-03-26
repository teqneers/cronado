package logging

import (
	"log"
	"log/slog"
	"os"
	"github.com/teqneers/cronado/internal/config"
)

func InitializeLogger(cfg *config.Config) {
	log.SetFlags(log.Ldate)

	slogLevel := slog.LevelInfo
	if cfg.Log.Level == "debug" {
		slogLevel = slog.LevelDebug
		slog.SetLogLoggerLevel(slog.LevelDebug)
	} else if cfg.Log.Level == "info" {
		slogLevel = slog.LevelInfo
		slog.SetLogLoggerLevel(slog.LevelInfo)
	} else if cfg.Log.Level == "warn" {
		slogLevel = slog.LevelWarn
		slog.SetLogLoggerLevel(slog.LevelWarn)
	} else if cfg.Log.Level == "error" {
		slogLevel = slog.LevelError
		slog.SetLogLoggerLevel(slog.LevelError)
	} else {
		slogLevel = slog.LevelInfo
		slog.SetLogLoggerLevel(slog.LevelInfo)
	}

	switch cfg.Log.Format {
	case "json":
		handler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slogLevel})
		slog.SetDefault(slog.New(handler))
	default:
		fallthrough
	case "text":
		handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slogLevel})
		slog.SetDefault(slog.New(handler))
	}
}
