// Package tunnel implements the encrypted UDP transport layer.
//
// # Wire format
//
// Every UDP datagram begins with a 3-byte frame header:
//
//	+----------+----------+--------------------+
//	| type (1) | len  (2) | payload (variable) |
//	+----------+----------+--------------------+
//
// Frame types:
//
//	0x01  DATA  — encrypted IP packet (nonce || ciphertext || tag)
//	0x02  PING  — keepalive request, no payload
//	0x03  PONG  — keepalive reply,   no payload
package tunnel

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/govpn/govpn/internal/cipher"
	"github.com/govpn/govpn/internal/device"
)

// Frame type constants.
const (
	frameData = 0x01
	framePing = 0x02
	framePong = 0x03
)

// recvBufSize must fit the largest possible incoming frame:
// header(3) + nonce(12) + ciphertext(≤1500) + tag(16) = ~1531
const recvBufSize = 2048

// Stats is a snapshot of cumulative traffic counters.
type Stats struct {
	TxPackets uint64
	RxPackets uint64
	TxBytes   uint64
	RxBytes   uint64
}

// Options configure a Tunnel at construction time.
type Options struct {
	// KeepaliveInterval is how often the client sends PING frames to keep NAT
	// mappings alive. Zero disables keepalives (suitable for servers).
	KeepaliveInterval time.Duration

	// Logger receives structured log output. nil discards all logs.
	Logger *slog.Logger
}

// Tunnel manages the encrypted bidirectional UDP pipe between two VPN nodes.
type Tunnel struct {
	conn   *net.UDPConn
	cipher *cipher.AEAD
	dev    device.PacketDevice
	log    *slog.Logger

	// serverAddr is non-nil on client Tunnels.
	serverAddr *net.UDPAddr
	keepalive  time.Duration

	// peers tracks client UDP addresses on the server side.
	peersMu sync.RWMutex
	peers   map[string]*net.UDPAddr

	// Counters — updated atomically.
	txPackets atomic.Uint64
	rxPackets atomic.Uint64
	txBytes   atomic.Uint64
	rxBytes   atomic.Uint64

	// closeOnce ensures resources are released exactly once.
	// cancel is stored as a channel to avoid a write→read race between
	// Run (which stores the cancel func) and Close (which calls it).
	closeOnce sync.Once
	cancelCh  chan context.CancelFunc
}

// NewServer returns a Tunnel that listens on listenAddr.
func NewServer(listenAddr string, c *cipher.AEAD, dev device.PacketDevice, opts Options) (*Tunnel, error) {
	addr, err := net.ResolveUDPAddr("udp", listenAddr)
	if err != nil {
		return nil, fmt.Errorf("tunnel: resolve listen addr: %w", err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("tunnel: listen %s: %w", listenAddr, err)
	}
	t := newTunnel(conn, c, dev, nil, opts)
	t.log.Info("server listening", "addr", listenAddr)
	return t, nil
}

// NewClient returns a Tunnel that sends to serverAddr.
func NewClient(serverAddr string, c *cipher.AEAD, dev device.PacketDevice, opts Options) (*Tunnel, error) {
	remote, err := net.ResolveUDPAddr("udp", serverAddr)
	if err != nil {
		return nil, fmt.Errorf("tunnel: resolve server addr: %w", err)
	}
	conn, err := net.ListenUDP("udp", &net.UDPAddr{})
	if err != nil {
		return nil, fmt.Errorf("tunnel: bind client socket: %w", err)
	}
	t := newTunnel(conn, c, dev, remote, opts)
	t.log.Info("client ready", "server", serverAddr)
	return t, nil
}

func newTunnel(conn *net.UDPConn, c *cipher.AEAD, dev device.PacketDevice, serverAddr *net.UDPAddr, opts Options) *Tunnel {
	logger := opts.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(noopWriter{}, nil))
	}
	return &Tunnel{
		conn:       conn,
		cipher:     c,
		dev:        dev,
		log:        logger,
		serverAddr: serverAddr,
		keepalive:  opts.KeepaliveInterval,
		peers:      make(map[string]*net.UDPAddr),
		cancelCh:   make(chan context.CancelFunc, 1),
	}
}

// Run starts the packet pump and blocks until ctx is cancelled or a fatal
// error occurs. Safe to call Close concurrently.
func (t *Tunnel) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	// Publish the cancel func so Close can call it without a data race.
	t.cancelCh <- cancel
	defer cancel()

	errCh := make(chan error, 1)

	go func() {
		if err := t.pumpTUNtoUDP(ctx); err != nil && !errors.Is(err, context.Canceled) {
			errCh <- err
		}
	}()

	go func() {
		if err := t.pumpUDPtoTUN(ctx); err != nil && !errors.Is(err, context.Canceled) {
			errCh <- err
		}
	}()

	if t.serverAddr != nil && t.keepalive > 0 {
		go t.keepaliveLoop(ctx)
	}

	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return err
	}
}

// Close shuts the tunnel down. Safe to call before, during, or after Run;
// safe to call more than once.
func (t *Tunnel) Close() {
	t.closeOnce.Do(func() {
		// Cancel the context created inside Run if Run has started.
		select {
		case cancel := <-t.cancelCh:
			cancel()
		default:
			// Run has not been called yet — nothing to cancel.
		}
		t.conn.Close()
	})
}

// Stats returns a snapshot of cumulative traffic counters.
func (t *Tunnel) Stats() Stats {
	return Stats{
		TxPackets: t.txPackets.Load(),
		RxPackets: t.rxPackets.Load(),
		TxBytes:   t.txBytes.Load(),
		RxBytes:   t.rxBytes.Load(),
	}
}

// Send encrypts pkt and delivers it to the peer(s).
// Exported so integration tests can inject packets without a live TUN device.
func (t *Tunnel) Send(pkt []byte) error {
	return t.sendData(pkt)
}

// ── pumps ─────────────────────────────────────────────────────────────────────

func (t *Tunnel) pumpTUNtoUDP(ctx context.Context) error {
	buf := make([]byte, t.dev.MTU()+64)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		pkt, err := t.dev.ReadPacket(buf)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("tunnel: TUN read: %w", err)
		}
		if err := t.sendData(pkt); err != nil {
			t.log.Warn("send error", "err", err)
		}
	}
}

func (t *Tunnel) pumpUDPtoTUN(ctx context.Context) error {
	buf := make([]byte, recvBufSize)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		_ = t.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, from, err := t.conn.ReadFromUDP(buf)
		if err != nil {
			var opErr *net.OpError
			if errors.As(err, &opErr) && opErr.Timeout() {
				continue
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("tunnel: UDP read: %w", err)
		}
		if err := t.handleFrame(buf[:n], from); err != nil {
			t.log.Warn("frame error", "from", from, "err", err)
		}
	}
}

func (t *Tunnel) sendData(pkt []byte) error {
	enc, err := t.cipher.Encrypt(pkt)
	if err != nil {
		return fmt.Errorf("tunnel: encrypt: %w", err)
	}
	frame := buildFrame(frameData, enc)

	if t.serverAddr != nil {
		if _, err := t.conn.WriteToUDP(frame, t.serverAddr); err != nil {
			return fmt.Errorf("tunnel: UDP write: %w", err)
		}
	} else {
		t.peersMu.RLock()
		for _, addr := range t.peers {
			if _, err := t.conn.WriteToUDP(frame, addr); err != nil {
				t.log.Warn("send to peer failed", "peer", addr, "err", err)
			}
		}
		t.peersMu.RUnlock()
	}

	t.txPackets.Add(1)
	t.txBytes.Add(uint64(len(pkt)))
	return nil
}

func (t *Tunnel) handleFrame(buf []byte, from *net.UDPAddr) error {
	if len(buf) < 3 {
		return nil
	}
	ft := buf[0]
	payloadLen := int(binary.BigEndian.Uint16(buf[1:3]))
	if len(buf) < 3+payloadLen {
		return nil
	}
	payload := buf[3 : 3+payloadLen]

	switch ft {
	case frameData:
		return t.handleData(payload, from)
	case framePing:
		return t.handlePing(from)
	case framePong:
		t.log.Debug("pong", "from", from)
	}
	return nil
}

func (t *Tunnel) handleData(payload []byte, from *net.UDPAddr) error {
	if t.serverAddr == nil {
		key := from.String()
		t.peersMu.Lock()
		if _, exists := t.peers[key]; !exists {
			t.peers[key] = from
			t.log.Info("peer registered", "addr", from)
		}
		t.peersMu.Unlock()
	}

	pkt, err := t.cipher.Decrypt(payload)
	if err != nil {
		t.log.Warn("decrypt failed — wrong key or tampered packet", "from", from)
		return nil // swallow: bad packets must not crash the tunnel
	}

	t.rxPackets.Add(1)
	t.rxBytes.Add(uint64(len(pkt)))
	t.log.Debug("data received", "from", from, "bytes", len(pkt))
	return t.dev.WritePacket(pkt)
}

func (t *Tunnel) handlePing(from *net.UDPAddr) error {
	t.log.Debug("ping", "from", from)
	_, err := t.conn.WriteToUDP(buildFrame(framePong, nil), from)
	return err
}

func (t *Tunnel) keepaliveLoop(ctx context.Context) {
	ping := buildFrame(framePing, nil)
	if _, err := t.conn.WriteToUDP(ping, t.serverAddr); err != nil {
		t.log.Warn("initial ping failed", "err", err)
	}

	tick := time.NewTicker(t.keepalive)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if _, err := t.conn.WriteToUDP(ping, t.serverAddr); err != nil {
				t.log.Warn("keepalive ping failed", "err", err)
			}
		}
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func buildFrame(ft byte, payload []byte) []byte {
	frame := make([]byte, 3+len(payload))
	frame[0] = ft
	binary.BigEndian.PutUint16(frame[1:3], uint16(len(payload)))
	copy(frame[3:], payload)
	return frame
}

type noopWriter struct{}

func (noopWriter) Write(p []byte) (int, error) { return len(p), nil }
