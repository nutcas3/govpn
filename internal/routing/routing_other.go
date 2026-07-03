//go:build !linux && !darwin && !windows

package routing

// New returns a stub that always returns ErrNotSupported.
func New() Manager { return stub{} }

type stub struct{}

func (stub) ConfigureInterface(_, _, _ string, _ int) error { return ErrNotSupported }
func (stub) AddRoute(_, _ string) error                     { return ErrNotSupported }
func (stub) DeleteRoute(_, _ string) error                  { return ErrNotSupported }
