// Package config defines the VPN node configuration and its JSON serialisation.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

// Mode distinguishes server from client nodes.
type Mode string

const (
	ModeServer Mode = "server"
	ModeClient Mode = "client"
)

// Sentinel validation errors.
var (
	ErrUnknownMode        = errors.New("config: mode must be \"server\" or \"client\"")
	ErrEmptyPassphrase    = errors.New("config: passphrase must not be empty")
	ErrEmptyLocalIP       = errors.New("config: local_ip is required")
	ErrMissingListenAddr  = errors.New("config: server mode requires listen_addr")
	ErrMissingServerAddr  = errors.New("config: client mode requires server_addr")
	ErrInvalidMTU         = errors.New("config: mtu must be between 576 and 65535")
)

// Duration is a time.Duration that round-trips through JSON as a human-readable
// string (e.g. "10s", "1m30s") rather than a raw nanosecond integer.
type Duration struct{ time.Duration }

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("config: invalid duration %q: %w", s, err)
	}
	d.Duration = dur
	return nil
}

// Config is the full runtime configuration for a govpn node.
// All fields map 1-to-1 to JSON keys.
type Config struct {
	// Mode is "server" or "client".
	Mode Mode `json:"mode"`

	// ListenAddr is the UDP address a server binds to (e.g. "0.0.0.0:1194").
	// Required when Mode == ModeServer.
	ListenAddr string `json:"listen_addr,omitempty"`

	// ServerAddr is the UDP address a client dials (e.g. "vpn.example.com:1194").
	// Required when Mode == ModeClient.
	ServerAddr string `json:"server_addr,omitempty"`

	// TUNName is the requested TUN interface name.
	// On macOS the kernel ignores this and assigns a utunN name instead.
	TUNName string `json:"tun_name"`

	// LocalIP is the CIDR address for this node's VPN interface (e.g. "10.8.0.1/24").
	LocalIP string `json:"local_ip"`

	// PeerIP is the peer's VPN IP used for point-to-point links on macOS/Windows.
	PeerIP string `json:"peer_ip,omitempty"`

	// Routes lists CIDR subnets that should be routed through the tunnel.
	Routes []string `json:"routes"`

	// Passphrase is the pre-shared secret from which the symmetric key is
	// derived. Must be identical on server and client.
	Passphrase string `json:"passphrase"`

	// MTU is the TUN interface MTU. Lower than Ethernet (1500) to leave room
	// for UDP + IP encapsulation headers.
	MTU int `json:"mtu"`

	// KeepaliveInterval is how often the client sends PING frames to keep NAT
	// mappings alive and inform the server of its UDP address.
	KeepaliveInterval Duration `json:"keepalive_interval"`

	// Verbose enables per-packet debug logging via slog.
	Verbose bool `json:"verbose"`
}

// Defaults returns a Config populated with safe, sane defaults.
// Callers merge over these with values from a config file.
func Defaults() *Config {
	return &Config{
		TUNName:           "govpn0",
		MTU:               1400,
		KeepaliveInterval: Duration{15 * time.Second},
	}
}

// Load reads path, merges its contents over Defaults(), and validates.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}

	cfg := Defaults()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks all required fields and constraints.
// Errors wrap the package-level sentinel variables for errors.Is matching.
func (c *Config) Validate() error {
	if c.Mode != ModeServer && c.Mode != ModeClient {
		return fmt.Errorf("%w: got %q", ErrUnknownMode, c.Mode)
	}
	if c.Passphrase == "" {
		return ErrEmptyPassphrase
	}
	if c.LocalIP == "" {
		return ErrEmptyLocalIP
	}
	if c.Mode == ModeServer && c.ListenAddr == "" {
		return ErrMissingListenAddr
	}
	if c.Mode == ModeClient && c.ServerAddr == "" {
		return ErrMissingServerAddr
	}
	if c.MTU < 576 || c.MTU > 65535 {
		return fmt.Errorf("%w: got %d", ErrInvalidMTU, c.MTU)
	}
	return nil
}

// ExampleServer returns a fully populated example server Config.
func ExampleServer() *Config {
	return &Config{
		Mode:              ModeServer,
		ListenAddr:        "0.0.0.0:1194",
		TUNName:           "govpn0",
		LocalIP:           "10.8.0.1/24",
		PeerIP:            "10.8.0.2",
		Routes:            []string{"10.8.0.0/24"},
		Passphrase:        "change-me-to-something-strong",
		MTU:               1400,
		KeepaliveInterval: Duration{15 * time.Second},
		Verbose:           false,
	}
}

// ExampleClient returns a fully populated example client Config.
func ExampleClient() *Config {
	return &Config{
		Mode:              ModeClient,
		ServerAddr:        "YOUR_SERVER_IP:1194",
		TUNName:           "govpn0",
		LocalIP:           "10.8.0.2/24",
		PeerIP:            "10.8.0.1",
		Routes:            []string{"10.8.0.0/24"},
		Passphrase:        "change-me-to-something-strong",
		MTU:               1400,
		KeepaliveInterval: Duration{15 * time.Second},
		Verbose:           false,
	}
}
