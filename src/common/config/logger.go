package config

import (
	"log/slog"
	"os"
)

func InitLogger(logLvl uint8) *slog.Logger {
	logger := slog.New(slog.NewTextHandler(os.Stdout,
		&slog.HandlerOptions{
			AddSource: true,
			Level:     parseLogLevel(logLvl),
		}))
	slog.SetDefault(logger)
	return logger
}

// parseLogLevel возвращает уровень логирования
func parseLogLevel(logLvl uint8) slog.Level {
	switch logLvl {
	case 0:
		return slog.LevelInfo
	case 1:
		return slog.LevelWarn
	case 2:
		return slog.LevelError
	case 3:
		return slog.LevelDebug
	default:
		return slog.LevelInfo
	}
}
