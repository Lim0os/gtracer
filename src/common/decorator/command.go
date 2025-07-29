package decorator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

type CommandDecorator[T any, C any] interface {
	Handle(context.Context, T) (C, error)
}

func ApplyCommandDecorator[T any, C any](handler CommandDecorator[T, C], logger *slog.Logger) CommandDecorator[T, C] {
	return CommandLoggingDecorator[T, C]{
		logger: logger,
		base:   handler,
	}
}

func generateActionName(handler any) string {

	return strings.Split(fmt.Sprintf("%T", handler), ".")[0]
}
