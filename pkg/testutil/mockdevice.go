package testutil

import (
	"time"

	"github.com/govpn/govpn/internal/device"
)

type MockDevice struct {
	mtu      int
	outgoing chan []byte
	incoming chan []byte
}

func NewMockDevice(mtu int) *MockDevice {
	return &MockDevice{
		mtu:      mtu,
		outgoing: make(chan []byte, 64),
		incoming: make(chan []byte, 64),
	}
}

func (m *MockDevice) Name() string { return "mock0" }
func (m *MockDevice) MTU() int     { return m.mtu }
func (m *MockDevice) Close() error { return nil }

func (m *MockDevice) ReadPacket(buf []byte) ([]byte, error) {
	pkt := <-m.outgoing
	n := copy(buf, pkt)
	return buf[:n], nil
}

func (m *MockDevice) WritePacket(pkt []byte) error {
	cp := make([]byte, len(pkt))
	copy(cp, pkt)
	m.incoming <- cp
	return nil
}

func (m *MockDevice) Recv(d time.Duration) ([]byte, bool) {
	select {
	case pkt := <-m.incoming:
		return pkt, true
	case <-time.After(d):
		return nil, false
	}
}

var _ device.PacketDevice = (*MockDevice)(nil)
