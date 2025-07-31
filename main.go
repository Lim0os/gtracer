package main

import (
	"context"
	"fmt"
	"gtrace/src/common/config"
	"gtrace/src/ports_adapters/primary/cli"
	"gtrace/src/ports_adapters/secondary/service/app"

	clir "gtrace/src/domain/cli"

	"log/slog"
	"os"
	"reflect"
)

func main() {
	conf, err := config.Execute()
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	switch conf.(type) {
	case *config.CommandCli:
		startCli(*conf.(*config.CommandCli))
	case *config.ServerCli:
		startServer(*conf.(*config.ServerCli))

	default:
		slog.Error("Error starting")
	}
}

func startCli(conf config.CommandCli) {
	logger := config.InitLogger(conf.LogLvl)
	logger.Info("starting cli")
	application := app.InitApp(logger)
	c := cli.NewCli(*application)
	cliRouter(conf, "gotrace", context.Context(context.TODO()), c.GoTrace)

}

func startServer(conf config.ServerCli) {
	logger := config.InitLogger(conf.LogLvl)
	logger.Info("starting server")

}

func cliRouter(cmd config.CommandCli, tag string, ctx context.Context, fn func(r *clir.Request) error) {
	val := reflect.ValueOf(cmd)
	typ := val.Type()

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		commandTag, hasTag := field.Tag.Lookup("cli_command")

		if hasTag && commandTag == tag {
			fieldValue := val.Field(i)

			var nestedStruct any
			if fieldValue.Kind() == reflect.Ptr && !fieldValue.IsNil() {
				nestedStruct = fieldValue.Elem().Interface()
			} else if fieldValue.Kind() != reflect.Ptr {
				nestedStruct = fieldValue.Interface()
			} else {
				slog.Warn(fmt.Sprintf("command %s is nil", tag))
				return
			}
			r := &clir.Request{
				Ctx:  ctx,
				Data: nestedStruct,
			}

			err := fn(r)
			if err != nil {
				slog.Error(err.Error())
				return
			}
			return
		}
	}
	slog.Warn(fmt.Sprintf("command %s not found", tag))
	return
}
