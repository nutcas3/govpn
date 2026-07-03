//go:build linux

package device

import "github.com/songgao/water"

func applyName(cfg *water.Config, name string) {
	cfg.PlatformSpecificParams = water.PlatformSpecificParams{Name: name}
}
