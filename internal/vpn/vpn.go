// Package vpn wires together the device, routing, cipher, and tunnel layers
// into a runnable VPN node.
package vpn

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/govpn/govpn/internal/cipher"
	"github.com/govpn/govpn/internal/config"
	"github.com/govpn/govpn/internal/device"
	"github.com/govpn/govpn/internal/routing"
	"github.com/govpn/govpn/internal/tunnel"
)

// Node is a fully configured, runnable VPN endpoint.
// Construct it with New; run it with Run.
type Node struct {
	cfg    *config.Config
	dev    *device.TUN
	rtmgr  routing.Manager
	tun    *tunnel.Tunnel
	logger *slog.Logger
}

// New constructs a Node from cfg.
//
// New opens the TUN device, configures the network interface and routes,
// and initialises the encrypted tunnel. It does not start packet forwarding;
// call Run for that.
//
// If construction fails mid-way, any resources already acquired are released.
func New(cfg *config.Config, logger *slog.Logger) (*Node, error) {
	if logger == nil {
		logger = slog.Default()
	}

	c, err := cipher.NewFromPassphrase(cfg.Passphrase)
	if err != nil {
		return nil, fmt.Errorf("vpn: cipher: %w", err)
	}

	var devLogger *slog.Logger
	if cfg.Verbose {
		devLogger = logger
	}

	dev, err := device.Open(cfg.TUNName, cfg.MTU, devLogger)
	if err != nil {
		return nil, fmt.Errorf("vpn: open TUN device: %w", err)
	}

	rtmgr := routing.New()

	if err := rtmgr.ConfigureInterface(dev.Name(), cfg.LocalIP, cfg.PeerIP, cfg.MTU); err != nil {
		dev.Close()
		return nil, fmt.Errorf("vpn: configure interface: %w", err)
	}
	logger.Info("interface configured", "name", dev.Name(), "ip", cfg.LocalIP)

	for _, subnet := range cfg.Routes {
		if err := rtmgr.AddRoute(dev.Name(), subnet); err != nil {
			logger.Warn("route skipped", "subnet", subnet, "err", err)
			continue
		}
		logger.Info("route added", "subnet", subnet, "via", dev.Name())
	}

	var tunLogger *slog.Logger
	if cfg.Verbose {
		tunLogger = logger
	}

	opts := tunnel.Options{
		KeepaliveInterval: cfg.KeepaliveInterval.Duration,
		Logger:            tunLogger,
	}

	var tun *tunnel.Tunnel
	switch cfg.Mode {
	case config.ModeServer:
		tun, err = tunnel.NewServer(cfg.ListenAddr, c, dev, opts)
	case config.ModeClient:
		tun, err = tunnel.NewClient(cfg.ServerAddr, c, dev, opts)
	}
	if err != nil {
		// Clean up routes and device before returning.
		for _, subnet := range cfg.Routes {
			_ = rtmgr.DeleteRoute(dev.Name(), subnet)
		}
		dev.Close()
		return nil, fmt.Errorf("vpn: tunnel init: %w", err)
	}

	return &Node{
		cfg:    cfg,
		dev:    dev,
		rtmgr:  rtmgr,
		tun:    tun,
		logger: logger,
	}, nil
}

// Run starts packet forwarding and blocks until ctx is done or a fatal error
// occurs. It always cleans up routes and the TUN device before returning.
func (n *Node) Run(ctx context.Context) error {
	defer n.cleanup()

	n.logger.Info("VPN node running", "mode", n.cfg.Mode)
	return n.tun.Run(ctx)
}

// Stats returns a snapshot of current tunnel traffic counters.
func (n *Node) Stats() tunnel.Stats {
	return n.tun.Stats()
}

// InterfaceName returns the OS-assigned TUN interface name.
func (n *Node) InterfaceName() string {
	return n.dev.Name()
}

// cleanup removes routes and closes the TUN device.
func (n *Node) cleanup() {
	for _, subnet := range n.cfg.Routes {
		if err := n.rtmgr.DeleteRoute(n.dev.Name(), subnet); err != nil {
			n.logger.Warn("route cleanup failed", "subnet", subnet, "err", err)
		}
	}
	if err := n.dev.Close(); err != nil {
		n.logger.Warn("TUN close failed", "err", err)
	}
	n.logger.Info("VPN node stopped")
}
