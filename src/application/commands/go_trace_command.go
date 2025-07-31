package commands

import (
	"context"
	"fmt"
	"gtrace/src/common/decorator"
	"gtrace/src/ports_adapters/secondary/service/instrumented"
	"gtrace/src/ports_adapters/secondary/service/parser"

	"log/slog"
	"os/exec"
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

const (
	instrumentedLog = "instrumented.log"
)

type GoTraceCommand decorator.CommandDecorator[TraceCommand, any]

func NewGoTraceCommand(parserService *parser.Parser, logger *slog.Logger, instrument *instrumented.Instrumented) decorator.CommandDecorator[TraceCommand, any] {
	handler := &goTraceCommand{
		instrumentedService: instrument,
		parserService:       parserService,
		logger:              logger,
	}
	return decorator.ApplyCommandDecorator[TraceCommand, any](handler, logger)
}

func (h *goTraceCommand) Handle(ctx context.Context, command TraceCommand) (any, error) {
	h.logger.Info("Начало выполнения команды Trace", "targetPath", command.TargetPath, "outputPath", command.OutputPath)

	if err := h.instrumentedService.Processed(command.TargetPath, command.OutputPath); err != nil {
		h.logger.Error("Ошибка при инструментировании проекта", "error", err)
		return nil, fmt.Errorf("инструментирование проекта: %w", err)
	}

	cmd := exec.Command("sh", "-c", fmt.Sprintf("cd %s && go run main.go > %s", command.OutputPath, instrumentedLog))
	h.logger.Debug("Формирование команды запуска", "cmd", cmd.String())

	if err := cmd.Start(); err != nil {
		return nil, err
	}
	if err := cmd.Wait(); err != nil {
		h.logger.Error("Ошибка завершения команды", "error", err)
		return nil, fmt.Errorf("wait command: %w", err)
	}
	graph, err := h.parserService.ParseFromFile(fmt.Sprintf("%s/%s", command.OutputPath, instrumentedLog))
	if err != nil {
		return nil, err
	}
	fmt.Println(graph)

	return nil, nil
}
