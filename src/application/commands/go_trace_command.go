package commands

import (
	"context"
	"gtracer/src/common/decorator"
	"gtracer/src/ports_adapters/secondary/service/instrumented"
	"gtracer/src/ports_adapters/secondary/service/parser"
	"log/slog"
)

type goTraceCommand struct {
	instrumentedService *instrumented.Instrumented
	parserService       *parser.Parser
	logger              *slog.Logger
}

type TraceCommand struct {
	Cleared    bool
	TargetPath string
	OutputPath string
}

type GoTraceCommand decorator.CommandDecorator[TraceCommand, any]

func NewGoTraceCommand(parserService *parser.Parser, logger *slog.Logger, instrument *instrumented.Instrumented) decorator.CommandDecorator[TraceCommand, any] {
	handler := &goTraceCommand{
		instrumentedService: instrument,
		parserService:       parserService,
		logger:              logger,
	}
	return decorator.ApplyCommandDecorator[TraceCommand, any](handler, logger)
}

func (h *goTraceCommand) Handle(context.Context, TraceCommand) (any, error) {
	err := h.instrumentedService.Processed()
}
