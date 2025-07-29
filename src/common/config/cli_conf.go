package config

import (
	"errors"
	"github.com/urfave/cli/v2"
	_ "log"
	"os"
)

type Config interface {
	Validate() error
}

type GoTrace struct {
	TargetProject string
	OutputProject string
}

type CommandCli struct {
	GoTrace *GoTrace `cli_command:"gotrace"`
	LogLvl  uint8
}

type ServerCli struct {
	Port   string
	LogLvl uint8
}

func (c *CommandCli) Validate() error {
	if c.GoTrace.TargetProject == "" {
		return errors.New("target project is required")
	}
	return nil
}

func (c *ServerCli) Validate() error {
	if c.Port == "" {
		return errors.New("port is required")
	}
	return nil
}

func Execute() (Config, error) {
	var result Config

	app := &cli.App{
		Name:  "myapp",
		Usage: "Run in different modes",
		Commands: []*cli.Command{
			{
				Name:  "server",
				Usage: "Run as web server",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "port",
						Aliases: []string{"p"},
						Value:   "8080",
						Usage:   "Port to listen on",
					},
					&cli.UintFlag{
						Name:    "log",
						Aliases: []string{"l"},
						Value:   0,
						Usage:   "Log level (0-3)",
					},
				},
				Action: func(c *cli.Context) error {
					result = &ServerCli{
						LogLvl: uint8(c.Uint("log")),
						Port:   c.String("port"),
					}
					return nil
				},
			},
			{
				Name:  "run",
				Usage: "Run CLI commands",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "target",
						Aliases:  []string{"t"},
						Usage:    "Target project (required)",
						Required: true,
					},
					&cli.StringFlag{
						Name:    "output",
						Aliases: []string{"o"},
						Usage:   "Output project",
					},
					&cli.UintFlag{
						Name:    "log",
						Aliases: []string{"l"},
						Value:   0,
						Usage:   "Log level (0-3)",
					},
				},
				Action: func(c *cli.Context) error {
					result = &CommandCli{
						GoTrace: &GoTrace{
							TargetProject: c.String("target"),
							OutputProject: c.String("output"),
						},
						LogLvl: uint8(c.Uint("log")),
					}
					return nil
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		return nil, err
	}

	return result, nil
}
