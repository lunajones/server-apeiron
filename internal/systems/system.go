package systems

import (
	"context"
	"time"

	"server-apeiron/internal/clock"
)

type Context struct {
	Context context.Context
	Now     time.Time
	Delta   time.Duration
	Tick    uint64
}

type System interface {
	Tick(Context) error
}

type SystemFunc func(Context) error

func (f SystemFunc) Tick(ctx Context) error {
	return f(ctx)
}

type namedSystem struct {
	name  string
	phase clock.TickPhase
	fn    func(Context) error
}

func NewSystemFunc(name string, phase clock.TickPhase, fn func(Context) error) System {
	return namedSystem{name: name, phase: phase, fn: fn}
}

func (s namedSystem) Tick(ctx Context) error {
	if s.fn == nil {
		return nil
	}
	return s.fn(ctx)
}

func (s namedSystem) Name() string {
	return s.name
}

func (s namedSystem) Phase() clock.TickPhase {
	return s.phase
}
