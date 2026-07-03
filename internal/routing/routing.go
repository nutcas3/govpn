// Package routing abstracts OS-specific network interface configuration and
// routing table management behind a single Manager interface.
//
// Call New() to obtain the correct implementation for the current platform.
package routing

import "errors"

// ErrNotSupported is returned when the current platform has no implementation.
var ErrNotSupported = errors.New("routing: platform not supported")

// Manager configures a TUN interface and the kernel routing table.
type Manager interface {
	// ConfigureInterface assigns localCIDR (e.g. "10.8.0.1/24") to the named
	// TUN interface, sets its MTU, and brings it up.
	// peerIP is the opposite end of point-to-point links (required on macOS).
	ConfigureInterface(ifaceName, localCIDR, peerIP string, mtu int) error

	// AddRoute installs a kernel route for subnet (CIDR) via ifaceName.
	AddRoute(ifaceName, subnet string) error

	// DeleteRoute removes a previously installed route.
	DeleteRoute(ifaceName, subnet string) error
}
