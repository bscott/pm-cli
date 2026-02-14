package cli

import (
	"fmt"
	"runtime"
)

func (c *VersionCmd) Run(ctx *Context) error {
	if ctx.Formatter.JSON {
		return ctx.Formatter.PrintJSON(map[string]interface{}{
			"name":       "pm-cli",
			"version":    Version,
			"go_version": runtime.Version(),
			"os":         runtime.GOOS,
			"arch":       runtime.GOARCH,
		})
	}

	fmt.Printf("pm-cli version %s\n", Version)
	fmt.Printf("Go version: %s\n", runtime.Version())
	fmt.Printf("OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	return nil
}
