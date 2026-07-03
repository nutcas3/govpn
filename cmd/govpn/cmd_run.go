package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/govpn/govpn/internal/config"
	"github.com/govpn/govpn/internal/tunnel"
	"github.com/govpn/govpn/internal/vpn"
)

func runServer(args []string) {
	fs := newFlagSet("server", "Start a VPN server node.\n\nRequires root/Administrator privileges to open a TUN device.")
	cfgPath := fs.String("config", "", "path to server JSON config file `(required)`")
	verbose := fs.Bool("verbose", false, "enable per-packet debug logging via slog")
	mustParse(fs, args)

	if *cfgPath == "" {
		fmt.Fprintln(os.Stderr, "")
		fs.Usage()
		fatal("--config is required\n\n  Generate one with: govpn init --mode server --out server.json")
	}

	cfg := loadAndCheck(*cfgPath, config.ModeServer)
	if *verbose {
		cfg.Verbose = true
	}

	runNode(cfg)
}

func runClient(args []string) {
	fs := newFlagSet("client", "Connect to a VPN server.\n\nRequires root/Administrator privileges to open a TUN device.")
	cfgPath := fs.String("config", "", "path to client JSON config file `(required)`")
	verbose := fs.Bool("verbose", false, "enable per-packet debug logging via slog")
	mustParse(fs, args)

	if *cfgPath == "" {
		fmt.Fprintln(os.Stderr, "")
		fs.Usage()
		fatal("--config is required\n\n  Generate one with: govpn init --mode client --out client.json")
	}

	cfg := loadAndCheck(*cfgPath, config.ModeClient)
	if *verbose {
		cfg.Verbose = true
	}

	runNode(cfg)
}

// runNode orchestrates the full startup sequence with UI feedback, then blocks
// until a signal or fatal error.
func runNode(cfg *config.Config) {
	printBanner(cfg)

	// ── slog logger ───────────────────────────────────────────────────────────
	logLevel := slog.LevelWarn
	if cfg.Verbose {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))

	// ── build node ────────────────────────────────────────────────────────────
	section("Initializing")

	sp := newSpinner("Building VPN node…")
	sp.start()

	node, err := vpn.New(cfg, logger)
	if err != nil {
		sp.finish(false, "Initialization failed")
		logErr(fmt.Sprintf("%v", err))
		if isPermissionErr(err) {
			logWarn("Hint: run with sudo (Linux/macOS) or as Administrator (Windows)")
		}
		os.Exit(1)
	}

	sp.finish(true, fmt.Sprintf("Interface %s  %s", bold(node.InterfaceName()), cyan(cfg.LocalIP)))

	printNodeInfo(cfg, node.InterfaceName())

	// ── run ───────────────────────────────────────────────────────────────────
	section("Running")
	logOK("Tunnel is up  " + dim("(Ctrl-C to stop)"))
	fmt.Fprintln(os.Stderr)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Live stats every 5 s.
	go printLiveStats(ctx, node)

	if err := node.Run(ctx); err != nil {
		logErr(fmt.Sprintf("Tunnel exited: %v", err))
		printSessionSummary(node.Stats())
		os.Exit(1)
	}

	fmt.Fprintln(os.Stderr)
	printSessionSummary(node.Stats())
}

// ── helpers ───────────────────────────────────────────────────────────────────

func loadAndCheck(path string, want config.Mode) *config.Config {
	sp := newSpinner(fmt.Sprintf("Loading %s", path))
	sp.start()

	cfg, err := config.Load(path)
	if err != nil {
		sp.finish(false, "Config error")
		fatal(fmt.Sprintf("%v", err))
	}

	if cfg.Mode != want {
		sp.finish(false, "Wrong config mode")
		fatal(fmt.Sprintf("config mode is %q — use 'govpn %s' for that mode", cfg.Mode, cfg.Mode))
	}

	sp.finish(true, fmt.Sprintf("Config loaded  %s", dim(path)))
	return cfg
}

func printBanner(cfg *config.Config) {
	modeStr := string(cfg.Mode)
	modeColored := cyan(modeStr)
	if cfg.Mode == config.ModeServer {
		modeColored = magenta(modeStr)
	}
	fmt.Fprint(os.Stderr, "\n  "+bold("govpn")+" "+dim("v"+Version)+"  "+modeColored+"\n")
}

func printNodeInfo(cfg *config.Config, ifaceName string) {
	rows := [][2]string{
		{"Interface", bold(ifaceName)},
		{"Local IP", cyan(cfg.LocalIP)},
		{"MTU", fmt.Sprintf("%d", cfg.MTU)},
	}
	switch cfg.Mode {
	case config.ModeServer:
		rows = append(rows, [2]string{"Listening", bold(cfg.ListenAddr)})
	case config.ModeClient:
		rows = append(rows, [2]string{"Server", bold(cfg.ServerAddr)})
	}
	for _, r := range cfg.Routes {
		rows = append(rows, [2]string{"Route", cyan(r)})
	}
	fmt.Fprintln(os.Stderr)
	printTable(rows)
	fmt.Fprintln(os.Stderr)
}

func printLiveStats(ctx context.Context, node *vpn.Node) {
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()

	var prevTxB, prevRxB uint64

	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			s := node.Stats()
			deltaTx := s.TxBytes - prevTxB
			deltaRx := s.RxBytes - prevRxB
			prevTxB, prevRxB = s.TxBytes, s.RxBytes

			fmt.Fprint(os.Stderr,
				"  "+dim("stats")+
					"  ↑ "+cyan(fmtBytes(deltaTx))+" ("+fmt.Sprintf("%d", s.TxPackets)+" pkts)"+
					"   ↓ "+green(fmtBytes(deltaRx))+" ("+fmt.Sprintf("%d", s.RxPackets)+" pkts)\n",
			)
		}
	}
}

func printSessionSummary(s tunnel.Stats) {
	section("Session summary")
	printTable([][2]string{
		{"Sent", fmt.Sprintf("%s in %d packets", cyan(fmtBytes(s.TxBytes)), s.TxPackets)},
		{"Received", fmt.Sprintf("%s in %d packets", green(fmtBytes(s.RxBytes)), s.RxPackets)},
	})
	fmt.Fprintln(os.Stderr)
}

func isPermissionErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return contains(msg, "permission denied") ||
		contains(msg, "operation not permitted") ||
		contains(msg, "Access is denied")
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsRune(s, sub))
}

func containsRune(s, sub string) bool {
	for i := range s {
		if i+len(sub) <= len(s) && s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
