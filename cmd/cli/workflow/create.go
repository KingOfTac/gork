package workflow

import (
	"context"
	"fmt"

	"github.com/kingoftac/gork/cmd/cli/cli"
)

func CreateHandler(ctx context.Context) error {
	args := cli.Args(ctx)
	cmd := cli.CurrentCommand(ctx)

	fmt.Println("command", cmd.Name)
	fmt.Println("args", args)
	return nil
}
