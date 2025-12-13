package cli

import (
	"context"
	"flag"
	"fmt"
)

type Handler func(ctx context.Context) error

type ctxKey int

const (
	ctxArgsKey ctxKey = iota
	ctxCommandKey
)

type Command struct {
	Name        string
	Description string

	Flags    func(fs *flag.FlagSet)
	Handler  Handler
	Commands []*Command
}

type CLI struct {
	Root *Command
}

func (c *CLI) Run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return c.printHelp(c.Root)
	}

	return c.execute(ctx, c.Root, args)
}

func (c *CLI) execute(
	ctx context.Context,
	cmd *Command,
	args []string,
) error {
	if len(args) > 0 {
		for _, sub := range cmd.Commands {
			if sub.Name == args[0] {
				return c.execute(ctx, sub, args[1:])
			}
		}
	}

	fs := flag.NewFlagSet(cmd.Name, flag.ContinueOnError)

	if cmd.Flags != nil {
		cmd.Flags(fs)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx = context.WithValue(ctx, ctxArgsKey, fs.Args())
	ctx = context.WithValue(ctx, ctxCommandKey, cmd)

	if cmd.Handler == nil {
		return c.printHelp(cmd)
	}

	return cmd.Handler(ctx)
}

func Args(ctx context.Context) []string {
	if v := ctx.Value(ctxArgsKey); v != nil {
		return v.([]string)
	}
	return nil
}

func CurrentCommand(ctx context.Context) *Command {
	if v := ctx.Value(ctxCommandKey); v != nil {
		return v.(*Command)
	}
	return nil
}

func (c *CLI) printHelp(cmd *Command) error {
	fmt.Println(cmd.Name)
	fmt.Println(cmd.Description)

	for _, sub := range cmd.Commands {
		fmt.Printf("  %s\t%s\n", sub.Name, sub.Description)
	}
	return nil
}

type Middleware func(Handler) Handler

func ApplyMiddleware(h Handler, m ...Middleware) Handler {
	for i := len(m) - 1; i >= 0; i-- {
		h = m[i](h)
	}
	return h
}

func WithLogging(next Handler) Handler {
	return func(ctx context.Context) error {
		cmd := CurrentCommand(ctx)
		fmt.Println("running:", cmd.Name)
		return next(ctx)
	}
}
