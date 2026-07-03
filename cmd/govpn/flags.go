package main

import (
	"flag"
	"fmt"
	"os"
)

// newFlagSet creates a flag.FlagSet with a formatted Usage that matches the
// govpn style (bold heading, dim sub-sections).
func newFlagSet(name, description string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ExitOnError)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s — %s\n\n", bold("govpn "+name), description)
		fmt.Fprintf(os.Stderr, "%s\n", dim("Usage:"))
		fmt.Fprintf(os.Stderr, "  govpn %s [flags]\n\n", name)
		fmt.Fprintf(os.Stderr, "%s\n", dim("Flags:"))
		fs.PrintDefaults()
		fmt.Fprintln(os.Stderr)
	}

	return fs
}

// mustParse parses args into fs, printing usage and exiting on error.
func mustParse(fs *flag.FlagSet, args []string) {
	if err := fs.Parse(args); err != nil {
		fs.Usage()
		fatal("%v", err)
	}
}
