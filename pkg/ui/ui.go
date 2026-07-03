package ui

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

var isTTY bool

// SetTTY sets whether output is to a TTY. Should be called at startup.
func SetTTY(v bool) {
	isTTY = v
}

func color(code, s string) string {
	if !isTTY {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func Bold(s string) string    { return color("1", s) }
func Dim(s string) string     { return color("2", s) }
func Green(s string) string   { return color("32", s) }
func Yellow(s string) string  { return color("33", s) }
func Cyan(s string) string    { return color("36", s) }
func Red(s string) string     { return color("31", s) }
func Magenta(s string) string { return color("35", s) }

func LogStep(msg string) {
	fmt.Fprint(os.Stderr, "  "+Dim("•")+" "+msg+"\n")
}

func LogOK(msg string) {
	fmt.Fprint(os.Stderr, "  "+Green("✓")+" "+msg+"\n")
}

func LogWarn(msg string) {
	fmt.Fprint(os.Stderr, "  "+Yellow("⚠")+" "+msg+"\n")
}

func LogErr(msg string) {
	fmt.Fprint(os.Stderr, "  "+Red("✗")+" "+msg+"\n")
}

func Fatal(msg string) {
	fmt.Fprint(os.Stderr, "\n"+Red("error")+": "+msg+"\n\n")
	os.Exit(1)
}

func Section(title string) {
	fmt.Fprint(os.Stderr, "\n"+Bold(title)+"\n"+Dim(strings.Repeat("─", 42))+"\n")
}

var brailleFrames = [...]string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type Spinner struct {
	mu   sync.Mutex
	msg  string
	stop chan struct{}
	done chan struct{}
}

func NewSpinner(msg string) *Spinner {
	return &Spinner{
		msg:  msg,
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
}

func (s *Spinner) Start() {
	if !isTTY {
		LogStep(s.msg + "…")
		close(s.done)
		return
	}

	go func() {
		defer close(s.done)
		for i := 0; ; i++ {
			select {
			case <-s.stop:
				fmt.Fprint(os.Stderr, "\r\x1b[2K")
				return
			case <-time.After(80 * time.Millisecond):
				s.mu.Lock()
				msg := s.msg
				s.mu.Unlock()
				fmt.Fprint(os.Stderr, "\r  "+Cyan(brailleFrames[i%len(brailleFrames)])+" "+msg)
			}
		}
	}()
}

func (s *Spinner) Update(msg string) {
	s.mu.Lock()
	s.msg = msg
	s.mu.Unlock()
}

func (s *Spinner) Finish(ok bool, msg string) {
	if !isTTY {
		if msg != "" {
			if ok {
				LogOK(msg)
			} else {
				LogErr(msg)
			}
		}
		return
	}
	select {
	case <-s.done:
	default:
		close(s.stop)
		<-s.done
	}
	if msg == "" {
		return
	}
	if ok {
		LogOK(msg)
	} else {
		LogErr(msg)
	}
}

func PrintTable(rows [][2]string) {
	maxKey := 0
	for _, r := range rows {
		if len(r[0]) > maxKey {
			maxKey = len(r[0])
		}
	}
	for _, r := range rows {
		pad := strings.Repeat(" ", maxKey-len(r[0]))
		fmt.Printf("  %s%s  %s  %s\n", Dim(r[0]), pad, Dim("│"), r[1])
	}
}

func FmtBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
