package main

import (
	"fmt"
	"runtime"
)

func runVersion() {
	fmt.Printf("%s %s\n", bold("govpn"), cyan("v"+Version))
	fmt.Printf("  go      %s\n", dim(runtime.Version()))
	fmt.Printf("  os/arch %s/%s\n", dim(runtime.GOOS), dim(runtime.GOARCH))
}
