package main

import (
	"github.com/govpn/govpn/pkg/ui"
)

// isTTY is true when stderr is connected to an interactive terminal.
// Populated once at startup by the platform-specific isTerminal().
var isTTY = isTerminal()

func init() {
	ui.SetTTY(isTTY)
}

func color(code, s string) string {
	if !isTTY {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func bold(s string) string    { return color("1", s) }
func dim(s string) string     { return color("2", s) }
func green(s string) string   { return color("32", s) }
func yellow(s string) string  { return color("33", s) }
func cyan(s string) string    { return color("36", s) }
func red(s string) string     { return color("31", s) }
func magenta(s string) string { return color("35", s) }

// All output goes to stderr so stdout remains clean for piped output.

func logStep(msg string) {
	ui.LogStep(msg)
}

func logOK(msg string) {
	ui.LogOK(msg)
}

func logWarn(msg string) {
	ui.LogWarn(msg)
}

func logErr(msg string) {
	ui.LogErr(msg)
}

func fatal(msg string) {
	ui.Fatal(msg)
}

func section(title string) {
	ui.Section(title)
}

type spinner = ui.Spinner

func newSpinner(msg string) *spinner {
	return ui.NewSpinner(msg)
}

func printTable(rows [][2]string) {
	ui.PrintTable(rows)
}

func fmtBytes(b uint64) string {
	return ui.FmtBytes(b)
}
