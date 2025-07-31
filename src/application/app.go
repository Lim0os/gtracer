package application

import "gtrace/src/application/commands"

type App struct {
	Commands Command
}

type Command struct {
	GoTraceCli commands.GoTraceCommand
}
