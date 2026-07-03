//go:build linux

package routing

import (
	"fmt"
	"os/exec"
	"strings"
)

// New returns the Linux Manager (iproute2).
func New() Manager { return &linuxManager{} }

type linuxManager struct{}

func (m *linuxManager) ConfigureInterface(name, localCIDR, _ string, mtu int) error {
	if err := ip("link", "set", "dev", name, "mtu", fmt.Sprint(mtu)); err != nil {
		return fmt.Errorf("routing: set mtu on %s: %w", name, err)
	}
	if err := ip("addr", "add", localCIDR, "dev", name); err != nil {
		if !strings.Contains(err.Error(), "File exists") {
			return fmt.Errorf("routing: assign %s to %s: %w", localCIDR, name, err)
		}
	}
	if err := ip("link", "set", "dev", name, "up"); err != nil {
		return fmt.Errorf("routing: bring up %s: %w", name, err)
	}
	return nil
}

func (m *linuxManager) AddRoute(name, subnet string) error {
	if err := ip("route", "add", subnet, "dev", name); err != nil {
		if strings.Contains(err.Error(), "File exists") {
			return nil // already present is fine
		}
		return fmt.Errorf("routing: add route %s via %s: %w", subnet, name, err)
	}
	return nil
}

func (m *linuxManager) DeleteRoute(name, subnet string) error {
	if err := ip("route", "del", subnet, "dev", name); err != nil {
		return fmt.Errorf("routing: delete route %s via %s: %w", subnet, name, err)
	}
	return nil
}

func ip(args ...string) error {
	out, err := exec.Command("ip", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("ip %s: %w — %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}
