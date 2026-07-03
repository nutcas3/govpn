// govpn is a minimal cross-platform Layer-3 VPN.
//
// Usage:
//
//	govpn <command> [flags]
//
// Commands:
//
//	server   Start a VPN server
//	client   Connect to a VPN server
//	init     Generate a config file
//	version  Print version information
package main

import (
	"fmt"
	"os"
)

// Version is set at build time via -ldflags "-X main.Version=x.y.z".
var Version = "dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(0)
	}

	cmd, args := os.Args[1], os.Args[2:]

	switch cmd {
	case "server":
		runServer(args)
	case "client":
		runClient(args)
	case "init":
		runInit(args)
	case "version", "--version", "-version":
		runVersion()
	case "help", "--help", "-help", "-h":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "%s: unknown command %q\n\n", progname(), cmd)
		usage()
		os.Exit(2)
	}
}

func usage() {
	w := os.Stderr
	fmt.Fprintf(w, "%s  %s\n\n", bold("govpn"), dim("— a minimal cross-platform VPN"))
	fmt.Fprintf(w, "%s\n", dim("Usage:"))
	fmt.Fprintf(w, "  govpn <command> [flags]\n\n")
	fmt.Fprintf(w, "%s\n", dim("Commands:"))
	printCmd("server",  "Start a VPN server node")
	printCmd("client",  "Connect to a VPN server")
	printCmd("init",    "Generate a config file interactively")
	printCmd("version", "Print version and build info")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%s\n", dim("Examples:"))
	fmt.Fprintf(w, "  govpn init --mode server --out server.json\n")
	fmt.Fprintf(w, "  govpn init --mode client --out client.json\n")
	fmt.Fprintf(w, "  sudo govpn server --config server.json\n")
	fmt.Fprintf(w, "  sudo govpn client --config client.json\n")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%s\n", dim("Run 'govpn <command> --help' for command-specific flags."))
}

func printCmd(name, desc string) {
	fmt.Fprintf(os.Stderr, "  %-10s %s\n", bold(name), desc)
}

func progname() string { return "govpn" }
