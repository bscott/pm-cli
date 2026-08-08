package main

import (
	"fmt"
	"os"
	"strings"

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

	// Handle --help-json before parsing to output the full schema.
	//
	// Only a leading occurrence counts. Scanning the whole argument list
	// matched flag *values* too, so `mail send -s "--help-json"` printed the
	// schema and exited 0 without sending — a silent no-op for any script
	// passing user-controlled text. A "--" terminator also ends the scan.
	for _, arg := range os.Args[1:] {
		if arg == "--" {
			break
		}
		if arg == "--help-json" {
			if err := cli.PrintHelpJSON(&c); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}
		if !strings.HasPrefix(arg, "-") {
			break // first positional argument: everything after is command input
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
