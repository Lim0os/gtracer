package config

import (
	"context"
	"io"
	"log/slog"
	"os"
	"time"
)

func InitLogger(logLvl uint8) *slog.Logger {
	handler := &customHandler{
		output: os.Stdout,
		level:  parseLogLevel(logLvl),
	}
	logger := slog.New(handler)
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

type customHandler struct {
	output io.Writer
	level  slog.Level
}

func (h *customHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *customHandler) Handle(_ context.Context, r slog.Record) error {

	buf := make([]byte, 0, 256)

	buf = append(buf, '[')
	buf = append(buf, r.Level.String()...)
	buf = append(buf, ']', ' ')

	buf = append(buf, "time="...)
	buf = r.Time.AppendFormat(buf, time.RFC3339Nano)
	buf = append(buf, ' ')

	buf = append(buf, "msg="...)
	buf = append(buf, '"')
	buf = append(buf, r.Message...)
	buf = append(buf, '"')

	r.Attrs(func(attr slog.Attr) bool {
		if attr.Key != "time" && attr.Key != "msg" && attr.Key != "level" {
			buf = append(buf, ' ')
			buf = append(buf, attr.Key...)
			buf = append(buf, '=')
			buf = append(buf, attr.Value.String()...)
		}
		return true
	})

	buf = append(buf, '\n')

	_, err := h.output.Write(buf)
	return err
}

func (h *customHandler) WithAttrs(attrs []slog.Attr) slog.Handler {

	return h
}

func (h *customHandler) WithGroup(name string) slog.Handler {

	return h
}
