// Package device defines the PacketDevice interface and provides a TUN-backed
// implementation using github.com/songgao/water.
//
// Separating the interface from the tunnel package means the tunnel can be
// tested without root privileges by substituting a mock device.
package device

import (
	"fmt"
	"log/slog"

	"github.com/songgao/water"
)

// PacketDevice is the interface the tunnel uses to exchange raw IP packets with
// the operating system network stack. *TUN satisfies this interface; so do
// test mocks.
type PacketDevice interface {
	// Name returns the OS-assigned interface name (e.g. "govpn0", "utun2").
	Name() string

	// MTU returns the maximum IP packet size the device accepts.
	MTU() int

	// ReadPacket blocks until a packet arrives and copies it into buf.
	// Returns the populated sub-slice of buf.
	ReadPacket(buf []byte) ([]byte, error)

	// WritePacket injects pkt into the OS network stack.
	WritePacket(pkt []byte) error

	// Close releases the underlying OS resources.
	Close() error
}

// TUN wraps a water TUN interface and implements PacketDevice.
type TUN struct {
	iface  *water.Interface
	mtu    int
	logger *slog.Logger
}

// Open creates and configures a TUN device.
//
// The name hint is honoured on Linux; macOS always assigns a utunN name.
// Pass a nil logger to discard all log output.
func Open(name string, mtu int, logger *slog.Logger) (*TUN, error) {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(noopWriter{}, nil))
	}

	cfg := water.Config{DeviceType: water.TUN}
	applyName(&cfg, name)

	iface, err := water.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("device: open TUN: %w", err)
	}

	d := &TUN{iface: iface, mtu: mtu, logger: logger}
	logger.Info("TUN device opened", "name", d.Name(), "mtu", mtu)

	return d, nil
}

// Name returns the OS-assigned interface name.
func (d *TUN) Name() string { return d.iface.Name() }

// MTU returns the configured MTU.
func (d *TUN) MTU() int { return d.mtu }

// ReadPacket reads the next raw IP packet from the TUN device.
// It returns a sub-slice of buf; callers must not retain it past the
// next ReadPacket call.
func (d *TUN) ReadPacket(buf []byte) ([]byte, error) {
	n, err := d.iface.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("device: read: %w", err)
	}

	d.logger.Debug("TUN read", "bytes", n)
	return buf[:n], nil
}

// WritePacket injects a raw IP packet into the OS network stack.
func (d *TUN) WritePacket(pkt []byte) error {
	if _, err := d.iface.Write(pkt); err != nil {
		return fmt.Errorf("device: write: %w", err)
	}

	d.logger.Debug("TUN write", "bytes", len(pkt))
	return nil
}

// Close releases the TUN device.
func (d *TUN) Close() error {
	return d.iface.Close()
}

// noopWriter discards all log output.
type noopWriter struct{}

func (noopWriter) Write(p []byte) (int, error) { return len(p), nil }
