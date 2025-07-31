package cli

import "gtrace/src/application"

type Cli struct {
	app application.App
}

func NewCli(app application.App) *Cli {
	return &Cli{app: app}
}
