package cli

import (
	"gtrace/src/application/commands"
	"gtrace/src/common/config"
	"gtrace/src/domain/cli"
)

func (c Cli) GoTrace(r *cli.Request) error {
	comm := r.Data.(config.GoTrace)
	command := commands.TraceCommand{
		Cleared:    false,
		TargetPath: comm.TargetProject,
		OutputPath: comm.OutputProject,
	}

	_, err := c.app.Commands.GoTraceCli.Handle(r.Ctx, command)
	if err != nil {
		return err
	}

	return nil

}
