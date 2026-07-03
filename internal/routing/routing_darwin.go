//go:build darwin

package routing

import (
	"fmt"
	"os/exec"
	"strings"
)

// New returns the macOS Manager (ifconfig + route).
func New() Manager { return &darwinManager{} }

type darwinManager struct{}

func (m *darwinManager) ConfigureInterface(name, localCIDR, peerIP string, mtu int) error {
	localIP, _, _ := strings.Cut(localCIDR, "/")
	if peerIP == "" {
		peerIP = localIP
	}

	if err := run("ifconfig", name, "mtu", fmt.Sprint(mtu)); err != nil {
		return fmt.Errorf("routing: set mtu on %s: %w", name, err)
	}
	if err := run("ifconfig", name, localIP, peerIP, "up"); err != nil {
		return fmt.Errorf("routing: configure %s (%s → %s): %w", name, localIP, peerIP, err)
	}
	return nil
}

func (m *darwinManager) AddRoute(name, subnet string) error {
	if err := run("route", "add", "-net", subnet, "-interface", name); err != nil {
		if strings.Contains(err.Error(), "File exists") {
			return nil
		}
		return fmt.Errorf("routing: add route %s via %s: %w", subnet, name, err)
	}
	return nil
}

func (m *darwinManager) DeleteRoute(_, subnet string) error {
	if err := run("route", "delete", "-net", subnet); err != nil {
		return fmt.Errorf("routing: delete route %s: %w", subnet, err)
	}
	return nil
}

func run(name string, args ...string) error {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w — %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}
