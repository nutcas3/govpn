package tunnel_test

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/govpn/govpn/internal/cipher"
	"github.com/govpn/govpn/internal/tunnel"
)

type mockDevice struct {
	mtu      int
	outgoing chan []byte
	incoming chan []byte
}

func newMockDevice(mtu int) *mockDevice {
	return &mockDevice{
		mtu:      mtu,
		outgoing: make(chan []byte, 64),
		incoming: make(chan []byte, 64),
	}
}

func (m *mockDevice) Name() string { return "mock0" }
func (m *mockDevice) MTU() int     { return m.mtu }
func (m *mockDevice) Close() error { return nil }

func (m *mockDevice) ReadPacket(buf []byte) ([]byte, error) {
	pkt := <-m.outgoing
	n := copy(buf, pkt)
	return buf[:n], nil
}

func (m *mockDevice) WritePacket(pkt []byte) error {
	cp := make([]byte, len(pkt))
	copy(cp, pkt)
	m.incoming <- cp
	return nil
}

func (m *mockDevice) recv(d time.Duration) ([]byte, bool) {
	select {
	case pkt := <-m.incoming:
		return pkt, true
	case <-time.After(d):
		return nil, false
	}
}

func freeAddr(t *testing.T) string {
	t.Helper()
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	port := conn.LocalAddr().(*net.UDPAddr).Port
	conn.Close()
	return fmt.Sprintf("127.0.0.1:%d", port)
}

// newPair creates a matched server+client tunnel pair and starts both.
// The client sends an initial packet so the server registers the client address
// before any test assertion runs.
func newPair(t *testing.T, passphrase string) (srv, cli *tunnel.Tunnel, srvDev, cliDev *mockDevice) {
	t.Helper()

	c, err := cipher.NewFromPassphrase(passphrase)
	if err != nil {
		t.Fatal(err)
	}

	addr := freeAddr(t)
	srvDev = newMockDevice(1400)
	cliDev = newMockDevice(1400)

	opts := tunnel.Options{KeepaliveInterval: 5 * time.Second}

	srv, err = tunnel.NewServer(addr, c, srvDev, opts)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	cli, err = tunnel.NewClient(addr, c, cliDev, opts)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		srv.Close()
		cli.Close()
	})

	go srv.Run(ctx) //nolint:errcheck
	go cli.Run(ctx) //nolint:errcheck

	// Send a registration packet from client so the server learns the client
	// UDP address (the PING is enough, but we also drain it from srvDev).
	// Allow time for the goroutines to start and the PING to arrive.
	time.Sleep(150 * time.Millisecond)

	// Register client addr explicitly by sending a data packet.
	regPkt := []byte{0x45, 0x00}
	if err := cli.Send(regPkt); err != nil {
		t.Fatalf("registration packet: %v", err)
	}
	// Drain it so test assertions start clean.
	srvDev.recv(500 * time.Millisecond)

	return srv, cli, srvDev, cliDev
}

func TestClientToServer(t *testing.T) {
	t.Parallel()

	_, cli, srvDev, _ := newPair(t, "test-passphrase")

	want := []byte{0x45, 0x00, 0x00, 0x28, 0x00, 0x01, 0x40, 0x00, 0x40, 0x11}
	if err := cli.Send(want); err != nil {
		t.Fatalf("Send: %v", err)
	}

	got, ok := srvDev.recv(2 * time.Second)
	if !ok {
		t.Fatal("timeout: no packet on server dev")
	}
	if !bytes.Equal(got, want) {
		t.Errorf("c→s mismatch:\n got  %x\n want %x", got, want)
	}
}

func TestServerToClient(t *testing.T) {
	t.Parallel()

	// newPair already registers the client addr via a data packet.
	srv, _, _, cliDev := newPair(t, "s2c-passphrase")

	want := []byte("server-to-client payload")
	if err := srv.Send(want); err != nil {
		t.Fatalf("srv.Send: %v", err)
	}

	got, ok := cliDev.recv(2 * time.Second)
	if !ok {
		t.Fatal("timeout: no packet on client dev")
	}
	if !bytes.Equal(got, want) {
		t.Errorf("s→c mismatch:\n got  %x\n want %x", got, want)
	}
}

func TestBidirectional(t *testing.T) {
	t.Parallel()

	srv, cli, srvDev, cliDev := newPair(t, "bidir")

	c2s := []byte("client→server")
	if err := cli.Send(c2s); err != nil {
		t.Fatalf("cli.Send: %v", err)
	}
	if pkt, ok := srvDev.recv(2 * time.Second); !ok || !bytes.Equal(pkt, c2s) {
		t.Errorf("c→s: got %q, want %q", pkt, c2s)
	}

	s2c := []byte("server→client")
	if err := srv.Send(s2c); err != nil {
		t.Fatalf("srv.Send: %v", err)
	}
	if pkt, ok := cliDev.recv(2 * time.Second); !ok || !bytes.Equal(pkt, s2c) {
		t.Errorf("s→c: got %q, want %q", pkt, s2c)
	}
}

func TestWrongKeyDropped(t *testing.T) {
	t.Parallel()

	goodCipher, _ := cipher.NewFromPassphrase("server-key")
	badCipher, _ := cipher.NewFromPassphrase("attacker-key")

	addr := freeAddr(t)
	srvDev := newMockDevice(1400)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	srv, err := tunnel.NewServer(addr, goodCipher, srvDev, tunnel.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Close)
	go srv.Run(ctx) //nolint:errcheck

	cliDev := newMockDevice(1400)
	cli, err := tunnel.NewClient(addr, badCipher, cliDev, tunnel.Options{KeepaliveInterval: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cli.Close)
	go cli.Run(ctx) //nolint:errcheck

	time.Sleep(80 * time.Millisecond)
	_ = cli.Send([]byte("should be dropped"))

	if pkt, ok := srvDev.recv(500 * time.Millisecond); ok {
		t.Errorf("server accepted packet with wrong key: %x", pkt)
	}
}

func TestStatsIncrement(t *testing.T) {
	t.Parallel()

	_, cli, srvDev, _ := newPair(t, "stats-test")

	const n = 5
	pkt := make([]byte, 100)
	for range n {
		if err := cli.Send(pkt); err != nil {
			t.Fatal(err)
		}
	}
	for range n {
		if _, ok := srvDev.recv(2 * time.Second); !ok {
			t.Fatal("timeout waiting for packet")
		}
	}

	stats := cli.Stats()
	// +1 for the registration packet sent in newPair
	if stats.TxPackets != n+1 {
		t.Errorf("TxPackets = %d, want %d", stats.TxPackets, n+1)
	}
}

func TestContextCancelStops(t *testing.T) {
	t.Parallel()

	c, _ := cipher.NewFromPassphrase("cancel-test")
	dev := newMockDevice(1400)

	ctx, cancel := context.WithCancel(context.Background())

	srv, err := tunnel.NewServer(freeAddr(t), c, dev, tunnel.Options{})
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { done <- srv.Run(ctx) }()

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run returned non-nil error after cancel: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Error("Run did not stop after context cancel")
	}
}
