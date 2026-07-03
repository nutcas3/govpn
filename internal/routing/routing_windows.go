//go:build windows

package routing

import (
	"fmt"
	"os/exec"
	"strings"
)

// New returns the Windows Manager (netsh + route).
// Requires the Wintun driver: https://www.wintun.net/
func New() Manager { return &windowsManager{} }

type windowsManager struct{}

func (m *windowsManager) ConfigureInterface(name, localCIDR, _ string, mtu int) error {
	localIP, mask, err := cidrToIPMask(localCIDR)
	if err != nil {
		return err
	}

	if err := run("netsh", "interface", "ip", "set", "address",
		"name="+name, "static", localIP, mask); err != nil {
		return fmt.Errorf("routing: set ip on %s: %w", name, err)
	}
	if err := run("netsh", "interface", "ipv4", "set", "subinterface",
		name, fmt.Sprintf("mtu=%d", mtu), "store=active"); err != nil {
		return fmt.Errorf("routing: set mtu on %s: %w", name, err)
	}
	return nil
}

func (m *windowsManager) AddRoute(name, subnet string) error {
	dest, mask, err := cidrToIPMask(subnet)
	if err != nil {
		return err
	}
	if err := run("route", "ADD", dest, "MASK", mask, "0.0.0.0", "IF", name); err != nil {
		return fmt.Errorf("routing: add route %s via %s: %w", subnet, name, err)
	}
	return nil
}

func (m *windowsManager) DeleteRoute(_, subnet string) error {
	dest, mask, err := cidrToIPMask(subnet)
	if err != nil {
		return err
	}
	if err := run("route", "DELETE", dest, "MASK", mask); err != nil {
		return fmt.Errorf("routing: delete route %s: %w", subnet, err)
	}
	return nil
}

// cidrToIPMask converts "10.8.0.0/24" → ("10.8.0.0", "255.255.255.0", nil).
func cidrToIPMask(cidr string) (ip, mask string, err error) {
	ip, prefixStr, ok := strings.Cut(cidr, "/")
	if !ok {
		return "", "", fmt.Errorf("routing: invalid CIDR %q", cidr)
	}
	var prefix int
	if _, err := fmt.Sscanf(prefixStr, "%d", &prefix); err != nil || prefix < 0 || prefix > 32 {
		return "", "", fmt.Errorf("routing: invalid prefix in %q", cidr)
	}
	var m [4]byte
	for i := range prefix {
		m[i/8] |= 1 << (7 - uint(i%8))
	}
	return ip, fmt.Sprintf("%d.%d.%d.%d", m[0], m[1], m[2], m[3]), nil
}

func run(name string, args ...string) error {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w — %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}
