//go:build !linux && !darwin && !windows

package main

func isTerminal() bool { return false }
