package cli

import "context"

type Request struct {
	Ctx  context.Context
	Data any
}
