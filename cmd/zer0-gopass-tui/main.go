package main

import (
	"fmt"
	"os"

	"zer0-gopass-tui/internal/app"
	"zer0-gopass-tui/internal/gopass"
)

func main() {
	if err := app.Run(os.Stdin, os.Stdout, gopass.ExecLister{}, gopass.ExecReader{}, gopass.ExecWriter{}); err != nil {
		fmt.Fprintln(os.Stderr, "zer0-gopass-tui: internal error")
		os.Exit(1)
	}
}
