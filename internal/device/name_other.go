//go:build !linux

package device

import "github.com/songgao/water"

// applyName is a no-op on macOS and Windows: the OS assigns the interface name.
func applyName(_ *water.Config, _ string) {}
