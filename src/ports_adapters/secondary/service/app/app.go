package app

import (
	"gtrace/src/application"
	"gtrace/src/application/commands"
	"gtrace/src/ports_adapters/secondary/service/instrumented"
	"gtrace/src/ports_adapters/secondary/service/parser"
	"log/slog"
)

func InitApp(logger *slog.Logger) *application.App {
	instrument := instrumented.New(logger)
	pars := parser.NewParser(logger)

	return &application.App{
		Commands: application.Command{
			GoTraceCli: commands.NewGoTraceCommand(pars, logger, instrument),
		},
	}
}
