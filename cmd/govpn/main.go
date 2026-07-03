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
		fmt.Fprint(os.Stderr, progname()+": unknown command "+fmt.Sprintf("%q", cmd)+"\n\n")
		usage()
		os.Exit(2)
	}
}

func usage() {
	w := os.Stderr
	fmt.Fprint(w, bold("govpn")+"  "+dim("— a minimal cross-platform VPN")+"\n\n")
	fmt.Fprint(w, dim("Usage:")+"\n")
	fmt.Fprint(w, "  govpn <command> [flags]\n\n")
	fmt.Fprint(w, dim("Commands:")+"\n")
	printCmd("server", "Start a VPN server node")
	printCmd("client", "Connect to a VPN server")
	printCmd("init", "Generate a config file interactively")
	printCmd("version", "Print version and build info")
	fmt.Fprintln(w)
	fmt.Fprint(w, dim("Examples:")+"\n")
	fmt.Fprint(w, "  govpn init --mode server --out server.json\n")
	fmt.Fprint(w, "  govpn init --mode client --out client.json\n")
	fmt.Fprint(w, "  sudo govpn server --config server.json\n")
	fmt.Fprint(w, "  sudo govpn client --config client.json\n")
	fmt.Fprintln(w)
	fmt.Fprint(w, dim("Run 'govpn <command> --help' for command-specific flags.")+"\n")
}

func printCmd(name, desc string) {
	fmt.Fprint(os.Stderr, "  "+bold(name)+"  "+desc+"\n")
}

func progname() string { return "govpn" }
