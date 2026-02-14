package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	"github.com/bscott/pm-cli/internal/cli"
)

func main() {
	var c cli.CLI

	parser := kong.Must(&c,
		kong.Name("pm-cli"),
		kong.Description("ProtonMail CLI via Proton Bridge IMAP/SMTP"),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
		}),
	)

	// Handle --help-json before parsing to output full schema
	for _, arg := range os.Args[1:] {
		if arg == "--help-json" {
			if err := cli.PrintHelpJSON(&c); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}
	}

	ctx, err := parser.Parse(os.Args[1:])
	if err != nil {
		parser.FatalIfErrorf(err)
	}

	// Create execution context
	execCtx, err := cli.NewContext(&c.Globals)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Run the command
	err = ctx.Run(execCtx)
	if err != nil {
		if execCtx.Formatter.JSON {
			execCtx.Formatter.PrintJSON(map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			})
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		os.Exit(1)
	}
}
