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
		fmt.Fprint(os.Stderr, bold("govpn "+name)+" — "+description+"\n\n")
		fmt.Fprint(os.Stderr, dim("Usage:")+"\n")
		fmt.Fprint(os.Stderr, "  govpn "+name+" [flags]\n\n")
		fmt.Fprint(os.Stderr, dim("Flags:")+"\n")
		fs.PrintDefaults()
		fmt.Fprintln(os.Stderr)
	}

	return fs
}

// mustParse parses args into fs, printing usage and exiting on error.
func mustParse(fs *flag.FlagSet, args []string) {
	if err := fs.Parse(args); err != nil {
		fs.Usage()
		fatal(fmt.Sprintf("%v", err))
	}
}
