package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/govpn/govpn/internal/config"
)

func runInit(args []string) {
	fs := newFlagSet("init", "Generate a govpn config file.\n\nWrites a ready-to-edit JSON config to --out (or stdout).")
	mode := fs.String("mode", "", `node mode: "server" or "client" (required)`)
	out := fs.String("out", "", "write config to this file (default: stdout)")
	mustParse(fs, args)

	if *mode == "" {
		fmt.Fprintln(os.Stderr)
		fs.Usage()
		fatal("--mode is required")
	}

	var cfg *config.Config
	switch config.Mode(*mode) {
	case config.ModeServer:
		cfg = config.ExampleServer()
	case config.ModeClient:
		cfg = config.ExampleClient()
	default:
		fatal(fmt.Sprintf("--mode must be \"server\" or \"client\", got %q", *mode))
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		fatal(fmt.Sprintf("marshal config: %v", err))
	}
	data = append(data, '\n')

	if *out == "" {
		os.Stdout.Write(data)
		return
	}

	sp := newSpinner(fmt.Sprintf("Writing %s", *out))
	sp.start()

	if err := os.WriteFile(*out, data, 0o600); err != nil {
		sp.finish(false, "Write failed")
		fatal(fmt.Sprintf("%v", err))
	}

	sp.finish(true, fmt.Sprintf("Config written to %s", bold(*out)))
	fmt.Fprintln(os.Stderr)

	printTable([][2]string{
		{"Mode", bold(string(cfg.Mode))},
		{"Local IP", cyan(cfg.LocalIP)},
		{"Passphrase", yellow("← change this!")},
	})

	if cfg.Mode == config.ModeServer {
		fmt.Fprint(os.Stderr, "\n  "+dim("Start with:")+"  sudo govpn server --config "+*out+"\n\n")
	} else {
		fmt.Fprint(os.Stderr, "\n  "+dim("Next step:")+"  Edit "+bold(*out)+" and set "+cyan("server_addr")+" to your server's address.\n\n")
	}
}
