package main

import (
	"context"
	"fmt"
	"gtracer/src/common/config"
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
	cliRouter(conf, "gotrace", context.Context(context.TODO()), func(ctx context.Context, a any) error {
		return fmt.Errorf("TestEroor")
	})

}

func startServer(conf config.ServerCli) {
	logger := config.InitLogger(conf.LogLvl)
	logger.Info("starting server")

}

func cliRouter(cmd config.CommandCli, tag string, ctx context.Context, fn func(context.Context, any) error) {
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

			err := fn(ctx, nestedStruct)
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
