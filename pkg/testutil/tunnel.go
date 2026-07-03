package testutil

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/govpn/govpn/internal/cipher"
	"github.com/govpn/govpn/internal/tunnel"
)

func FreeAddr(t *testing.T) string {
	t.Helper()
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	port := conn.LocalAddr().(*net.UDPAddr).Port
	conn.Close()
	return fmt.Sprintf("127.0.0.1:%d", port)
}

func NewPair(t *testing.T, passphrase string) (srv, cli *tunnel.Tunnel, srvDev, cliDev *MockDevice) {
	t.Helper()

	c, err := cipher.NewFromPassphrase(passphrase)
	if err != nil {
		t.Fatal(err)
	}

	addr := FreeAddr(t)
	srvDev = NewMockDevice(1400)
	cliDev = NewMockDevice(1400)

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

	go srv.Run(ctx)
	go cli.Run(ctx)

	time.Sleep(150 * time.Millisecond)

	regPkt := []byte{0x45, 0x00}
	if err := cli.Send(regPkt); err != nil {
		t.Fatalf("registration packet: %v", err)
	}
	srvDev.recv(500 * time.Millisecond)

	return srv, cli, srvDev, cliDev
}
